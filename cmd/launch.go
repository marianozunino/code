package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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

// launchProject handles the project selection and launching process.
// It returns an error if any operation fails.
func launchProject(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), maxWaitTime)
	defer cancel()

	mruList := mru.NewMRUList(cfg.MruFile, cfg.BaseDir)
	allProjects := project.FindProjects(cfg.BaseDir)

	uniqueProjects := project.RemoveDuplicates(append(mruList.Items(), allProjects...))
	if len(uniqueProjects) == 0 {
		return fmt.Errorf("no projects found in %s", cfg.BaseDir)
	}

	run, err := runner.NewLuaRunner(cfg.SelectorFile)
	if err != nil {
		return fmt.Errorf("failed to create runner: %w", err)
	}

	selectedProject, err := run.Select(uniqueProjects)
	if err != nil {
		return fmt.Errorf("project selection failed: %w", err)
	}
	if selectedProject == "" {
		return nil
	}

	fullPath := filepath.Join(cfg.BaseDir, selectedProject)
	if !isDirectory(fullPath) {
		return fmt.Errorf("not a directory: %s", fullPath)
	}

	windowTitle := fmt.Sprintf("nvim ~ %s", filepath.Base(fullPath))

	if err := launchOrFocusWindow(ctx, run, fullPath, windowTitle); err != nil {
		return fmt.Errorf("failed to launch/focus window: %w", err)
	}

	return mruList.Update(selectedProject)
}

// launchOrFocusWindow either focuses an existing window or launches a new one.
// It returns an error if the window cannot be launched or found.
func launchOrFocusWindow(ctx context.Context, run *runner.LuaRunner, projectPath, windowTitle string) error {
	windowID, _ := window.FindWindow(windowTitle)

	if windowID == 0 {
		if err := run.Start(projectPath, windowTitle); err != nil {
			return err
		}
		windowID, _ = waitForWindow(ctx, windowTitle)
	} else {
		if err := window.FocusWindow(windowID); err != nil {
			return err
		}
	}

	return nil
}

// isDirectory checks if the given path is a directory.
func isDirectory(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// waitForWindow waits for a window with the given title to appear.
// It returns the window ID or an error if the window is not found within the timeout.
func waitForWindow(ctx context.Context, title string) (int64, error) {
	backoff := initialBackoff

	for {
		select {
		case <-ctx.Done():
			return 0, fmt.Errorf("timeout waiting for window: %s", title)
		default:
			if windowID, _ := window.FindWindow(title); windowID != 0 {
				return windowID, nil
			}
			time.Sleep(backoff)
			backoff *= backoffFactor
		}
	}
}
