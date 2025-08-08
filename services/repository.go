package services

import (
	"log/slog"
	"strings"

	"github.com/ch00k/oar/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ProjectRepository interface {
	FindByID(id uuid.UUID) (*Project, error)
	FindByName(name string) (*Project, error)
	Create(project *Project) (*Project, error)
	Update(project *Project) error
	List() ([]*Project, error)
	Delete(id uuid.UUID) error
}

type projectRepository struct {
	db     *gorm.DB
	mapper *ProjectMapper
}

func (r *projectRepository) List() ([]*Project, error) {
	var models []models.ProjectModel
	if err := r.db.Find(&models).Error; err != nil {
		return nil, err
	}

	projects := make([]*Project, len(models))
	for i, model := range models {
		projects[i] = r.mapper.ToDomain(&model)
	}
	return projects, nil
}

func (r *projectRepository) FindByID(id uuid.UUID) (*Project, error) {
	var model models.ProjectModel
	if err := r.db.First(&model, id).Error; err != nil {
		slog.Error("Database operation failed",
			"layer", "repository",
			"operation", "find_project",
			"project_id", id,
			"error", err)
		return nil, err // Pass through as-is
	}
	return r.mapper.ToDomain(&model), nil
}

func (r *projectRepository) FindByName(name string) (*Project, error) {
	var model models.ProjectModel
	if err := r.db.Where("name = ?", name).First(&model).Error; err != nil {
		return nil, err
	}
	return r.mapper.ToDomain(&model), nil
}

func (r *projectRepository) Create(project *Project) (*Project, error) {
	model := r.mapper.ToModel(project)
	res := r.db.Create(model)
	if res.Error != nil {
		slog.Error("Database operation failed",
			"layer", "repository",
			"operation", "create_project",
			"project_id", project.ID,
			"project_name", project.Name,
			"error", res.Error)
		return nil, res.Error // Pass through as-is
	}
	return r.mapper.ToDomain(model), nil
}

func (r *projectRepository) Update(project *Project) error {
	model := r.mapper.ToModel(project)

	// Use Select to explicitly update all fields, including empty strings
	// This ensures that clearing variables (empty string) actually updates the database
	return r.db.Model(&models.ProjectModel{}).Where("id = ?", model.ID).Select("*").Updates(model).Error
}

func (r *projectRepository) Delete(id uuid.UUID) error {
	err := r.db.Delete(&models.ProjectModel{}, id).Error
	if err != nil {
		slog.Error("Database operation failed",
			"layer", "repository",
			"operation", "delete_project",
			"project_id", id,
			"error", err)
	}
	return err // Pass through as-is
}

func NewProjectRepository(db *gorm.DB, encryption *EncryptionService) ProjectRepository {
	return &projectRepository{
		db:     db,
		mapper: NewProjectMapper(encryption),
	}
}

type DeploymentRepository interface {
	FindByID(id uuid.UUID) (*Deployment, error)
	Create(deployment *Deployment) error
	ListByProjectID(projectID uuid.UUID) ([]*Deployment, error)
}

type deploymentRepository struct {
	db     *gorm.DB
	mapper *DeploymentMapper
}

func (r *deploymentRepository) FindByID(id uuid.UUID) (*Deployment, error) {
	var model models.DeploymentModel
	if err := r.db.First(&model, id).Error; err != nil {
		return nil, err
	}
	return r.mapper.ToDomain(&model), nil
}

func (r *deploymentRepository) Create(deployment *Deployment) error {
	model := r.mapper.ToModel(deployment)
	return r.db.Create(model).Error
}

func (r *deploymentRepository) ListByProjectID(projectID uuid.UUID) ([]*Deployment, error) {
	var models []models.DeploymentModel
	if err := r.db.Where("project_id = ?", projectID).Find(&models).Error; err != nil {
		return nil, err
	}

	deployments := make([]*Deployment, len(models))
	for i, model := range models {
		deployments[i] = r.mapper.ToDomain(&model)
	}
	return deployments, nil
}

func NewDeploymentRepository(db *gorm.DB) DeploymentRepository {
	return &deploymentRepository{
		db:     db,
		mapper: &DeploymentMapper{},
	}
}

// Helper functions
func parseFiles(s string) []string {
	if s == "" {
		return []string{}
	}
	return strings.Split(s, "\x00") // null-separated for better handling
}

func serializeFiles(files []string) string {
	return strings.Join(files, "\x00")
}
