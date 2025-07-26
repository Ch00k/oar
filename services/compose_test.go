package services

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function to extract message content from JSON streaming output
func extractMessageFromJSON(jsonStr string) (string, error) {
	var msg struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &msg); err != nil {
		return "", err
	}
	return msg.Message, nil
}

// Tests for NewComposeProject
func TestNewComposeProject_Success(t *testing.T) {
	// Create test project with working directory
	testProject := createTestProjectWithOptions(ProjectOptions{
		Name:             "test-compose-project",
		WorkingDir:       "/tmp/test-compose-project",
		ComposeFiles:     []string{"docker-compose.yml", "docker-compose.override.yml"},
		EnvironmentFiles: []string{".env", ".env.local"},
	})
	tempDir := t.TempDir()
	testProject.WorkingDir = filepath.Join(tempDir, testProject.ID.String())

	// Create the working directory and git subdirectory
	gitDir := filepath.Join(testProject.WorkingDir, GitDir)
	err := os.MkdirAll(gitDir, 0o755)
	require.NoError(t, err)

	// Test
	composeProject := NewComposeProject(testProject)

	// Assertions
	assert.NotNil(t, composeProject)
	assert.Equal(t, testProject.Name, composeProject.Name)
	assert.Equal(t, gitDir, composeProject.WorkingDir)
	assert.Equal(t, testProject.ComposeFiles, composeProject.ComposeFiles)
	assert.Equal(t, testProject.EnvironmentFiles, composeProject.EnvironmentFiles)
}

func TestNewComposeProject_InvalidProject(t *testing.T) {
	// Create project with invalid working directory
	testProject := createTestProjectWithOptions(ProjectOptions{
		Name:             "test-compose-project",
		WorkingDir:       "/tmp/test-compose-project",
		ComposeFiles:     []string{"docker-compose.yml", "docker-compose.override.yml"},
		EnvironmentFiles: []string{".env", ".env.local"},
	})
	testProject.WorkingDir = "" // Invalid working directory

	// Test
	composeProject := NewComposeProject(testProject)

	// Assertions
	assert.Nil(t, composeProject)
}

// Tests for ComposeProject.prepareCommand
func TestComposeProject_PrepareCommand_Basic(t *testing.T) {
	composeProject := createTestComposeProject()
	tempDir := t.TempDir()
	composeProject.WorkingDir = tempDir

	// Test
	cmd, err := composeProject.prepareCommand("up", []string{"--detach"})

	// Assertions
	assert.NoError(t, err)
	assert.NotNil(t, cmd)
	assert.Contains(t, cmd.Path, "docker") // Path may be full path like /usr/bin/docker
	assert.Equal(t, tempDir, cmd.Dir)

	// Verify command arguments
	expectedArgs := []string{
		"docker", // cmd.Args[0] is always the command name
		"compose",
		"--project-name", "test-project",
		"--file", filepath.Join(tempDir, "docker-compose.yml"),
		"up",
		"--detach",
	}
	assert.Equal(t, expectedArgs, cmd.Args)
}

func TestComposeProject_PrepareCommand_MultipleFiles(t *testing.T) {
	composeProject := createTestComposeProject()
	composeProject.ComposeFiles = []string{
		"docker-compose.yml",
		"docker-compose.override.yml",
		"docker-compose.prod.yml",
	}
	tempDir := t.TempDir()
	composeProject.WorkingDir = tempDir

	// Test
	cmd, err := composeProject.prepareCommand("down", []string{"--remove-orphans"})

	// Assertions
	assert.NoError(t, err)
	assert.NotNil(t, cmd)

	// Verify all compose files are included
	expectedArgs := []string{
		"docker",
		"compose",
		"--project-name", "test-project",
		"--file", filepath.Join(tempDir, "docker-compose.yml"),
		"--file", filepath.Join(tempDir, "docker-compose.override.yml"),
		"--file", filepath.Join(tempDir, "docker-compose.prod.yml"),
		"down",
		"--remove-orphans",
	}
	assert.Equal(t, expectedArgs, cmd.Args)
}

func TestComposeProject_PrepareCommand_NoFiles(t *testing.T) {
	composeProject := createTestComposeProject()
	composeProject.ComposeFiles = []string{} // No compose files
	tempDir := t.TempDir()
	composeProject.WorkingDir = tempDir

	// Test
	cmd, err := composeProject.prepareCommand("ps", []string{})

	// Assertions
	assert.NoError(t, err)
	assert.NotNil(t, cmd)

	// Should still work with no files (docker compose will use defaults)
	expectedArgs := []string{
		"docker",
		"compose",
		"--project-name", "test-project",
		"ps",
	}
	assert.Equal(t, expectedArgs, cmd.Args)
}

