// Package test provides utility functions for testing Oar CLI
package test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func ReadFile(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		fmt.Println("Error reading file:", err)
		return "", err
	}
	return string(content), nil
}

func RenderTemplate(data map[string]any) string {
	tmplContent, err := ReadFile("testdata/output/project_add.golden")
	if err != nil {
		fmt.Println("Error reading golden file:", err)
		return ""
	}
	tmpl, err := template.New("add").Parse(tmplContent)
	if err != nil {
		fmt.Println("Error parsing template:", err)
		return ""
	}

	var builder strings.Builder

	if err := tmpl.Execute(&builder, data); err != nil {
		fmt.Println("Error executing template:", err)
		return ""
	}
	return builder.String()
}

type RepoFile struct {
	Path    string
	Content string
}

func InitGitRepo(path string, files []RepoFile) (*git.Repository, error) {
	repo, err := git.PlainInit(path, false)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize git repository: %w", err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("failed to get worktree: %w", err)
	}

	if err := AddRepoFiles(worktree, files); err != nil {
		return nil, fmt.Errorf("failed to add files to git repository: %w", err)
	}

	if _, err := worktree.Commit("Initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "John Doe",
			Email: "john@doe.org",
			When:  time.Now(),
		},
	}); err != nil {
		return nil, fmt.Errorf("failed to commit changes: %w", err)
	}

	return repo, nil
}

func AddRepoFiles(repoWorktree *git.Worktree, files []RepoFile) error {
	repoDir := repoWorktree.Filesystem.Root()

	for _, file := range files {
		filePath := filepath.Join(repoDir, file.Path)
		if err := os.WriteFile(filePath, []byte(file.Content), 0o644); err != nil {
			return fmt.Errorf("failed to write file %s: %w", file.Path, err)
		}
		if _, err := repoWorktree.Add(file.Path); err != nil {
			return fmt.Errorf("failed to add file %s to git: %w", file.Path, err)
		}
	}

	return nil
}

// Trim trims trailing spaces left by tablewriter on each line to make the lines length-aligned
func Trim(input string) string {
	lines := strings.Split(input, "\n")

	for i, line := range lines {
		// Remove only trailing spaces and tabs, preserve other whitespace
		lines[i] = strings.TrimRight(line, " \n")
	}

	return strings.Join(lines, "\n")
}
