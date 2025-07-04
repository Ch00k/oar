package project

import (
	"fmt"
	"log/slog"

	"github.com/ch00k/oar/cmd/output"
	"github.com/ch00k/oar/cmd/utils"
	"github.com/ch00k/oar/internal/app"
	"github.com/ch00k/oar/services"
	"github.com/spf13/cobra"
)

func NewCmdProjectAdd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a Git repository as a managed project",
		Long: `Add a new Docker Compose project from a Git repository.
Oar will clone the repository and detect Docker Compose files automatically.`,
		Run: func(cmd *cobra.Command, args []string) {
			// Get flag values
			gitURL, _ := cmd.Flags().GetString("git-url")
			name, _ := cmd.Flags().GetString("name")
			composeFiles, _ := cmd.Flags().GetStringArray("compose-file")
			envFiles, _ := cmd.Flags().GetStringArray("env-file")

			// Create project struct from CLI input
			project := services.NewProject(name, gitURL, composeFiles, envFiles)

			// Call service
			createdProject, err := app.GetProjectService().CreateProject(&project)
			if err != nil {
				utils.HandleCommandError("creating project from %s", err, "Git URL", gitURL)
				return
			}

			out, err := output.PrintProjectDetails(createdProject, true)
			if err != nil {
				utils.HandleCommandError("printing project details table", err)
			}

			if _, err := fmt.Fprintln(cmd.OutOrStdout(), out); err != nil {
				utils.HandleCommandError("printing project details", err)
			}
		},
	}

	cmd.Flags().StringP("git-url", "u", "", "Git repository URL")
	cmd.Flags().StringP("name", "n", "", "Custom project name (auto-detected if not specified)")
	cmd.Flags().StringArrayP("compose-file", "f", nil, "Docker Compose file path (relative to repository root)")
	cmd.Flags().StringArrayP("env-file", "e", nil, "Environment file path (absolute)")
	if err := cmd.MarkFlagRequired("git-url"); err != nil {
		slog.Error("Failed to mark git-url flag as required", "error", err)
		panic(fmt.Sprintf("CLI setup error: %v", err)) // This is a setup error, should panic
	}
	return cmd
}
