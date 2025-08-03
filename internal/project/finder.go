package project

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Cache configuration
const (
	cacheFileName    = ".code_projects_cache"
	cacheMaxAge      = 5 * time.Minute
	maxCacheAttempts = 3
)

// ProjectCache represents the cached project data
type ProjectCache struct {
	Projects     []string  `json:"projects"`
	LastScan     time.Time `json:"last_scan"`
	BaseDirMod   time.Time `json:"base_dir_mod"`
	ProjectCount int       `json:"project_count"`
	ScanDuration string    `json:"scan_duration"`
	CacheVersion int       `json:"cache_version"`
}

// Current cache version for invalidation when logic changes
const cacheVersion = 1

// Directories to skip during project search (prioritized by frequency)
var skipDirs = map[string]bool{
	// Most common first for faster lookup
	"node_modules": true,
	".git":         true,
	"vendor":       true,
	"target":       true,
	"build":        true,
	"dist":         true,

	// Dependencies and package managers
	".npm":             true,
	".yarn":            true,
	".pnpm":            true,
	"bower_components": true,

	// Build outputs and caches
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

// Project indicators ordered by frequency/likelihood
var projectIndicators = []string{
	".git",         // Most common
	"package.json", // Node.js projects
	"go.mod",       // Go projects
	"Cargo.toml",   // Rust projects
	"pom.xml",      // Maven projects
	"build.gradle", // Gradle projects
	"Makefile",     // Make-based projects
	"CMakeLists.txt",
	"requirements.txt", // Python
	"setup.py",
	"pyproject.toml",
	"composer.json", // PHP
	"Gemfile",       // Ruby
	"mix.exs",       // Elixir
	"elm.json",      // Elm
	"deno.json",     // Deno
	"pubspec.yaml",  // Dart/Flutter
}

// Performance tracking structure
type findStats struct {
	dirsScanned   int64
	dirsSkipped   int64
	projectsFound int64
	cacheHit      bool
	startTime     time.Time
}

// getCachePath returns the path to the cache file
func getCachePath(baseDir string) string {
	return filepath.Join(baseDir, cacheFileName)
}

// getBaseDirModTime gets the modification time of the base directory
func getBaseDirModTime(baseDir string) (time.Time, error) {
	stat, err := os.Stat(baseDir)
	if err != nil {
		return time.Time{}, err
	}
	return stat.ModTime(), nil
}

// loadCache attempts to load and validate the project cache
func loadCache(baseDir string) (*ProjectCache, bool) {
	cachePath := getCachePath(baseDir)

	// Check if cache file exists and is recent
	cacheInfo, err := os.Stat(cachePath)
	if err != nil {
		return nil, false
	}

	// Check cache age
	if time.Since(cacheInfo.ModTime()) > cacheMaxAge {
		return nil, false
	}

	// Load cache content
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, false
	}

	var cache ProjectCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, false
	}

	// Validate cache version
	if cache.CacheVersion != cacheVersion {
		return nil, false
	}

	// Check if base directory has been modified
	baseDirMod, err := getBaseDirModTime(baseDir)
	if err != nil {
		return nil, false
	}

	if baseDirMod.After(cache.BaseDirMod) {
		return nil, false
	}

	return &cache, true
}

// saveCache saves the project cache to disk
func saveCache(baseDir string, projects []string, scanStart time.Time) {
	baseDirMod, err := getBaseDirModTime(baseDir)
	if err != nil {
		return
	}

	cache := ProjectCache{
		Projects:     projects,
		LastScan:     scanStart,
		BaseDirMod:   baseDirMod,
		ProjectCount: len(projects),
		ScanDuration: time.Since(scanStart).String(),
		CacheVersion: cacheVersion,
	}

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return
	}

	cachePath := getCachePath(baseDir)
	if err := os.WriteFile(cachePath, data, 0o644); err != nil {
		return
	}
}

// shouldSkipDir determines if a directory should be skipped
func shouldSkipDir(dirName string) bool {
	// Check exact matches (case-sensitive for performance)
	if skipDirs[dirName] {
		return true
	}

	// Skip hidden directories that start with . (except .git which we handle separately)
	if strings.HasPrefix(dirName, ".") && dirName != ".git" {
		return true
	}

	// Quick pattern checks for common cases
	lower := strings.ToLower(dirName)
	if strings.Contains(lower, "cmake-build-") ||
		strings.Contains(lower, ".terraform") {
		return true
	}

	return false
}

// isProjectRoot checks if a directory is a project root with optimized indicator checking
func isProjectRoot(path string, stats *findStats) bool {
	checkStart := time.Now()

	// Check indicators in order of likelihood, exit early on first match
	for _, indicator := range projectIndicators {
		if _, err := os.Stat(filepath.Join(path, indicator)); err == nil {
			atomic.AddInt64(&stats.projectsFound, 1)
			return true
		}

		// Early timeout for very slow filesystem
		if time.Since(checkStart) > 50*time.Millisecond {
			break
		}
	}

	return false
}

// FindProjects finds all projects in the given directory with caching
func FindProjects(devDir string) []string {
	start := time.Now()

	stats := &findStats{
		startTime: start,
	}

	// Try to load from cache first
	if cache, valid := loadCache(devDir); valid {
		stats.cacheHit = true
		return cache.Projects
	}

	// Cache miss - perform filesystem scan
	projects := findProjectsFilesystem(devDir, stats)

	// Save to cache asynchronously
	go func() {
		saveCache(devDir, projects, start)
	}()

	return projects
}

// findProjectsFilesystem performs the actual filesystem scanning
func findProjectsFilesystem(devDir string, stats *findStats) []string {
	maxDepth := 3
	var projects []string
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Use smaller worker pool to reduce contention
	workerCount := runtime.NumCPU()
	semaphore := make(chan struct{}, workerCount)

	filepath.WalkDir(devDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Continue walking, ignore errors
		}

		if !d.IsDir() {
			return nil
		}

		atomic.AddInt64(&stats.dirsScanned, 1)

		// Calculate depth
		relPath := strings.TrimPrefix(path, devDir)
		depth := strings.Count(relPath, string(filepath.Separator))

		// Skip if too deep
		if depth > maxDepth {
			atomic.AddInt64(&stats.dirsSkipped, 1)
			return filepath.SkipDir
		}

		// Skip common non-project directories
		dirName := d.Name()
		if shouldSkipDir(dirName) {
			atomic.AddInt64(&stats.dirsSkipped, 1)
			return filepath.SkipDir
		}

		// Check for project indicators concurrently
		wg.Add(1)
		go func(p string, rel string) {
			defer wg.Done()

			// Acquire semaphore with timeout to prevent blocking
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-time.After(10 * time.Millisecond):
			}

			if isProjectRoot(p, stats) {
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

	wg.Wait()

	return projects
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

// ClearCache removes the project cache file
func ClearCache(baseDir string) error {
	cachePath := getCachePath(baseDir)
	err := os.Remove(cachePath)
	if os.IsNotExist(err) {
		return nil // Cache doesn't exist, nothing to clear
	}
	return err
}

// GetCacheInfo returns information about the current cache
func GetCacheInfo(baseDir string) (bool, time.Time, int) {
	if cache, valid := loadCache(baseDir); valid {
		return true, cache.LastScan, len(cache.Projects)
	}
	return false, time.Time{}, 0
}

// AddCustomSkipDir allows adding custom directories to skip
func AddCustomSkipDir(dirName string) {
	skipDirs[dirName] = true
}
