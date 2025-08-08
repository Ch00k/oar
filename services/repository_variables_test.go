package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestProjectRepository_ClearVariables tests that setting empty variables
// actually clears existing variables in the database
func TestProjectRepository_ClearVariables(t *testing.T) {
	db := setupTestDB(t)
	encryption := setupTestEncryption(t)
	repo := NewProjectRepository(db, encryption)

	// Create a project with some variables
	project := &Project{
		Name:         "test-clear-vars",
		GitURL:       "https://github.com/test/repo.git",
		WorkingDir:   "/tmp/test",
		ComposeFiles: []string{"docker-compose.yml"},
		Variables:    []string{"VAR1=value1", "VAR2=value2", "VAR3=value3"},
		Status:       ProjectStatusStopped,
	}

	// Create the project
	createdProject, err := repo.Create(project)
	require.NoError(t, err)
	assert.Len(t, createdProject.Variables, 3)

	// Verify variables are stored
	retrievedProject, err := repo.FindByID(createdProject.ID)
	require.NoError(t, err)
	assert.Len(t, retrievedProject.Variables, 3)
	assert.Contains(t, retrievedProject.Variables, "VAR1=value1")
	assert.Contains(t, retrievedProject.Variables, "VAR2=value2")
	assert.Contains(t, retrievedProject.Variables, "VAR3=value3")

	// Clear the variables (set to empty slice)
	retrievedProject.Variables = []string{}

	// Update the project
	err = repo.Update(retrievedProject)
	require.NoError(t, err)

	// Retrieve the project again to verify variables are cleared
	updatedProject, err := repo.FindByID(createdProject.ID)
	require.NoError(t, err)
	assert.Len(t, updatedProject.Variables, 0, "Variables should be empty after clearing")
}

// TestProjectRepository_UpdateEmptyVariables tests updating various field combinations
// including empty strings to ensure GORM Updates with Select("*") works correctly
func TestProjectRepository_UpdateEmptyVariables(t *testing.T) {
	db := setupTestDB(t)
	encryption := setupTestEncryption(t)
	repo := NewProjectRepository(db, encryption)

	// Create a project with initial data
	project := &Project{
		Name:         "test-update-empty",
		GitURL:       "https://github.com/test/repo.git",
		WorkingDir:   "/tmp/test",
		ComposeFiles: []string{"docker-compose.yml", "docker-compose.override.yml"},
		Variables:    []string{"VAR1=value1", "VAR2=value2"},
		Status:       ProjectStatusStopped,
	}

	// Create the project
	createdProject, err := repo.Create(project)
	require.NoError(t, err)

	// Update with empty variables and files
	createdProject.Variables = []string{}
	createdProject.ComposeFiles = []string{}

	// Update the project
	err = repo.Update(createdProject)
	require.NoError(t, err)

	// Retrieve and verify all fields are updated correctly
	updatedProject, err := repo.FindByID(createdProject.ID)
	require.NoError(t, err)

	assert.Len(t, updatedProject.Variables, 0, "Variables should be empty")
	assert.Len(t, updatedProject.ComposeFiles, 0, "ComposeFiles should be empty")
	assert.Equal(t, createdProject.Name, updatedProject.Name)
	assert.Equal(t, createdProject.GitURL, updatedProject.GitURL)
	assert.Equal(t, createdProject.WorkingDir, updatedProject.WorkingDir)
}
