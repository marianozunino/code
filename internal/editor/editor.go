package editor

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/marianozunino/code/internal/must"
	"github.com/marianozunino/code/internal/window"
)

type Window struct {
	ID   int    `json:"id"`
	Type string `json:"type"`
	Name string `json:"name"`
}

func LaunchNeovim(dir string) error {
	dirName := filepath.Base(dir)
	title := fmt.Sprintf("nvim ~ %s", dirName)
	tmuxCmd := fmt.Sprintf("tmux new -c %s -A -s %s nvim %s", dir, dirName, dir)

	windowID := must.Must(window.FindWindow(title))
	if windowID == 0 {
		cmd := exec.Command("kitty", "-d", dir, "-T", title, "--class", title, "sh", "-c", tmuxCmd)
		if err := cmd.Start(); err != nil {
			return err
		}

		windowID = waitForWindow(title)
	}

	if windowID != 0 {
		window.FocusWindow(windowID)
	}
	return nil
}

func waitForWindow(title string) int64 {
	for backoff := 0.1; backoff < 2.0; backoff *= 2 {
		time.Sleep(time.Duration(backoff * float64(time.Second)))
		if windowID, _ := window.FindWindow(title); windowID != 0 {
			return windowID
		}
	}
	return 0
}
