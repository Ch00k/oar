// Package handlers provides HTTP request handlers for the Oar application.
package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/a-h/templ"
	"github.com/ch00k/oar/services"
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
			"operation", "project_grid_response",
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
	environmentFiles := r.Form["environment_files"]
	tempClonePath := r.FormValue("temp_clone_path")

	// Create and save project
	project := services.NewProject(name, gitURL, composeFiles, environmentFiles)

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

	h.handleProjectGridResponse(w, r, err, "project-created",
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

	h.handleProjectGridResponse(w, r, err, "project-deleted",
		"Project deleted successfully", "Project has been removed and all data cleaned up.",
		"Failed to delete project", errorDesc)
}

func (h *ProjectHandlers) Stop(w http.ResponseWriter, r *http.Request) {
	projectID, err := parseProjectID(r)
	if err != nil {
		http.Error(w, "Invalid project ID", http.StatusBadRequest)
		return
	}

	err = h.projectManager.Stop(projectID)

	// Get the updated project to return the card
	project, getErr := h.projectManager.Get(projectID)
	if getErr != nil {
		slog.Error("Handler operation failed",
			"layer", "handler",
			"operation", "get_project_after_stop",
			"project_id", projectID,
			"error", getErr)
		http.Error(w, "Failed to retrieve updated project", http.StatusInternalServerError)
		return
	}

	trigger := "project-stopped"
	if err == nil {
		// Close the modal on successful stop
		trigger = fmt.Sprintf("project-stopped,close-modal-stop-project-modal-%s", projectID.String())
	}

	h.handleProjectCardResponse(w, r, project, err, trigger,
		"Project stopped successfully", "Project has been stopped and containers have been shut down.",
		"Failed to stop project", "There was an error stopping the project. Please try again.")
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

// Helper function to build project from form data
func (h *ProjectHandlers) buildProjectFromForm(r *http.Request, projectID uuid.UUID) *services.Project {
	status, _ := services.ParseProjectStatus(r.FormValue("status"))

	var lastCommit *string
	if lc := r.FormValue("last_commit"); lc != "" {
		lastCommit = &lc
	}

	return &services.Project{
		ID:               projectID,
		Name:             r.FormValue("name"),
		GitURL:           r.FormValue("git_url"),
		WorkingDir:       r.FormValue("working_dir"),
		ComposeFiles:     r.Form["compose_files"],
		EnvironmentFiles: r.Form["environment_files"],
		Status:           status,
		LastCommit:       lastCommit,
	}
}

// RegisterRoutes registers all project-related routes
func (h *ProjectHandlers) RegisterRoutes(r chi.Router) {
	r.Post("/projects/create", h.Create)
	r.Post("/projects/{projectID}/edit", h.Edit)
	r.Delete("/projects/{projectID}", h.Delete)
	r.Post("/projects/{projectID}/stop", h.Stop)
	r.Get("/projects/{projectID}/deploy/stream", h.DeployStream)
	r.Get("/projects/{projectID}/stop/stream", h.StopStream)
	r.Get("/projects/{projectID}/logs/stream", h.LogsStream)
}
