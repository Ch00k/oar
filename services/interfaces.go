package services

import (
	"github.com/google/uuid"
)

// GitExecutor defines the contract for Git operations
type GitExecutor interface {
	Clone(gitURL, workingDir string) error
	Pull(workingDir string) error
	GetLatestCommit(workingDir string) (string, error)
}

// DockerComposeExecutor defines the contract for Docker Compose operations
type DockerComposeExecutor interface {
	Up(project *Project) (*DeploymentResult, error)
	Down(project *Project) (string, error)
}

// ProjectManager defines the contract for project management operations
type ProjectManager interface {
	ListProjects() ([]*Project, error)
	GetProject(id uuid.UUID) (*Project, error)
	GetProjectByName(name string) (*Project, error)
	CreateProject(project *Project) (*Project, error)
	DeployProject(projectID uuid.UUID, pull bool) (*Deployment, error)
	StopProject(projectID uuid.UUID) error
	RemoveProject(projectID uuid.UUID) error
}
