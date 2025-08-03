package project

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// Directories to skip during project search
var skipDirs = map[string]bool{
	// Dependencies and package managers
	"node_modules":     true,
	"vendor":           true,
	".npm":             true,
	".yarn":            true,
	".pnpm":            true,
	"bower_components": true,

	// Build outputs and caches
	"dist":          true,
	"build":         true,
	"target":        true,
	"out":           true,
	"bin":           true,
	"obj":           true,
	".next":         true,
	".nuxt":         true,
	".output":       true,
	".cache":        true,
	".parcel-cache": true,
	"coverage":      true,
	".nyc_output":   true,

	// Version control and IDE
	".git":     true,
	".svn":     true,
	".hg":      true,
	".bzr":     true,
	".vscode":  true,
	".idea":    true,
	".eclipse": true,

	// Language-specific
	"__pycache__":         true,
	".pytest_cache":       true,
	".mypy_cache":         true,
	"venv":                true,
	".venv":               true,
	"env":                 true,
	".env":                true,
	"site-packages":       true,
	".tox":                true,
	"Pods":                true,
	"DerivedData":         true,
	".gradle":             true,
	".m2":                 true,
	"cmake-build-debug":   true,
	"cmake-build-release": true,

	// Temporary and system files
	"tmp":       true,
	"temp":      true,
	".tmp":      true,
	".DS_Store": true,
	"Thumbs.db": true,

	// Logs
	"logs":  true,
	"log":   true,
	"*.log": true,
}

// Additional patterns to skip (case-insensitive)
var skipPatterns = []string{
	"cmake-build-",
	".terraform",
	"terraform.tfstate",
}

// shouldSkipDir determines if a directory should be skipped
func shouldSkipDir(dirName string) bool {
	// Check exact matches (case-sensitive for performance)
	if skipDirs[dirName] {
		return true
	}

	// Check patterns (case-insensitive)
	lowerName := strings.ToLower(dirName)
	for _, pattern := range skipPatterns {
		if strings.Contains(lowerName, strings.ToLower(pattern)) {
			return true
		}
	}

	// Skip hidden directories that start with . (except .git which we handle separately)
	if strings.HasPrefix(dirName, ".") && dirName != ".git" {
		return true
	}

	return false
}

// isProjectRoot checks if a directory is a project root by looking for common indicators
func isProjectRoot(path string) bool {
	indicators := []string{
		".git",
		"go.mod",
		"package.json",
		"Cargo.toml",
		"pom.xml",
		"build.gradle",
		"CMakeLists.txt",
		"Makefile",
		"requirements.txt",
		"setup.py",
		"pyproject.toml",
		"composer.json",
		"Gemfile",
		"mix.exs",
		"elm.json",
		"deno.json",
		"pubspec.yaml",
	}

	for _, indicator := range indicators {
		if _, err := os.Stat(filepath.Join(path, indicator)); err == nil {
			return true
		}
	}

	return false
}

func FindProjects(devDir string) []string {
	maxDepth := 3
	var projects []string
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Use worker pool to limit goroutines
	semaphore := make(chan struct{}, runtime.NumCPU()*2)

	err := filepath.WalkDir(devDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Continue walking, ignore errors
		}

		if !d.IsDir() {
			return nil
		}

		// Calculate depth
		relPath := strings.TrimPrefix(path, devDir)
		depth := strings.Count(relPath, string(filepath.Separator))

		// Skip if too deep
		if depth > maxDepth {
			return filepath.SkipDir
		}

		// Skip common non-project directories
		dirName := d.Name()
		if shouldSkipDir(dirName) {
			return filepath.SkipDir
		}

		// Check for project indicators concurrently
		wg.Add(1)
		go func(p string, rel string) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			if isProjectRoot(p) {
				// Clean up the relative path
				cleanRel := strings.TrimPrefix(rel, string(filepath.Separator))
				if cleanRel != "" {
					mu.Lock()
					projects = append(projects, cleanRel)
					mu.Unlock()
				}
			}
		}(path, relPath)

		return nil
	})
	if err != nil {
		// Log error but don't fail completely
		// You might want to add proper logging here
		_ = err
	}

	wg.Wait()
	return projects
}

// Helper function to check if a directory is a git repository (kept for backward compatibility)
func isGitRepo(path string) bool {
	gitPath := filepath.Join(path, ".git")
	stat, err := os.Stat(gitPath)
	return err == nil && (stat.IsDir() || stat.Mode().IsRegular()) // .git can be a file in submodules
}

// RemoveDuplicates removes duplicate strings from a slice with improved performance
func RemoveDuplicates(items []string) []string {
	if len(items) <= 1 {
		return items
	}

	// Pre-allocate with estimated capacity
	seen := make(map[string]struct{}, len(items))
	result := make([]string, 0, len(items))

	for _, item := range items {
		if _, exists := seen[item]; !exists {
			seen[item] = struct{}{}
			result = append(result, item)
		}
	}

	return result
}

// AddCustomSkipDir allows adding custom directories to skip
func AddCustomSkipDir(dirName string) {
	skipDirs[dirName] = true
}

// AddCustomSkipPattern allows adding custom patterns to skip
func AddCustomSkipPattern(pattern string) {
	skipPatterns = append(skipPatterns, pattern)
}
