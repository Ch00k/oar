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
		return nil, fmt.Errorf("failed to list projects: %w", err)
	}
	return projects, nil
}

// Get retrieves a project by ID
func (s *ProjectService) Get(id uuid.UUID) (*Project, error) {
	project, err := s.projectRepository.FindByID(id)
	if err != nil {
		return nil, fmt.Errorf("failed to get project with ID %s: %w", id, err)
	}
	return project, nil
}

func (s *ProjectService) GetByName(name string) (*Project, error) {
	project, err := s.projectRepository.FindByName(name)
	if err != nil {
		return nil, fmt.Errorf("failed to find project with name %s: %w", name, err)
	}
	return project, nil
}

// Create creates a new project
func (s *ProjectService) Create(project *Project) (*Project, error) {
	project.WorkingDir = filepath.Join(s.config.WorkspaceDir, project.ID.String())

	gitDir, err := project.GitDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get git directory: %w", err)
	}

	// Clone repository first
	if err := s.gitService.Clone(project.GitURL, gitDir); err != nil {
		return nil, fmt.Errorf("failed to clone repository: %w", err)
	}

	// Get commit info
	commit, _ := s.gitService.GetLatestCommit(gitDir)
	project.LastCommit = &commit

	// TODO: Discover compose files is none are provided

	project, err = s.projectRepository.Create(project)
	if err != nil {
		// Cleanup on failure
		if err := os.RemoveAll(project.WorkingDir); err != nil {
			slog.Error(
				"Failed to remove project directory after creation failure",
				"working_dir",
				project.WorkingDir,
				"error",
				err,
			)
			return nil, fmt.Errorf("failed to create project: %w", err)
		}
		return nil, fmt.Errorf("failed to create project: %w", err)
	}

	return project, nil
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
		outputChan <- "OAR_MSG:default:Pulling latest changes from Git..."
		if err := s.pullLatestChanges(project); err != nil {
			outputChan <- fmt.Sprintf("OAR_MSG:error:Failed to pull latest changes: %v", err)
			return fmt.Errorf("failed to pull latest changes: %w", err)
		}
		outputChan <- "OAR_MSG:success:Git pull completed successfully"
	}

	commitHash, err := s.gitService.GetLatestCommit(gitDir)
	if err != nil {
		slog.Error("Failed to get latest commit", "project_id", project.ID, "error", err)
		return fmt.Errorf("failed to get latest commit: %w", err)
	}

	deployment := NewDeployment(projectID, commitHash)

	// Deploy using Docker Compose
	slog.Info("Starting Docker Compose deployment",
		"project_id", project.ID,
		"project_name", project.Name,
		"compose_files", project.ComposeFiles,
		"pull", pull)

	composeProject := NewComposeProject(project)

	outputChan <- "OAR_MSG:default:Starting Docker Compose deployment..."
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

	outputChan <- "OAR_MSG:success:Docker Compose deployment completed successfully"

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

	outputChan <- "OAR_MSG:default:Starting Docker Compose shutdown..."
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

	outputChan <- "OAR_MSG:success:Docker Compose shutdown completed successfully"

	project.Status = ProjectStatusStopped
	return s.Update(project)
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
