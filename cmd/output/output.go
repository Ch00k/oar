// Package output provides functions to print messages with optional color formatting
package output

import (
	"fmt"
	"strings"

	"github.com/ch00k/oar/services"

	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/renderer"
	"github.com/olekukonko/tablewriter/tw"
)

const (
	Plain   = color.FgWhite
	Success = color.FgGreen
	Warning = color.FgYellow
	Error   = color.FgRed
)

var maybeColorize func(kind color.Attribute, tmpl string, a ...any) string

// InitColors sets up color functions based on environment
func InitColors(isColorDisabled bool) {
	// Check if colors should be enabled
	if color.NoColor || isColorDisabled {
		// Fallback to plain formatting if colors are not supported
		maybeColorize = func(kind color.Attribute, tmpl string, a ...any) string {
			return fmt.Sprintf(tmpl, a...)
		}
	} else {
		// Enable colors
		maybeColorize = func(kind color.Attribute, tmpl string, a ...any) string {
			return color.New(kind).SprintfFunc()(tmpl, a...)
		}
	}
}

// PrintMessage formats a message with color (if enabled) and prints it
func PrintMessage(kind color.Attribute, tmpl string, a ...any) string {
	if maybeColorize == nil || kind == Plain {
		return fmt.Sprintf(tmpl+"\n", a...)
	} else {
		// TODO: Print warnings and errors to stderr?
		return fmt.Sprintln(maybeColorize(kind, tmpl, a...))
	}
}

func PrintTable(header []string, data [][]string) (string, error) {
	buf := strings.Builder{}

	table := tablewriter.NewTable(
		&buf,
		tablewriter.WithRenderer(renderer.NewBlueprint(tw.Rendition{
			Borders: tw.BorderNone,
			Settings: tw.Settings{
				Lines: tw.Lines{
					ShowHeaderLine: tw.Off,
				},
				Separators: tw.Separators{
					BetweenColumns: tw.Off,
				},
			},
		})),
		tablewriter.WithConfig(tablewriter.Config{
			Row: tw.CellConfig{
				Alignment: tw.CellAlignment{PerColumn: []tw.Align{tw.AlignRight, tw.AlignLeft}},
			},
		}))

	if len(header) > 0 {
		table.Header(header)
	}

	if err := table.Bulk(data); err != nil {
		return "", fmt.Errorf("bulk adding data to table: %w", err)
	}

	if err := table.Render(); err != nil {
		return "", fmt.Errorf("rendering table: %w", err)
	}

	return buf.String(), nil
}

func PrintProjectDetails(project *services.Project, short bool) (string, error) {
	gitDir, err := project.GitDir()
	if err != nil {
		return "", fmt.Errorf("failed to get git directory: %w", err)
	}

	data := [][]string{
		{"ID", project.ID.String()},
		{"Name", project.Name},
		{"Working Directory", project.WorkingDir},
	}

	if !short {
		data = append(
			data,
			[][]string{
				{"Git Directory", gitDir},
				{"Git URL", project.GitURL},
				{"Compose Files", strings.Join(project.ComposeFiles, "\n")},
				{"Environment Files", strings.Join(project.EnvironmentFiles, "\n")},
			}...,
		)
	}
	data = append(data,
		[]string{"Status", project.Status.String()},
	)
	if !short {
		data = append(data,
			[][]string{
				{"Last Commit", project.LastCommitStr()},
				{"Created At", project.CreatedAt.Format("2006-01-02 15:04:05")},
				{"Updated At", project.UpdatedAt.Format("2006-01-02 15:04:05")},
			}...,
		)
	}

	table, err := PrintTable([]string{}, data)
	if err != nil {
		return "", fmt.Errorf("printing project details table: %w", err)
	}
	return table, nil
}

func PrintProjectList(projects []*services.Project) (string, error) {
	if len(projects) == 0 {
		return PrintMessage(Plain, "No projects found."), nil
	}

	header := []string{
		"ID",
		"Name",
		//"Working Directory",
		//"Git Directory",
		//"Git URL",
		//"Compose Files",
		//"Environment Files",
		"Status",
		//"Last Commit",
		"Created At",
		"Updated At",
	}
	var data [][]string
	for _, project := range projects {
		//gitDir, err := project.GitDir()
		//if err != nil {
		//    return fmt.Errorf("failed to get git directory: %w", err)
		//}

		data = append(data, []string{
			project.ID.String(),
			project.Name,
			// project.WorkingDir,
			// gitDir,
			// project.GitURL,
			// strings.Join(project.ComposeFiles, ", "),
			// strings.Join(project.EnvironmentFiles, ", "),
			// project.LastCommitStr(),
			project.Status.String(),
			project.CreatedAt.Format("2006-01-02 15:04:05"),
			project.UpdatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	table, err := PrintTable(header, data)
	if err != nil {
		return "", fmt.Errorf("printing project list table: %w", err)
	}

	return table, nil
}
