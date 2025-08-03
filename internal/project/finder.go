package project

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

func FindProjects(devDir string) []string {
	maxDepth := 3
	var projects []string
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Use worker pool to limit goroutines
	semaphore := make(chan struct{}, runtime.NumCPU()*2)

	filepath.WalkDir(devDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil
		}

		// Skip if too deep
		depth := strings.Count(strings.TrimPrefix(path, devDir), string(filepath.Separator))
		if depth > maxDepth {
			return filepath.SkipDir
		}

		// Check for .git concurrently
		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			if isGitRepo(p) {
				relPath := strings.TrimPrefix(p, devDir+string(filepath.Separator))
				mu.Lock()
				projects = append(projects, relPath)
				mu.Unlock()
			}
		}(path)

		return nil
	})

	wg.Wait()
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