// Tests for specific command builders
func TestComposeProject_CommandUp(t *testing.T) {
	composeProject := createTestComposeProject()
	tempDir := t.TempDir()
	composeProject.WorkingDir = tempDir

	// Test
	cmd, err := composeProject.commandUp()

	// Assertions
	assert.NoError(t, err)
	assert.NotNil(t, cmd)

	// Verify up-specific arguments
	args := cmd.Args
	assert.Contains(t, args, "up")
	assert.Contains(t, args, "--detach")
	assert.Contains(t, args, "--wait")
	assert.Contains(t, args, "--quiet-pull")
	assert.Contains(t, args, "--no-color")
	assert.Contains(t, args, "--remove-orphans")
}

func TestComposeProject_CommandDown(t *testing.T) {
	composeProject := createTestComposeProject()
	tempDir := t.TempDir()
	composeProject.WorkingDir = tempDir

	// Test
	cmd, err := composeProject.commandDown()

	// Assertions
	assert.NoError(t, err)
	assert.NotNil(t, cmd)

	// Verify down-specific arguments
	args := cmd.Args
	assert.Contains(t, args, "down")
	assert.Contains(t, args, "--remove-orphans")
}

func TestComposeProject_CommandLogs(t *testing.T) {
	composeProject := createTestComposeProject()
	tempDir := t.TempDir()
	composeProject.WorkingDir = tempDir

	// Test
	cmd, err := composeProject.commandLogs()

	// Assertions
	assert.NoError(t, err)
	assert.NotNil(t, cmd)

	// Verify logs-specific arguments
	args := cmd.Args
	assert.Contains(t, args, "logs")
	assert.Contains(t, args, "--follow")
}

// Tests for executeCommand (using real commands that are safe)
func TestComposeProject_ExecuteCommand_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping command execution test in short mode")
	}

	composeProject := createTestComposeProject()

	// Use a simple command that should work on most systems
	cmd := exec.Command("echo", "test output")

	// Test
	output, err := composeProject.executeCommand(cmd)

	// Assertions
	assert.NoError(t, err)
	assert.Equal(t, "test output\n", output)
}

func TestComposeProject_ExecuteCommand_Error(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping command execution test in short mode")
	}

	composeProject := createTestComposeProject()

	// Use a command that should fail
	cmd := exec.Command("false") // 'false' command always exits with code 1

	// Test
	output, err := composeProject.executeCommand(cmd)

	// Assertions
	assert.Error(t, err)
	assert.Empty(t, output)
}

// Tests for executeCommandStreaming
func TestComposeProject_ExecuteCommandStreaming_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping streaming command test in short mode")
	}

	composeProject := createTestComposeProject()

	// Create a command that outputs multiple lines
	cmd := exec.Command("sh", "-c", "echo 'line1'; echo 'line2'; echo 'line3'")

	// Create output channel
	outputChan := make(chan string, 10)
	var receivedLines []string

	// Start goroutine to collect output
	done := make(chan bool)
	go func() {
		for line := range outputChan {
			receivedLines = append(receivedLines, line)
		}
		done <- true
	}()

	// Test
	err := composeProject.executeCommandStreaming(cmd, outputChan)
	close(outputChan)

	// Wait for output collection to complete
	<-done

	// Assertions
	assert.NoError(t, err)
	assert.Len(t, receivedLines, 3)

	// Extract messages from JSON and verify content
	var extractedMessages []string
	for _, line := range receivedLines {
		message, err := extractMessageFromJSON(line)
		assert.NoError(t, err)
		extractedMessages = append(extractedMessages, message)
	}

	assert.Contains(t, extractedMessages, "line1")
	assert.Contains(t, extractedMessages, "line2")
	assert.Contains(t, extractedMessages, "line3")
}

func TestComposeProject_ExecuteCommandStreaming_Error(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping streaming command test in short mode")
	}

	composeProject := createTestComposeProject()

	// Use a command that should fail
	cmd := exec.Command("false")

	// Create output channel
	outputChan := make(chan string, 10)

	// Test
	err := composeProject.executeCommandStreaming(cmd, outputChan)
	close(outputChan)

	// Assertions
	assert.Error(t, err)
}

// Tests for high-level operations (Up, Down, Logs)
// Note: These tests use mocked Docker commands since we can't assume Docker is available

