package core

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"text/template"
	"time"

	"gopkg.in/yaml.v3"
)

// Project represents a development project
type Project struct {
	Path string
	Name string
}

// ProjectFinder finds Git repositories in a directory
type ProjectFinder struct{}

// FindProjects scans a directory for Git repositories
func (pf *ProjectFinder) FindProjects(devDir string) []string {
	var projects []string

	// Simple directory walk without external dependencies
	err := filepath.Walk(devDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		if !info.IsDir() {
			return nil
		}
		if isGitRepo(path) {
			relPath := strings.TrimPrefix(path, devDir+"/")
			projects = append(projects, relPath)
			return filepath.SkipDir // Don't scan inside git repos
		}
		return nil
	})

	if err != nil {
		return []string{}
	}

	return projects
}

// isGitRepo checks if a directory is a Git repository
func isGitRepo(path string) bool {
	_, err := os.Stat(filepath.Join(path, ".git"))
	return err == nil
}

// RemoveDuplicates removes duplicate strings from a slice
func RemoveDuplicates(items []string) []string {
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

// MRUList manages the most recently used projects
type MRUList struct {
	filename string
	baseDir  string
	items    []string
	dirty    bool
	mu       sync.RWMutex
	timer    *time.Timer
}

// NewMRUList creates a new MRU list
func NewMRUList(filename, baseDir string) *MRUList {
	mru := &MRUList{
		filename: filename,
		baseDir:  baseDir,
		items:    loadMRU(filename),
		dirty:    false,
	}

	// Set up periodic save timer
	mru.timer = time.AfterFunc(5*time.Second, func() {
		mru.saveIfDirty()
	})

	return mru
}

// loadMRU loads MRU items from file
func loadMRU(filename string) []string {
	data, err := os.ReadFile(filename)
	if err != nil {
		return []string{}
	}
	return strings.Fields(string(data))
}

// Update adds a project to the MRU list
func (m *MRUList) Update(project string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	fullPath := filepath.Join(m.baseDir, project)
	absPath, _ := filepath.Abs(fullPath)
	m.items = updateMRUList(absPath, m.items)
	m.dirty = true

	// Reset timer for immediate save
	if m.timer != nil {
		m.timer.Stop()
	}
	m.timer = time.AfterFunc(1*time.Second, func() {
		m.saveIfDirty()
	})

	return nil
}

// Items returns the MRU items as relative paths
func (m *MRUList) Items() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var relative []string
	for _, item := range m.items {
		rel, err := filepath.Rel(m.baseDir, item)
		if err == nil {
			relative = append(relative, rel)
		}
	}
	return relative
}

// Close ensures the MRU list is saved
func (m *MRUList) Close() error {
	if m.timer != nil {
		m.timer.Stop()
	}
	return m.save()
}

// save saves the MRU list to file
func (m *MRUList) save() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	content := strings.Join(m.items, "\n")
	err := os.WriteFile(m.filename, []byte(content), 0o644)
	if err == nil {
		m.dirty = false
	}
	return err
}

// saveIfDirty saves only if the list has been modified
func (m *MRUList) saveIfDirty() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.dirty {
		content := strings.Join(m.items, "\n")
		if err := os.WriteFile(m.filename, []byte(content), 0o644); err == nil {
			m.dirty = false
		}
	}

	// Reset timer for next save
	m.timer = time.AfterFunc(5*time.Second, func() {
		m.saveIfDirty()
	})
}

// updateMRUList updates the MRU list with a new project
func updateMRUList(project string, list []string) []string {
	newList := []string{project}
	for _, p := range list {
		if p != project && len(newList) < 10 {
			newList = append(newList, p)
		}
	}
	return newList
}

// WindowManager handles Sway window operations
type WindowManager struct{}

// FindWindow finds a window by title
func (wm *WindowManager) FindWindow(title string) (int64, error) {
	cmd := exec.Command("swaymsg", "-t", "get_tree")
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("failed to get sway tree: %w", err)
	}

	var tree SwayTree
	if err := json.Unmarshal(output, &tree); err != nil {
		return 0, fmt.Errorf("failed to parse sway tree: %w", err)
	}

	// Search for window with matching title
	for _, node := range tree.Nodes {
		if windowID := findNodeByTitle(node, title); windowID != 0 {
			return windowID, nil
		}
	}

	return 0, nil
}

