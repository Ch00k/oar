// Package services provides interfaces and implementations for various services in Oar.
package services

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

type ProjectStatus int

const (
	ProjectStatusRunning ProjectStatus = iota
	ProjectStatusStopped
	ProjectStatusError
	ProjectStatusUnknown
)

func (s ProjectStatus) String() string {
	switch s {
	case ProjectStatusRunning:
		return "running"
	case ProjectStatusStopped:
		return "stopped"
	case ProjectStatusError:
		return "error"
	case ProjectStatusUnknown:
		return "unknown"
	default:
		return "unknown"
	}
}

func ParseProjectStatus(s string) (ProjectStatus, error) {
	switch s {
	case "running":
		return ProjectStatusRunning, nil
	case "stopped":
		return ProjectStatusStopped, nil
	case "error":
		return ProjectStatusError, nil
	case "unknown":
		return ProjectStatusUnknown, nil
	default:
		return ProjectStatusUnknown, fmt.Errorf("invalid project status: %q", s)
	}
}

// ProjectService provides methods to manage Docker Compose projects.
type ProjectService struct {
	projectRepository    ProjectRepository
	deploymentRepository DeploymentRepository
	gitService           GitExecutor
	config               *Config
}

// Ensure ProjectService implements ProjectManager
var _ ProjectManager = (*ProjectService)(nil)

// List returns all projects
func (s *ProjectService) List() ([]*Project, error) {
	projects, err := s.projectRepository.List()
	if err != nil {
		slog.Error("Service operation failed",
			"layer", "service",
			"operation", "list_projects",
			"error", err)
		return nil, err
	}
	return projects, nil
}

// Get retrieves a project by ID
func (s *ProjectService) Get(id uuid.UUID) (*Project, error) {
	project, err := s.projectRepository.FindByID(id)
	if err != nil {
		slog.Error("Service operation failed",
			"layer", "service",
			"operation", "get_project",
			"project_id", id,
			"error", err)
		return nil, err // Pass through as-is
	}
	return project, nil
}

func (s *ProjectService) GetByName(name string) (*Project, error) {
	project, err := s.projectRepository.FindByName(name)
	if err != nil {
		slog.Error("Service operation failed",
			"layer", "service",
			"operation", "get_project",
			"project_name", name,
			"error", err)
		return nil, err // Pass through as-is
	}
	return project, nil
}

// Create creates a new project (backward compatibility)
func (s *ProjectService) Create(project *Project) (*Project, error) {
	return s.CreateFromTempClone(project, "")
}

// CreateFromTempClone creates a new project, optionally using a temp clone
func (s *ProjectService) CreateFromTempClone(
	project *Project,
	tempClonePath string,
) (*Project, error) {
	project.WorkingDir = filepath.Join(s.config.WorkspaceDir, project.ID.String())

	gitDir, err := project.GitDir()
	if err != nil {
		slog.Error("Service operation failed",
			"layer", "service",
			"operation", "create_project",
			"project_id", project.ID,
			"project_name", project.Name,
			"error", err)
		return nil, err
	}

	// Move temp clone to permanent location if provided
	if tempClonePath != "" {
		// Verify temp directory exists
		if _, err := os.Stat(tempClonePath); os.IsNotExist(err) {
			slog.Error("Service operation failed",
				"layer", "service",
				"operation", "create_project",
				"project_id", project.ID,
				"temp_path", tempClonePath,
				"error", "temp directory not found")
			return nil, err
		}

		// Ensure parent directory exists
		if err := os.MkdirAll(filepath.Dir(gitDir), 0o755); err != nil {
			slog.Error("Service operation failed",
				"layer", "service",
				"operation", "create_project",
				"project_id", project.ID,
				"error", err)
			return nil, err
		}

		// Move temp directory to permanent location
		if err := os.Rename(tempClonePath, gitDir); err != nil {
			// Clean up temp clone on error
			if cleanupErr := os.RemoveAll(tempClonePath); cleanupErr != nil {
				slog.Error("Failed to cleanup temp clone after move failure",
					"layer", "service",
					"operation", "create_project_cleanup",
					"temp_path", tempClonePath,
					"error", cleanupErr)
			}
			slog.Error("Service operation failed",
				"layer", "service",
				"operation", "create_project",
				"project_id", project.ID,
				"project_name", project.Name,
				"temp_clone_path", tempClonePath,
				"error", err)
			return nil, err
		}

		slog.Info("Temp clone moved to project location",
			"temp_path", tempClonePath,
			"project_id", project.ID,
			"git_dir", gitDir)
	} else {
		// Clone repository first (fallback for cases without discovery)
		if err := s.gitService.Clone(project.GitURL, gitDir); err != nil {
			slog.Error("Service operation failed",
				"layer", "service",
				"operation", "create_project",
				"project_id", project.ID,
				"project_name", project.Name,
				"git_url", project.GitURL,
				"error", err)
			return nil, err
		}
	}

	// Get commit info
	commit, _ := s.gitService.GetLatestCommit(gitDir)
	project.LastCommit = &commit

	// TODO: Discover compose files is none are provided

	// Save working directory for cleanup before repository call
	workingDir := project.WorkingDir

	createdProject, err := s.projectRepository.Create(project)
	if err != nil {
		// Cleanup on failure using saved working directory
		if cleanupErr := os.RemoveAll(workingDir); cleanupErr != nil {
			slog.Error(
				"Failed to remove project directory after creation failure",
				"working_dir",
				workingDir,
				"error",
				cleanupErr,
			)
		}
		slog.Error("Service operation failed",
			"layer", "service",
			"operation", "create_project",
			"project_id", project.ID,
			"project_name", project.Name,
			"git_url", project.GitURL,
			"error", err)
		return nil, err // Pass through as-is
	}

	return createdProject, nil
}

