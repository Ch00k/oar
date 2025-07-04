package services

import (
	"strings"

	"github.com/ch00k/oar/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ProjectRepository interface {
	FindByID(id uuid.UUID) (*Project, error)
	FindByName(name string) (*Project, error)
	Create(project *Project) (*Project, error)
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
		return nil, err
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
		return nil, res.Error
	}
	return r.mapper.ToDomain(model), nil
}

func (r *projectRepository) Delete(id uuid.UUID) error {
	return r.db.Delete(&models.ProjectModel{}, id).Error
}

func NewProjectRepository(db *gorm.DB) ProjectRepository {
	return &projectRepository{
		db:     db,
		mapper: &ProjectMapper{},
	}
}

type DeploymentRepository interface {
	FindByID(id uuid.UUID) (*Deployment, error)
	Save(deployment *Deployment) error
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

func (r *deploymentRepository) Save(deployment *Deployment) error {
	model := r.mapper.ToModel(deployment)
	return r.db.Save(model).Error
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
