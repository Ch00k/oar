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
	dockerComposeService DockerComposeExecutor
	config               *Config
}

// Ensure ProjectService implements ProjectManager
var _ ProjectManager = (*ProjectService)(nil)

// ListProjects returns all projects
func (s *ProjectService) ListProjects() ([]*Project, error) {
	projects, err := s.projectRepository.List()
	if err != nil {
		return nil, fmt.Errorf("failed to list projects: %w", err)
	}
	return projects, nil
}

// GetProject retrieves a project by ID
func (s *ProjectService) GetProject(id uuid.UUID) (*Project, error) {
	project, err := s.projectRepository.FindByID(id)
	if err != nil {
		return nil, fmt.Errorf("failed to get project with ID %s: %w", id, err)
	}
	return project, nil
}

func (s *ProjectService) GetProjectByName(name string) (*Project, error) {
	project, err := s.projectRepository.FindByName(name)
	if err != nil {
		return nil, fmt.Errorf("failed to find project with name %s: %w", name, err)
	}
	return project, nil
}

// CreateProject creates a new project
func (s *ProjectService) CreateProject(project *Project) (*Project, error) {
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

	// Discover Docker Compose files
	var composeFiles []string

	if _, err := os.Stat(filepath.Join(gitDir, "compose.yaml")); err == nil {
		composeFiles = []string{"compose.yaml"}
	} else if _, err := os.Stat(filepath.Join(gitDir, "compose.yml")); err == nil {
		composeFiles = []string{"compose.yml"}
	} else if _, err := os.Stat(filepath.Join(gitDir, "docker-compose.yaml")); err == nil {
		composeFiles = []string{"docker-compose.yaml"}
	} else if _, err := os.Stat(filepath.Join(gitDir, "docker-compose.yml")); err == nil {
		composeFiles = []string{"docker-compose.yml"}
	} else {
		// TODO: Communicate to user that no Docker Compose files were found
		slog.Warn("No Docker Compose files found in repository", "git_url", project.GitURL, "repo_dir", gitDir)
	}

	project.ComposeFiles = composeFiles

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

func (s *ProjectService) DeployProject(projectID uuid.UUID, pull bool) (*Deployment, error) {
	// Get project
	project, err := s.GetProject(projectID)
	if err != nil {
		return nil, fmt.Errorf("project not found: %w", err)
	}

	// Pull latest changes if requested
	if pull {
		if err := s.pullLatestChanges(project); err != nil {
			return nil, fmt.Errorf("failed to pull latest changes: %w", err)
		}
	}

	commitHash, err := s.gitService.GetLatestCommit(project.WorkingDir)
	if err != nil {
		slog.Error("Failed to get latest commit", "project_id", project.ID, "error", err)
		return nil, fmt.Errorf("failed to get latest commit: %w", err)
	}

	deployment := NewDeployment(projectID, commitHash)

	// Deploy using Docker Compose
	slog.Info("Starting Docker Compose deployment",
		"project_id", project.ID,
		"project_name", project.Name,
		"compose_files", project.ComposeFiles,
		"pull", pull)

	result, err := s.dockerComposeService.Up(project)
	if err != nil {
		slog.Error("Docker Compose deployment failed",
			"project_id", project.ID,
			"error", err,
			"result", result)
		return nil, fmt.Errorf("docker compose command failed: %w", err)
	}

	slog.Info("Docker Compose deployment completed", "project_id", project.ID)

	// Update deployment
	deployment.Status = DeploymentStatusCompleted
	deployment.Output = result.Output

	// Update project
	project.Status = ProjectStatusRunning
	project.LastCommit = &commitHash

	// TODO: Transaction
	if err := s.deploymentRepository.Save(&deployment); err != nil {
		return nil, fmt.Errorf("failed to update deployment record: %w", err)
	}

	if _, err := s.projectRepository.Create(project); err != nil {
		return nil, fmt.Errorf("failed to update project status: %w", err)
	}

	return &deployment, nil
}

func (s *ProjectService) StopProject(projectID uuid.UUID) error {
	// Get project
	project, err := s.GetProject(projectID)
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
	output, err := s.dockerComposeService.Down(project)
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

	if _, err := s.projectRepository.Create(project); err != nil {
		return fmt.Errorf("failed to update project status: %w", err)
	}

	return nil
}

func (s *ProjectService) RemoveProject(projectID uuid.UUID) error {
	// Get project
	project, err := s.GetProject(projectID)
	if err != nil {
		return fmt.Errorf("project not found: %w", err)
	}

	// Stop Docker Compose project if running
	if err := s.StopProject(projectID); err != nil {
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

func (s *ProjectService) pullLatestChanges(project *Project) error {
	slog.Info("Pulling latest changes", "project_id", project.ID, "git_url", project.GitURL)

	err := s.gitService.Pull(project.WorkingDir)
	if err != nil {
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
	dockerComposeService DockerComposeExecutor,
	config *Config,
) *ProjectService {
	return &ProjectService{
		projectRepository:    projectRepository,
		deploymentRepository: deploymentRepository,
		gitService:           gitService,
		dockerComposeService: dockerComposeService,
		config:               config,
	}
}

// NewProjectServiceWithDefaults creates a ProjectService with default implementations
func NewProjectServiceWithDefaults(
	projectRepository ProjectRepository,
	deploymentRepository DeploymentRepository,
	config *Config,
) *ProjectService {
	return NewProjectService(
		projectRepository,
		deploymentRepository,
		&GitService{},
		&DockerComposeProjectService{},
		config,
	)
}