// FocusWindow focuses a window by ID
func (wm *WindowManager) FocusWindow(windowID int64) error {
	cmd := exec.Command("swaymsg", fmt.Sprintf(`[con_id="%d"] focus`, windowID))
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to focus window: %w", err)
	}

	// Check if the command succeeded
	if strings.Contains(string(output), "success") {
		return nil
	}

	return fmt.Errorf("swaymsg focus command failed: %s", string(output))
}

// SwayNode represents a node in the Sway tree
type SwayNode struct {
	ID            int64      `json:"id"`
	Name          string     `json:"name"`
	AppID         *string    `json:"app_id"`
	Nodes         []SwayNode `json:"nodes"`
	FloatingNodes []SwayNode `json:"floating_nodes"`
}

// SwayTree represents the root of the Sway tree
type SwayTree struct {
	Nodes []SwayNode `json:"nodes"`
}

// findNodeByTitle recursively searches for a node with the given title
func findNodeByTitle(node SwayNode, title string) int64 {
	// Check if this node matches
	if node.AppID != nil && node.Name == title {
		return node.ID
	}

	// Search in regular nodes
	for _, n := range node.Nodes {
		if windowID := findNodeByTitle(n, title); windowID != 0 {
			return windowID
		}
	}

	// Search in floating nodes
	for _, n := range node.FloatingNodes {
		if windowID := findNodeByTitle(n, title); windowID != 0 {
			return windowID
		}
	}

	return 0
}

// Config represents the application configuration
type Config struct {
	Selector SelectorConfig `yaml:"selector"`
	Editor   EditorConfig   `yaml:"editor"`
	Format   FormatConfig   `yaml:"format"`
}

// SelectorConfig defines the project selector settings
type SelectorConfig struct {
	Command string   `yaml:"command"`
	Args    []string `yaml:"args"`
}

// EditorConfig defines the editor launch settings
type EditorConfig struct {
	Command string `yaml:"command"`
	Args    string `yaml:"args"` // Template string
}

// FormatConfig defines the formatting settings
type FormatConfig struct {
	ProjectTitle string `yaml:"project_title"` // Template string
	ExtractPath  string `yaml:"extract_path"`  // Template string
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		Selector: SelectorConfig{
			Command: "fuzzel",
			Args:    []string{"--dmenu", "--prompt=Project: "},
		},
		Editor: EditorConfig{
			Command: "kitty",
			Args:    "-d {{.Dir}} -T {{.Title}} --class {{.Title}} sh -c \"tmux new -c {{.Dir}} -A -s {{.Name}} nvim {{.Dir}}\"",
		},
		Format: FormatConfig{
			ProjectTitle: "📘 {{.Path}}",
			ExtractPath:  "{{.Title | trimPrefix \"📘 \"}}",
		},
	}
}

