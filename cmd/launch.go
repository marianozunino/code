package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"mzunino.com.uy/go/code/internal/mru"
	"mzunino.com.uy/go/code/internal/project"
	"mzunino.com.uy/go/code/internal/runner"
	"mzunino.com.uy/go/code/internal/window"
)

const (
	maxWaitTime    = 2 * time.Second
	initialBackoff = 100 * time.Millisecond
	backoffFactor  = 2
)

// launchProject handles the project selection and launching process with async optimizations.
// It returns an error if any operation fails.
func launchProject(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), maxWaitTime)
	defer cancel()

	// Initialize MRU list
	mruList := mru.NewMRUList(cfg.MruFile, cfg.BaseDir)

	type projectResult struct {
		projects []string
		err      error
	}

	projectsCh := make(chan projectResult, 1)
	mruCh := make(chan []string, 1)

	// Project discovery goroutine
	go func() {
		projects := project.FindProjects(cfg.BaseDir)
		projectsCh <- projectResult{projects: projects, err: nil}
	}()

	// MRU loading goroutine
	go func() {
		items := mruList.Items()
		mruCh <- items
	}()

	var allProjects []string
	var mruProjects []string

	projectsReceived := false
	mruReceived := false

	// Wait for both operations to complete
	for !projectsReceived || !mruReceived {
		select {
		case result := <-projectsCh:
			if result.err != nil {
				return fmt.Errorf("failed to find projects: %w", result.err)
			}
			allProjects = result.projects
			projectsReceived = true

		case items := <-mruCh:
			mruProjects = items
			mruReceived = true

		case <-ctx.Done():
			return fmt.Errorf("timeout during project discovery")
		}
	}

	// Merge and deduplicate projects
	uniqueProjects := project.RemoveDuplicates(append(mruProjects, allProjects...))

	if len(uniqueProjects) == 0 {
		return fmt.Errorf("no projects found in %s", cfg.BaseDir)
	}

	// Initialize runner
	run, err := runner.NewLuaRunner(cfg.SelectorFile)
	if err != nil {
		return fmt.Errorf("failed to create runner: %w", err)
	}

	// Project selection
	selectionStart := time.Now()
	selectedProject, err := run.Select(uniqueProjects)
	if err != nil {
		return fmt.Errorf("project selection failed: %w", err)
	}
	if selectedProject == "" {
		return nil
	}
	log.Printf("[PERF] Project selection: duration=%v selected=%s", time.Since(selectionStart), selectedProject)

	// Validate selected project
	fullPath := filepath.Join(cfg.BaseDir, selectedProject)
	if !isDirectory(fullPath) {
		return fmt.Errorf("not a directory: %s", fullPath)
	}

	windowTitle := fmt.Sprintf("nvim ~ %s", filepath.Base(fullPath))

	// Launch window and update MRU concurrently
	var wg sync.WaitGroup
	var windowErr error
	var mruErr error

	wg.Add(1)
	go func() {
		defer wg.Done()
		start := time.Now()
		windowErr = launchOrFocusWindow(ctx, run, fullPath, windowTitle)
		duration := time.Since(start)
		if windowErr != nil {
			log.Printf("[PERF] Window launch failed: duration=%v error=%v", duration, windowErr)
		} else {
			log.Printf("[PERF] Window launch completed: duration=%v", duration)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		start := time.Now()
		mruErr = mruList.Update(selectedProject)
		duration := time.Since(start)
		if mruErr != nil {
			log.Printf("[PERF] MRU update failed: duration=%v error=%v", duration, mruErr)
		} else {
			log.Printf("[PERF] MRU update completed: duration=%v", duration)
		}
	}()

	wg.Wait()

	if windowErr != nil {
		return fmt.Errorf("failed to launch/focus window: %w", windowErr)
	}

	if mruErr != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to update MRU: %v\n", mruErr)
	}

	return nil
}

// launchOrFocusWindow either focuses an existing window or launches a new one with async optimization.
// It returns an error if the window cannot be launched or found.
func launchOrFocusWindow(ctx context.Context, run *runner.LuaRunner, projectPath, windowTitle string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	type windowResult struct {
		windowID int64
		err      error
	}

	windowCh := make(chan windowResult, 1)

	// Check for existing window
	go func() {
		windowID, err := window.FindWindow(windowTitle)
		windowCh <- windowResult{windowID: windowID, err: err}
	}()

	var result windowResult
	select {
	case result = <-windowCh:
	case <-ctx.Done():
		result = windowResult{windowID: 0, err: nil}
	}

	if result.err != nil {
		result.windowID = 0
	}

	if result.windowID == 0 {
		if err := run.Start(projectPath, windowTitle); err != nil {
			return err
		}

		go func() {
			waitForWindow(ctx, windowTitle)
		}()
	} else {
		// Existing window found, focus it
		if err := window.FocusWindow(result.windowID); err != nil {
			return run.Start(projectPath, windowTitle)
		}
	}

	return nil
}

// isDirectory checks if the given path is a directory.
func isDirectory(path string) bool {
	info, err := os.Stat(path)
	isDir := err == nil && info.IsDir()
	return isDir
}

// waitForWindow waits for a window with the given title to appear.
// It returns the window ID or an error if the window is not found within the timeout.
func waitForWindow(ctx context.Context, title string) (int64, error) {
	backoff := initialBackoff
	attempts := 0

	for {
		select {
		case <-ctx.Done():
			return 0, fmt.Errorf("timeout waiting for window: %s", title)
		default:
			attempts++
			if windowID, _ := window.FindWindow(title); windowID != 0 {
				return windowID, nil
			}

			time.Sleep(backoff)
			backoff *= backoffFactor

			if backoff > time.Second {
				backoff = time.Second
			}
		}
	}
}