func TestComposeProject_Up_MockDocker(t *testing.T) {
	// Skip if Docker is not available (this is more of an integration test)
	t.Skip("Skipping Docker integration test - would require Docker to be available")

	composeProject := createTestComposeProject()
	tempDir := t.TempDir()
	composeProject.WorkingDir = tempDir

	// Create a mock compose file
	composeContent := `version: '3'
services:
  test:
    image: hello-world
`
	err := os.WriteFile(filepath.Join(tempDir, "docker-compose.yml"), []byte(composeContent), 0o644)
	require.NoError(t, err)

	// Test (this would require Docker to be running)
	output, err := composeProject.Up()

	// Assertions (these would need to be adjusted based on actual Docker behavior)
	// This is more of a placeholder for integration testing
	_ = output
	_ = err
}

// Tests for edge cases and error conditions
func TestComposeProject_EmptyProjectName(t *testing.T) {
	composeProject := createTestComposeProject()
	composeProject.Name = ""
	tempDir := t.TempDir()
	composeProject.WorkingDir = tempDir

	// Test
	cmd, err := composeProject.prepareCommand("up", []string{})

	// Assertions
	assert.NoError(t, err)
	assert.NotNil(t, cmd)

	// Should still create command with empty project name
	assert.Contains(t, cmd.Args, "--project-name")
	assert.Contains(t, cmd.Args, "")
}

func TestComposeProject_InvalidWorkingDirectory(t *testing.T) {
	composeProject := createTestComposeProject()
	composeProject.WorkingDir = "/non/existent/directory"

	// Test
	cmd, err := composeProject.prepareCommand("up", []string{})

	// Assertions
	assert.NoError(t, err) // prepareCommand doesn't validate directory existence
	assert.NotNil(t, cmd)
	assert.Equal(t, "/non/existent/directory", cmd.Dir)
}

// Tests for streaming operations with channels
func TestComposeProject_StreamingChannelManagement(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping streaming test in short mode")
	}

	composeProject := createTestComposeProject()

	// Test that streaming properly handles channel operations
	outputChan := make(chan string, 10)

	// Create a simple command for testing
	cmd := exec.Command("echo", "streaming test")

	// Test
	err := composeProject.executeCommandStreaming(cmd, outputChan)

	// Close channel and collect output
	close(outputChan)
	var output []string
	for line := range outputChan {
		output = append(output, line)
	}

	// Assertions
	assert.NoError(t, err)
	assert.Len(t, output, 1)

	// Extract message from JSON and verify content
	message, err := extractMessageFromJSON(output[0])
	assert.NoError(t, err)
	assert.Equal(t, "streaming test", message)
}

// Test for concurrent streaming operations
func TestComposeProject_ConcurrentStreaming(t *testing.T) {
	t.Skip("Skipping - flaky test in CI")
	if testing.Short() {
		t.Skip("Skipping concurrent streaming test in short mode")
	}

	composeProject := createTestComposeProject()

	// Number of concurrent operations
	numOps := 3

	// Channels for coordination
	done := make(chan error, numOps)

	// Start multiple streaming operations concurrently
	for i := range numOps {
		go func(id int) {
			outputChan := make(chan string, 10)
			// Use a more reliable command that should always produce output
			cmd := exec.Command("sh", "-c", fmt.Sprintf("printf 'concurrent test %d\\n'", id))

			err := composeProject.executeCommandStreaming(cmd, outputChan)
			close(outputChan)

			// Collect output to ensure it completes
			var lines []string
			for line := range outputChan {
				lines = append(lines, line)
			}

			if err != nil {
				done <- fmt.Errorf("command failed for operation %d: %w", id, err)
				return
			}

			expectedOutput := fmt.Sprintf("concurrent test %d", id)
			if len(lines) != 1 {
				done <- fmt.Errorf("unexpected number of lines for operation %d: got %d, expected 1", id, len(lines))
				return
			}

			// Extract message from JSON and verify content
			message, err := extractMessageFromJSON(lines[0])
			if err != nil {
				done <- fmt.Errorf("failed to parse JSON for operation %d: %w", id, err)
				return
			}

			if message != expectedOutput {
				done <- fmt.Errorf("unexpected output for operation %d: got %s, expected %s", id, message, expectedOutput)
				return
			}

			done <- nil
		}(i)
	}

	// Wait for all operations to complete
	for range numOps {
		select {
		case err := <-done:
			assert.NoError(t, err)
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for concurrent operations")
		}
	}
}