func (s *ProjectService) Update(project *Project) error {
	return s.projectRepository.Update(project)
}

func (s *ProjectService) DeployStreaming(
	projectID uuid.UUID,
	pull bool,
	outputChan chan<- string,
) error {
	defer close(outputChan)

	// Get project
	project, err := s.Get(projectID)
	if err != nil {
		return fmt.Errorf("project not found: %w", err)
	}

	gitDir, err := project.GitDir()
	if err != nil {
		return fmt.Errorf("failed to get git directory: %w", err)
	}

	// Pull latest changes if requested
	if pull {
		outputChan <- `{"type":"info","message":"Pulling latest changes from Git...","source":"oar"}`
		if err := s.pullLatestChanges(project); err != nil {
			outputChan <- fmt.Sprintf(`{"type":"error","message":"Failed to pull latest changes: %v"}`, err)
			return err
		}
		outputChan <- `{"type":"success","message":"Git pull completed successfully"}`
	}

	commitHash, err := s.gitService.GetLatestCommit(gitDir)
	if err != nil {
		slog.Error("Service operation failed",
			"layer", "service",
			"operation", "deploy_project",
			"project_id", project.ID,
			"error", err)
		return err
	}

	deployment := NewDeployment(projectID, commitHash)

	// Deploy using Docker Compose
	slog.Info("Starting Docker Compose deployment",
		"project_id", project.ID,
		"project_name", project.Name,
		"compose_files", project.ComposeFiles,
		"pull", pull)

	composeProject := NewComposeProject(project)

	outputChan <- `{"type":"info","message":"Starting Docker Compose deployment...","source":"oar"}`
	err = composeProject.UpStreaming(outputChan)
	if err != nil {
		slog.Error(
			"Docker Compose up failed",
			"project_id",
			project.ID,
			"error",
			err,
		)
		return fmt.Errorf("failed to start project: %w", err)
	}
	slog.Info(
		"Docker Compose project started",
		"project_id",
		project.ID,
	)

	// Update deployment
	deployment.Status = DeploymentStatusCompleted
	// deployment.Output = result.Output

	// Update project
	project.Status = ProjectStatusRunning
	project.LastCommit = &commitHash

	// TODO: Transaction
	if err := s.deploymentRepository.Create(&deployment); err != nil {
		return fmt.Errorf("failed to update deployment record: %w", err)
	}

	if err := s.projectRepository.Update(project); err != nil {
		return fmt.Errorf("failed to update project status: %w", err)
	}

	// Send unified message with both display text and project state
	outputChan <- fmt.Sprintf(`{"type":"success","message":"Docker Compose deployment completed successfully","projectState":{"status":"running","lastCommit":"%s"}}`, commitHash)

	return nil
}

func (s *ProjectService) Start(projectID uuid.UUID) error {
	// Get project
	project, err := s.Get(projectID)
	if err != nil {
		return fmt.Errorf("project not found: %w", err)
	}

	// Start Docker Compose
	slog.Info(
		"Starting Docker Compose project",
		"project_id",
		project.ID,
		"project_name",
		project.Name,
	)

	composeProject := NewComposeProject(project)

	output, err := composeProject.Up()
	if err != nil {
		slog.Error(
			"Docker Compose up failed",
			"project_id",
			project.ID,
			"error",
			err,
			"output",
			output,
		)
		return fmt.Errorf("failed to start project: %w", err)
	}
	slog.Info(
		"Docker Compose project started",
		"project_id",
		project.ID,
		"output_length",
		len(output),
	)

	project.Status = ProjectStatusRunning
	return s.Update(project)
}

func (s *ProjectService) StartStreaming(projectID uuid.UUID, outputChan chan<- string) error {
	// Get project
	project, err := s.Get(projectID)
	if err != nil {
		return fmt.Errorf("project not found: %w", err)
	}

	// Start Docker Compose
	slog.Info(
		"Starting Docker Compose project",
		"project_id",
		project.ID,
		"project_name",
		project.Name,
	)

	composeProject := NewComposeProject(project)

	err = composeProject.UpStreaming(outputChan)
	if err != nil {
		slog.Error(
			"Docker Compose up failed",
			"project_id",
			project.ID,
			"error",
			err,
		)
		return fmt.Errorf("failed to start project: %w", err)
	}
	slog.Info(
		"Docker Compose project started",
		"project_id",
		project.ID,
	)

	project.Status = ProjectStatusRunning
	return s.Update(project)
}

