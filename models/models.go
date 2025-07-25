// Package models provides the database models for Oar.
package models

import (
	"time"

	"github.com/google/uuid"
)

type BaseModel struct {
	ID        uuid.UUID `gorm:"type:char(36);primaryKey"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

type ProjectModel struct {
	BaseModel
	Name             string `gorm:"not null;unique"`
	GitURL           string `gorm:"not null"`
	WorkingDir       string `gorm:"not null"` // directory where the project is cloned
	ComposeFiles     string `gorm:"not null"` // list of compose file paths separated by null character (\0)
	EnvironmentFiles string `gorm:"not null"` // list of environment file paths separated by null character (\0)
	Status           string `gorm:"not null"` // running, stopped, error
	LastCommit       *string

	Deployments []DeploymentModel `gorm:"foreignKey:ProjectID;constraint:OnDelete:CASCADE"`
}

type DeploymentModel struct {
	BaseModel
	ProjectID   uuid.UUID `gorm:"not null;index"`
	CommitHash  string    `gorm:"not null"`
	CommandLine string    `gorm:"not null"`  // Command executed for deployment
	Status      string    `gorm:"not null"`  // in_progress, success, failed
	Output      string    `gorm:"type:text"` // Command output/logs

	Project ProjectModel `gorm:"foreignKey:ProjectID;constraint:OnDelete:CASCADE"`
}
