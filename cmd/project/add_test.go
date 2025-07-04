package project

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ch00k/oar/cmd/test"
	"github.com/ch00k/oar/internal/app"
	"github.com/ch00k/oar/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testOptions struct {
	gitURL string
	name   string
}

func TestProjectAddHappy(t *testing.T) {
	tests := []struct {
		name           string
		options        testOptions
		composeFiles   []string
		expectedOutput string
		expectError    string
	}{
		{
			name: "Success",
			options: testOptions{
				gitURL: "",
				name:   "test-project",
			},
			composeFiles:   []string{"testdata/composefiles/compose.yaml"},
			expectedOutput: "testdata/output/project_add.golden",
			expectError:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory for test data
			tempDir, err := os.MkdirTemp("", "oar-test-*")
			require.NoError(t, err)
			defer os.RemoveAll(tempDir) // nolint: errcheck

			// Setup a test git repository
			repoDir := filepath.Join(tempDir, "test-git-repo")

			var repoFiles []test.RepoFile

			for _, file := range tt.composeFiles {
				_, fileName := filepath.Split(file)
				fileContent, err := test.ReadFile(file)
				require.NoError(t, err, "Failed to read compose file %s", file)

				repoFiles = append(repoFiles, test.RepoFile{
					Path:    fileName,
					Content: fileContent,
				})
			}

			_, err = test.InitGitRepo(repoDir, repoFiles)
			assert.NoError(t, err, "Failed to initialize git repository")

			// Initialize the app with test data directory
			err = app.Initialize(tempDir)
			require.NoError(t, err)

			// Build args
			args := []string{"--git-url", repoDir, "--name", tt.options.name}

			var stdout, stderr bytes.Buffer
			rootCmd := NewCmdProjectAdd()
			rootCmd.SetOut(&stdout)
			rootCmd.SetErr(&stderr)
			rootCmd.SetArgs(args)

			// Debug: Print the command being executed
			t.Logf("Executing command: oar %s", strings.Join(args, " "))

			err = rootCmd.Execute()

			stdoutStr := test.Trim(stdout.String())
			stderrStr := stderr.String()

			if err != nil {
				t.Logf("Command execution failed: %s", err)
				t.Logf("Standard Output: %s", stdoutStr)
				t.Logf("Standard Error: %s", stderrStr)
				t.FailNow()
			}

			createdProject, err := app.GetProjectService().GetProjectByName(tt.options.name)

			// Verify results
			assert.NoError(t, err, "Expected command to succeed")
			data := map[string]any{
				"ProjectID":   createdProject.ID.String(),
				"ProjectName": createdProject.Name,
				"ProjectsDir": filepath.Join(tempDir, services.ProjectsDir),
			}
			expectedOutput := test.RenderTemplate(data)
			t.Logf("Expected output: %s", expectedOutput)
			t.Logf("Actual output: %s", stdoutStr)
			assert.Equal(t, expectedOutput, stdoutStr, "Output should match expected format")
		})
	}
}