func (s *ProjectService) Stop(projectID uuid.UUID) error {
	// Get project
	project, err := s.Get(projectID)
	if err != nil {
		return fmt.Errorf("project not found: %w", err)
	}

	// Stop Docker Compose
	slog.Info(
		"Stopping Docker Compose project",
		"project_id",
		project.ID,
		"project_name",
		project.Name,
	)

	composeProject := NewComposeProject(project)

	output, err := composeProject.Down()
	if err != nil {
		slog.Error(
			"Docker Compose down failed",
			"project_id",
			project.ID,
			"error",
			err,
			"output",
			output,
		)
		return fmt.Errorf("failed to stop project: %w", err)
	}
	slog.Info(
		"Docker Compose project stopped",
		"project_id",
		project.ID,
		"output_length",
		len(output),
	)

	project.Status = ProjectStatusStopped
	return s.Update(project)
}

func (s *ProjectService) StopStreaming(projectID uuid.UUID, outputChan chan<- string) error {
	defer close(outputChan)

	// Get project
	project, err := s.Get(projectID)
	if err != nil {
		return fmt.Errorf("project not found: %w", err)
	}

	// Stop Docker Compose
	slog.Info(
		"Stopping Docker Compose project with streaming",
		"project_id",
		project.ID,
		"project_name",
		project.Name,
	)

	composeProject := NewComposeProject(project)

	outputChan <- `{"type":"info","message":"Starting Docker Compose shutdown...","source":"oar"}`
	err = composeProject.DownStreaming(outputChan)
	if err != nil {
		slog.Error(
			"Docker Compose down failed",
			"project_id",
			project.ID,
			"error",
			err,
		)
		return fmt.Errorf("failed to stop project: %w", err)
	}
	slog.Info(
		"Docker Compose project stopped",
		"project_id",
		project.ID,
	)

	project.Status = ProjectStatusStopped
	err = s.Update(project)
	if err != nil {
		return fmt.Errorf("failed to update project status: %w", err)
	}

	// Send unified message with both display text and project state
	outputChan <- `{"type":"success","message":"Docker Compose shutdown completed successfully","projectState":{"status":"stopped"}}`

	return nil
}

func (s *ProjectService) Remove(projectID uuid.UUID) error {
	// Get project
	project, err := s.Get(projectID)
	if err != nil {
		return fmt.Errorf("project not found: %w", err)
	}

	// Stop Docker Compose project if running
	if err := s.Stop(projectID); err != nil {
		slog.Warn("Failed to stop project before removal", "project_id", project.ID, "error", err)
		return fmt.Errorf("failed to stop project before removal: %w", err)
	}

	// Remove project directory
	if err := os.RemoveAll(project.WorkingDir); err != nil {
		return fmt.Errorf("failed to remove project directory: %w", err)
	}

	// Delete project from database
	if err := s.projectRepository.Delete(projectID); err != nil {
		return fmt.Errorf("failed to delete project from database: %w", err)
	}

	slog.Info(
		"Project removed successfully",
		"project_id",
		project.ID,
		"working_dir",
		project.WorkingDir,
	)
	return nil
}

func (s *ProjectService) GetLogsStreaming(projectID uuid.UUID, outputChan chan<- string) error {
	defer close(outputChan)

	// Get projectID
	project, err := s.Get(projectID)
	if err != nil {
		return fmt.Errorf("project not found: %w", err)
	}

	// Stream logs using Docker Compose
	slog.Info(
		"Streaming logs for Docker Compose project",
		"project_id",
		project.ID,
		"project_name",
		project.Name,
	)

	composeProject := NewComposeProject(project)

	err = composeProject.LogsStreaming(outputChan)
	if err != nil {
		slog.Error(
			"Failed to stream logs",
			"project_id",
			project.ID,
			"error",
			err,
		)
		return fmt.Errorf("failed to stream logs: %w", err)
	}
	slog.Info(
		"Logs streaming completed",
		"project_id",
		project.ID,
		"project_name",
		project.Name,
	)
	return nil
}

func (s *ProjectService) pullLatestChanges(project *Project) error {
	slog.Info("Pulling latest changes", "project_id", project.ID, "git_url", project.GitURL)

	gitDir, err := project.GitDir()
	if err != nil {
		return fmt.Errorf("failed to get git directory: %w", err)
	}

	if err = s.gitService.Pull(gitDir); err != nil {
		slog.Error("Failed to pull changes", "project_id", project.ID, "error", err)
		return fmt.Errorf("failed to pull changes: %w", err)
	}

	slog.Info("Git pull completed", "project_id", project.ID)
	return nil
}

// NewProjectService creates a new ProjectService with dependency injection
func NewProjectService(
	projectRepository ProjectRepository,
	deploymentRepository DeploymentRepository,
	gitService GitExecutor,
	config *Config,
) *ProjectService {
	return &ProjectService{
		projectRepository:    projectRepository,
		deploymentRepository: deploymentRepository,
		gitService:           gitService,
		config:               config,
	}
}