// LoadConfig loads configuration from a YAML file or returns default config
func LoadConfig(configFile string) (*Config, error) {
	if configFile == "" {
		return DefaultConfig(), nil
	}

	data, err := os.ReadFile(configFile)
	if err != nil {
		return DefaultConfig(), nil // Return default if file doesn't exist
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &config, nil
}

// Selector provides methods for project selection
type Selector struct {
	config *Config
}

// NewSelector creates a new selector instance
func NewSelector(config *Config) *Selector {
	return &Selector{config: config}
}

// Select runs the selector command and returns the selected project
func (s *Selector) Select(projects []string) (string, error) {
	if len(projects) == 0 {
		return "", fmt.Errorf("no projects provided")
	}

	// Format projects using template
	formatted := make([]string, len(projects))
	for i, project := range projects {
		formatted[i] = s.formatProjectTitle(project)
	}

	// Run selector command
	cmd := exec.Command(s.config.Selector.Command, s.config.Selector.Args...)
	cmd.Stdin = strings.NewReader(strings.Join(formatted, "\n"))

	output, err := cmd.Output()
	if err != nil {
		// Check if it's a cancellation (exit code 1 for rofi/fuzzel)
		if exitError, ok := err.(*exec.ExitError); ok && exitError.ExitCode() == 1 {
			return "", nil // User cancelled, return empty string
		}
		return "", fmt.Errorf("command execution failed: %w", err)
	}

	result := strings.TrimSpace(string(output))
	if result == "" {
		return "", fmt.Errorf("no project selected")
	}

	// Extract path from formatted result
	return s.extractPath(result), nil
}

// Start launches the editor for the given project
func (s *Selector) Start(dir, title string) error {
	editorCmd, editorArgs := s.buildEditorCommand(dir, title)

	cmd := exec.Command(editorCmd, editorArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Start()
}

// formatProjectTitle formats a project path using the template
func (s *Selector) formatProjectTitle(path string) string {
	tmpl, err := template.New("project").Parse(s.config.Format.ProjectTitle)
	if err != nil {
		return path // Fallback to original path
	}

	var buf strings.Builder
	data := map[string]string{
		"Path": path,
	}

	if err := tmpl.Execute(&buf, data); err != nil {
		return path // Fallback to original path
	}

	return buf.String()
}

// extractPath extracts the project path from formatted title
func (s *Selector) extractPath(title string) string {
	tmpl, err := template.New("extract").Funcs(template.FuncMap{
		"trimPrefix":   strings.TrimPrefix,
		"trimSuffix":   strings.TrimSuffix,
		"trimSpace":    strings.TrimSpace,
		"removePrefix": removePrefix,
	}).Parse(s.config.Format.ExtractPath)
	if err != nil {
		return title // Fallback to original title
	}

	var buf strings.Builder
	data := map[string]string{
		"Title": title,
	}

	if err := tmpl.Execute(&buf, data); err != nil {
		return title // Fallback to original title
	}

	return strings.TrimSpace(buf.String())
}

// removePrefix is a custom template function that handles Unicode prefixes correctly
func removePrefix(prefix, s string) string {
	// Template functions receive arguments in reverse order when using pipe operator
	if strings.HasPrefix(s, prefix) {
		return s[len(prefix):]
	}
	return s
}

// buildEditorCommand builds the editor command and arguments
func (s *Selector) buildEditorCommand(dir, title string) (string, []string) {
	tmpl, err := template.New("editor").Funcs(template.FuncMap{
		"sanitize": sanitizeForTmux,
	}).Parse(s.config.Editor.Args)
	if err != nil {
		// Fallback to simple command
		return s.config.Editor.Command, []string{"-d", dir, "-T", title, "--class", title}
	}

	var buf strings.Builder
	data := map[string]string{
		"Dir":           dir,
		"Title":         title,
		"Name":          filepath.Base(dir),
		"SanitizedName": sanitizeForTmux(filepath.Base(dir)),
	}

	if err := tmpl.Execute(&buf, data); err != nil {
		// Fallback to simple command
		return s.config.Editor.Command, []string{"-d", dir, "-T", title, "--class", title}
	}

	// Handle shell commands properly - don't split quoted arguments
	args := parseShellArgs(buf.String())
	return s.config.Editor.Command, args
}

// parseShellArgs parses shell arguments while preserving quoted strings
func parseShellArgs(s string) []string {
	var args []string
	var current strings.Builder
	var inQuotes bool
	var quoteChar rune

	for _, r := range s {
		switch {
		case !inQuotes && (r == '"' || r == '\''):
			inQuotes = true
			quoteChar = r
		case inQuotes && r == quoteChar:
			inQuotes = false
			quoteChar = 0
		case !inQuotes && (r == ' ' || r == '\t'):
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}

	if current.Len() > 0 {
		args = append(args, current.String())
	}

	return args
}

// sanitizeForTmux sanitizes a string for use as a tmux session name
func sanitizeForTmux(name string) string {
	// Replace any non-alphanumeric characters with underscores
	result := ""
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			result += string(r)
		} else {
			result += "_"
		}
	}
	return result
}
