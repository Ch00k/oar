// Package handlers provides HTTP request handlers for the Oar application.
package handlers

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/a-h/templ"
	"github.com/ch00k/oar/services"
	"github.com/ch00k/oar/ui/components/projectform"
	"github.com/ch00k/oar/ui/pages"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type ProjectHandlers struct {
	projectManager services.ProjectManager
}

func NewProjectHandlers(pm services.ProjectManager) *ProjectHandlers {
	return &ProjectHandlers{
		projectManager: pm,
	}
}

// Helper functions
func parseProjectID(r *http.Request) (uuid.UUID, error) {
	projectIDStr := chi.URLParam(r, "projectID")
	return uuid.Parse(projectIDStr)
}

func setupSSEHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
}

func renderComponentWithError(w http.ResponseWriter, r *http.Request, component templ.Component, errorMsg string) {
	if err := component.Render(r.Context(), w); err != nil {
		slog.Error("Handler operation failed",
			"layer", "handler",
			"operation", "render_component",
			"error", err)
		http.Error(w, errorMsg, http.StatusInternalServerError)
	}
}

func flushResponse(w http.ResponseWriter) {
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Generic CRUD response handler
func (h *ProjectHandlers) handleProjectGridResponse(
	w http.ResponseWriter,
	r *http.Request,
	err error,
	trigger, successTitle, successDesc, errorTitle, errorDesc string,
) {
	w.Header().Set("HX-Trigger", trigger)

	projects, listErr := h.projectManager.List()
	if listErr != nil {
		slog.Error("Handler operation failed",
			"layer", "handler",
			"operation", "list_projects",
			"error", listErr)
		projects = []*services.Project{} // Empty list on error
	}

	var component templ.Component
	if err != nil {
		slog.Error("Handler operation failed",
			"layer", "handler",
			"operation", "project_grid_response",
			"error", err)
		component = pages.ProjectsGridWithErrorToast(projects, errorTitle, errorDesc)
	} else {
		component = pages.ProjectsGridWithSuccessToast(projects, successTitle, successDesc)
	}

	renderComponentWithError(w, r, component, "Failed to render projects grid")
}

// Generic project card response handler
func (h *ProjectHandlers) handleProjectCardResponse(
	w http.ResponseWriter,
	r *http.Request,
	project *services.Project,
	err error,
	trigger, successTitle, successDesc, errorTitle, errorDesc string,
) {
	w.Header().Set("HX-Trigger", trigger)

	var component templ.Component
	if err != nil {
		slog.Error("Handler operation failed",
			"layer", "handler",
			"operation", "project_card_response",
			"error", err)
		component = pages.ProjectCardWithErrorToast(project, errorTitle, errorDesc)
	} else {
		component = pages.ProjectCardWithSuccessToast(project, successTitle, successDesc)
	}

	renderComponentWithError(w, r, component, "Failed to render project card")
}

// Generic streaming handler
func (h *ProjectHandlers) handleStreaming(w http.ResponseWriter, r *http.Request,
	operation func(uuid.UUID, chan string) error,
	successMsg, errorMsgPrefix string,
) {
	projectID, err := parseProjectID(r)
	if err != nil {
		http.Error(w, "Invalid project ID", http.StatusBadRequest)
		return
	}

	setupSSEHeaders(w)

	// Create channels for streaming
	outputChan := make(chan string, 100)
	done := make(chan error, 1)

	// Execute operation in goroutine
	go func() {
		err := operation(projectID, outputChan)
		done <- err
	}()

	// Stream output to client
	for line := range outputChan {
		if _, err := fmt.Fprintf(w, "data: %s\n\n", line); err != nil {
			slog.Error("Handler operation failed",
				"layer", "handler",
				"operation", "stream_write",
				"error", err)
			break
		}
		flushResponse(w)
	}

	// Handle completion - services already send completion messages as JSON
	serviceErr := <-done
	if serviceErr != nil {
		slog.Error("Handler operation failed",
			"layer", "handler",
			"operation", "streaming",
			"error", serviceErr)
		// Send error completion as JSON message
		errorMsg := fmt.Sprintf(`{"type":"error","message":"%s: %v"}`, errorMsgPrefix, serviceErr)
		if _, err := fmt.Fprintf(w, "data: %s\n\n", errorMsg); err != nil {
			slog.Error("Handler operation failed",
				"layer", "handler",
				"operation", "stream_completion",
				"error", err)
		}
	}
	// Success completion is already sent by the service as JSON with project state
	flushResponse(w)
}

// CRUD Handlers

// Create creates a new project
func (h *ProjectHandlers) Create(w http.ResponseWriter, r *http.Request) {
	// Parse form data
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	// Extract form values
	name := r.FormValue("name")
	gitURL := r.FormValue("git_url")
	composeFiles := r.Form["compose_files"]
	variables := parseVariablesFromRaw(r.FormValue("variables_raw"))
	tempClonePath := r.FormValue("temp_clone_path")

	// Validate form data
	if validationError := h.validateProjectForm(name, gitURL); validationError != "" {
		h.handleProjectGridResponse(w, r, errors.New(validationError), "project-creation-failed",
			"Project created successfully", "New project has been added and cloned.",
			"Failed to create project", validationError)
		return
	}

	// Create project and set Git authentication
	project := services.NewProject(name, gitURL, composeFiles, variables)
	project.GitAuth = CreateTempAuthConfig(r)

	var err error
	if tempClonePath == "" {
		_, err = h.projectManager.Create(&project)
	} else {
		// Use temp clone approach if temp clone path is provided
		if projectService, ok := h.projectManager.(*services.ProjectService); ok {
			_, err = projectService.CreateFromTempClone(&project, tempClonePath)
		} else {
			_, err = h.projectManager.Create(&project)
		}

		// Cleanup temp clone on any error (as fallback if service doesn't clean up)
		if err != nil {
			if cleanupErr := os.RemoveAll(tempClonePath); cleanupErr != nil {
				slog.Error("Failed to cleanup temp clone directory",
					"layer", "handler",
					"operation", "create_project_cleanup",
					"temp_path", tempClonePath,
					"error", cleanupErr)
			}
		}
	}

	var errorDesc string
	if err != nil {
		// Log at handler level with request context
		slog.Error("Handler operation failed",
			"layer", "handler",
			"operation", "create_project",
			"project_name", project.Name,
			"git_url", project.GitURL,
			"error", err)

		// Format user-friendly message
		errorDesc = services.FormatErrorForUser(err)
	}

	// Send different triggers for success vs failure
	trigger := "project-created"
	if err != nil {
		trigger = "project-creation-failed"
	}

	h.handleProjectGridResponse(w, r, err, trigger,
		"Project created successfully", "New project has been added and cloned.",
		"Failed to create project", errorDesc)
}

func (h *ProjectHandlers) Edit(w http.ResponseWriter, r *http.Request) {
	projectID, err := parseProjectID(r)
	if err != nil {
		http.Error(w, "Invalid project ID", http.StatusBadRequest)
		return
	}

	// Parse form data
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	// Build project struct from form data
	project := h.buildProjectFromForm(r, projectID)

	// Update project
	err = h.projectManager.Update(project)

	h.handleProjectCardResponse(w, r, project, err, fmt.Sprintf("project-updated-%s", projectID),
		"Project updated successfully", "Project has been updated with your changes.",
		"Failed to update project", "There was an error updating the project. Please try again.")
}

func (h *ProjectHandlers) Delete(w http.ResponseWriter, r *http.Request) {
	projectID, err := parseProjectID(r)
	if err != nil {
		http.Error(w, "Invalid project ID", http.StatusBadRequest)
		return
	}

	err = h.projectManager.Remove(projectID)

	var errorDesc string
	if err != nil {
		// Log at handler level with request context
		slog.Error("Handler operation failed",
			"layer", "handler",
			"operation", "delete_project",
			"project_id", projectID,
			"error", err)

		// Format user-friendly message
		errorDesc = services.FormatErrorForUser(err)
	}

	// Send different triggers for success vs failure
	trigger := "project-deleted"
	if err != nil {
		trigger = "project-deletion-failed"
	}

	h.handleProjectGridResponse(w, r, err, trigger,
		"Project deleted successfully", "Project has been removed and all data cleaned up.",
		"Failed to delete project", errorDesc)
}

// Streaming Handlers

// DeployStream deploys a project and streams the output
func (h *ProjectHandlers) DeployStream(w http.ResponseWriter, r *http.Request) {
	// Check if pull parameter is set
	pullChanges := r.URL.Query().Get("pull") == "true"

	operation := func(id uuid.UUID, outputChan chan string) error {
		return h.projectManager.DeployStreaming(id, pullChanges, outputChan)
	}

	h.handleStreaming(w, r, operation,
		"Project deployed successfully", "Deployment failed")
}

func (h *ProjectHandlers) StopStream(w http.ResponseWriter, r *http.Request) {
	operation := func(id uuid.UUID, outputChan chan string) error {
		return h.projectManager.StopStreaming(id, outputChan)
	}

	h.handleStreaming(w, r, operation,
		"Project stopped successfully", "Stop failed")
}

func (h *ProjectHandlers) LogsStream(w http.ResponseWriter, r *http.Request) {
	operation := func(id uuid.UUID, outputChan chan string) error {
		return h.projectManager.GetLogsStreaming(id, outputChan)
	}

	h.handleStreaming(w, r, operation,
		"Log streaming ended", "Log streaming failed")
}

func (h *ProjectHandlers) GetConfig(w http.ResponseWriter, r *http.Request) {
	projectID, err := parseProjectID(r)
	if err != nil {
		http.Error(w, "Invalid project ID", http.StatusBadRequest)
		return
	}

	config, err := h.projectManager.GetConfig(projectID)
	if err != nil {
		slog.Error("Handler operation failed",
			"layer", "handler",
			"operation", "get_config",
			"project_id", projectID,
			"error", err)

		// Return error message in HTML format
		errorMessage := fmt.Sprintf("Failed to get project configuration: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprintf(w, `<div class="text-red-600">%s</div>`, errorMessage)
		return
	}

	// Return just the config content using the configContent template
	component := pages.ConfigContent(config)
	renderComponentWithError(w, r, component, "Failed to render config content")
}

// Helper function to build project from form data
func (h *ProjectHandlers) buildProjectFromForm(r *http.Request, projectID uuid.UUID) *services.Project {
	status, _ := services.ParseProjectStatus(r.FormValue("status"))

	var lastCommit *string
	if lc := r.FormValue("last_commit"); lc != "" {
		lastCommit = &lc
	}

	return &services.Project{
		ID:           projectID,
		Name:         r.FormValue("name"),
		GitURL:       r.FormValue("git_url"),
		GitAuth:      CreateTempAuthConfig(r),
		WorkingDir:   r.FormValue("working_dir"),
		ComposeFiles: r.Form["compose_files"],
		Variables:    parseVariablesFromRaw(r.FormValue("variables_raw")),
		Status:       status,
		LastCommit:   lastCommit,
	}
}

// parseVariablesFromRaw parses raw variable text into a slice of KEY=value strings
func parseVariablesFromRaw(raw string) []string {
	var variables []string
	lines := strings.Split(raw, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip empty lines and comments
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Check if line contains = separator
		if strings.Contains(trimmed, "=") {
			// Split only on the first = to handle values containing =
			equalIndex := strings.Index(trimmed, "=")
			key := strings.TrimSpace(trimmed[:equalIndex])
			value := strings.TrimSpace(trimmed[equalIndex+1:])

			// Skip if key is empty
			if key != "" {
				variables = append(variables, key+"="+value)
			}
		}
	}

	return variables
}

// AuthFields handles dynamic rendering of authentication fields based on auth type
func (h *ProjectHandlers) AuthFields(w http.ResponseWriter, r *http.Request) {
	authType := r.FormValue("auth_type")
	idPrefix := r.FormValue("id_prefix")

	// Default to empty if not provided
	if idPrefix == "" {
		idPrefix = "new-project"
	}

	// For edit mode, we might have a project ID to get existing values
	var project *services.Project
	if projectIDStr := r.FormValue("project_id"); projectIDStr != "" {
		if projectID, err := uuid.Parse(projectIDStr); err == nil {
			project, _ = h.projectManager.Get(projectID)
		}
	}

	component := projectform.AuthFields(projectform.AuthFieldsProps{
		AuthType: authType,
		IDPrefix: idPrefix,
		Project:  project,
	})

	renderComponentWithError(w, r, component, "Failed to render auth fields")
}

// NewForm renders a fresh project creation form for modal loading
func (h *ProjectHandlers) NewForm(w http.ResponseWriter, r *http.Request) {
	component := projectform.ProjectForm(projectform.ProjectFormProps{
		Mode:       "create",
		Project:    nil,
		FormID:     "add-project-form",
		FormAction: "/projects/create",
		FormTarget: "#projects-grid",
		FormSwap:   "innerHTML",
		IDPrefix:   "new-project",
	})

	renderComponentWithError(w, r, component, "Failed to render project form")
}

// ValidateComposeFiles validates Docker Compose file paths
func (h *ProjectHandlers) ValidateComposeFiles(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	composeFiles := r.Form["compose_files"]
	if len(composeFiles) == 0 {
		w.WriteHeader(http.StatusOK)
		return
	}

	var warnings []string
	var errors []string

	for _, file := range composeFiles {
		file = strings.TrimSpace(file)
		if file == "" {
			continue
		}

		// Validate compose file format
		if !isValidComposeFile(file) {
			errors = append(errors, fmt.Sprintf("'%s' is not a valid compose file name", file))
			continue
		}

		// Check for common patterns
		if !strings.Contains(file, ".yml") && !strings.Contains(file, ".yaml") {
			warnings = append(warnings, fmt.Sprintf("'%s' should end with .yml or .yaml", file))
		}
	}

	// Return validation feedback
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)

	if len(errors) > 0 {
		_, _ = fmt.Fprintf(w, `<div class="mt-1 p-2 bg-red-50 border border-red-200 rounded text-red-700 text-xs">
			<div class="font-medium">Invalid files:</div>
			<ul class="list-disc list-inside">%s</ul>
		</div>`, strings.Join(formatListItems(errors), ""))
	} else if len(warnings) > 0 {
		_, _ = fmt.Fprintf(w, `<div class="mt-1 p-2 bg-yellow-50 border border-yellow-200 rounded text-yellow-700 text-xs">
			<div class="font-medium">Suggestions:</div>
			<ul class="list-disc list-inside">%s</ul>
		</div>`, strings.Join(formatListItems(warnings), ""))
	} else {
		_, _ = fmt.Fprintf(w, `<div class="mt-1 p-2 bg-green-50 border border-green-200 rounded text-green-700 text-xs">
			✓ All compose files look valid
		</div>`)
	}
}

// Helper function to validate compose file format
func isValidComposeFile(filename string) bool {
	// Must contain valid characters
	if matched, _ := regexp.MatchString(`^[a-zA-Z0-9._/-]+$`, filename); !matched {
		return false
	}

	// Common compose file patterns
	validPatterns := []string{
		"compose.yml", "compose.yaml",
		"docker-compose.yml", "docker-compose.yaml",
		".yml", ".yaml", // Files ending with these extensions
	}

	for _, pattern := range validPatterns {
		if strings.Contains(filename, pattern) {
			return true
		}
	}

	return false
}

// Helper function to format list items for HTML
func formatListItems(items []string) []string {
	var formatted []string
	for _, item := range items {
		formatted = append(formatted, fmt.Sprintf("<li>%s</li>", item))
	}
	return formatted
}

// RegisterRoutes registers all project-related routes
func (h *ProjectHandlers) RegisterRoutes(r chi.Router) {
	r.Post("/projects/create", h.Create)
	r.Post("/projects/{projectID}/edit", h.Edit)
	r.Delete("/projects/{projectID}", h.Delete)
	r.Get("/projects/{projectID}/deploy/stream", h.DeployStream)
	r.Get("/projects/{projectID}/stop/stream", h.StopStream)
	r.Get("/projects/{projectID}/logs/stream", h.LogsStream)
	r.Get("/projects/{projectID}/config", h.GetConfig)
	r.Post("/projects/auth-fields", h.AuthFields)
	r.Get("/projects/new-form", h.NewForm)
	r.Post("/projects/validate/compose-files", h.ValidateComposeFiles)
}

// validateProjectForm validates the project creation form
func (h *ProjectHandlers) validateProjectForm(name, gitURL string) string {
	// Validate project name
	if name == "" {
		return "Project name is required"
	}

	// Check for valid characters (alphanumeric, dash, underscore)
	if matched, _ := regexp.MatchString("^[a-zA-Z0-9_-]+$", name); !matched {
		return "Project name can only contain letters, numbers, dashes, and underscores"
	}

	// Check name uniqueness
	existingProject, err := h.projectManager.GetByName(name)
	if err == nil && existingProject != nil {
		return "Project name already exists"
	}

	// Validate Git URL
	if gitURL == "" {
		return "Git URL is required"
	}

	// Basic URL validation for Git repositories
	if !isValidGitURL(gitURL) {
		return "Invalid Git URL format"
	}

	return ""
}

// Helper function to validate Git URL format
func isValidGitURL(url string) bool {
	// HTTP/HTTPS URLs
	if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
		if strings.Contains(url, ".git") || strings.Contains(url, "github.com") ||
			strings.Contains(url, "gitlab.com") || strings.Contains(url, "bitbucket.org") {
			return true
		}
	}

	// SSH URLs (git@host:user/repo.git format)
	if strings.HasPrefix(url, "git@") && strings.Contains(url, ":") && strings.HasSuffix(url, ".git") {
		return true
	}

	return false
}
