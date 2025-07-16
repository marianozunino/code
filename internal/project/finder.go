package project

import (
	"os"
	"path/filepath"
	"strings"
)

func FindProjects(devDir string) []string {
	var projects []string

	filepath.Walk(devDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if !info.IsDir() {
			return nil
		}

		if isGitRepo(path) {
			relPath := strings.TrimPrefix(path, devDir)
			relPath = strings.TrimPrefix(relPath, string(filepath.Separator))
			projects = append(projects, relPath)
			return filepath.SkipDir
		}

		return nil
	})

	return projects
}

// Helper function to check if a directory is a git repository
func isGitRepo(path string) bool {
	gitPath := filepath.Join(path, ".git")
	stat, err := os.Stat(gitPath)
	return err == nil && stat.IsDir()
}

// RemoveDuplicates removes duplicate strings from a slice
func RemoveDuplicates(items []string) []string {
	if len(items) == 0 {
		return items
	}

	seen := make(map[string]bool, len(items))
	result := make([]string, 0, len(items))

	for _, item := range items {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}

	return result
}
