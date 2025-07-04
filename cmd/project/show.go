package project

import (
	"fmt"

	"github.com/ch00k/oar/cmd/output"
	"github.com/ch00k/oar/cmd/utils"
	"github.com/ch00k/oar/internal/app"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

func NewCmdProjectShow() *cobra.Command {
	return &cobra.Command{
		Use:   "show <project-id>",
		Short: "Show detailed project information",
		Long:  "Display comprehensive information about a project including configuration, deployment history, and current status.",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) < 1 {
				if err := cmd.Help(); err != nil {
					utils.HandleCommandError("showing help", err)
				}
				return
			}

			projectID, err := uuid.Parse(args[0])
			if err != nil {
				utils.HandleInvalidUUID("project operation", args[0])
				return
			}

			project, err := app.GetProjectService().GetProject(projectID)
			if err != nil {
				utils.HandleCommandError("retrieving project", err, "project_id", projectID)
				return
			}

			out, err := output.PrintProjectDetails(project, false)
			if err != nil {
				utils.HandleCommandError("printing project details table", err)
			}

			if _, err := fmt.Fprintln(cmd.OutOrStdout(), out); err != nil {
				utils.HandleCommandError("printing project details", err, "project_id", projectID)
			}
		},
	}
}
