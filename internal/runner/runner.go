package runner

import (
	"crypto/md5"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	lua "github.com/yuin/gopher-lua"
)

var (
	ErrInvalidConfig = errors.New("invalid configuration")
	ErrMissingField  = errors.New("missing required field")
)

type Config struct {
	SelectorCmd string
	Args        []string
	Show        func(string) string
	Process     func(string) string
	EditorCmd   func(string, string) (string, []string)
}

type LuaRunner struct {
	config     *Config
	state      *lua.LState
	mu         sync.RWMutex
	fnCache    map[string]lua.LValue
	configHash string
	configFile string
	lastMod    time.Time
}

// NewLuaRunner creates a new Lua runner with caching and config validation
func NewLuaRunner(configFile string) (*LuaRunner, error) {
	runner := &LuaRunner{
		configFile: configFile,
		fnCache:    make(map[string]lua.LValue),
	}

	if err := runner.loadConfig(); err != nil {
		return &LuaRunner{config: defaultConfig()}, nil
	}

	return runner, nil
}

// loadConfig loads and caches the Lua configuration
func (lr *LuaRunner) loadConfig() error {
	if lr.configFile == "" {
		lr.config = defaultConfig()
		return nil
	}

	stat, err := os.Stat(lr.configFile)
	if err != nil {
		return err
	}

	if !stat.ModTime().After(lr.lastMod) && lr.config != nil {
		return nil
	}

	state := lua.NewState()
	if err := state.DoFile(lr.configFile); err != nil {
		state.Close()
		return fmt.Errorf("lua file error: %w", err)
	}

	config, err := parseConfig(state)
	if err != nil {
		state.Close()
		return err
	}

	lr.mu.Lock()
	if lr.state != nil {
		lr.state.Close()
	}
	lr.state = state
	lr.config = config
	lr.lastMod = stat.ModTime()
	lr.configHash = lr.calculateConfigHash()
	lr.fnCache = make(map[string]lua.LValue)
	lr.mu.Unlock()

	return nil
}

// calculateConfigHash creates a hash of the config for change detection
func (lr *LuaRunner) calculateConfigHash() string {
	h := md5.New()
	h.Write([]byte(lr.configFile))
	h.Write([]byte(lr.lastMod.String()))
	return fmt.Sprintf("%x", h.Sum(nil))
}

// Close safely closes the Lua runner
func (lr *LuaRunner) Close() {
	lr.mu.Lock()
	defer lr.mu.Unlock()
	if lr.state != nil {
		lr.state.Close()
		lr.state = nil
	}
}

// Select runs the project selector with optimized string operations
func (lr *LuaRunner) Select(projects []string) (string, error) {
	if len(projects) == 0 {
		return "", errors.New("no projects provided")
	}

	lr.mu.RLock()
	if err := lr.loadConfig(); err != nil {
		lr.mu.RUnlock()
		return "", err
	}
	config := lr.config
	lr.mu.RUnlock()

	var builder strings.Builder
	for i, project := range projects {
		if i > 0 {
			builder.WriteByte('\n')
		}
		builder.WriteString(config.Show(project))
	}

	cmd := exec.Command(config.SelectorCmd, config.Args...)
	cmd.Stdin = strings.NewReader(builder.String())

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("command execution failed: %w", err)
	}

	result := config.Process(strings.TrimSpace(string(output)))
	if result == "" {
		return "", errors.New("no project selected")
	}

	return result, nil
}

// Start launches the editor with the given directory and title
func (lr *LuaRunner) Start(dir, title string) error {
	lr.mu.RLock()
	config := lr.config
	lr.mu.RUnlock()

	editorCmd, editorArgs := config.EditorCmd(dir, title)
	cmd := exec.Command(editorCmd, editorArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Start()
}

func parseConfig(L *lua.LState) (*Config, error) {
	returnValue := L.Get(-1)
	if returnValue.Type() != lua.LTTable {
		return nil, ErrInvalidConfig
	}

	table := returnValue.(*lua.LTable)
	config := &Config{}

	if err := parseSelectorCommand(L, table, config); err != nil {
		return nil, err
	}

	if err := parseEditorCommand(L, table, config); err != nil {
		return nil, err
	}

	if err := parseFunctions(L, table, config); err != nil {
		return nil, err
	}

	if err := validateConfig(config); err != nil {
		return nil, err
	}

	return config, nil
}

func parseSelectorCommand(L *lua.LState, table *lua.LTable, config *Config) error {
	cmdFn := L.GetField(table, "selector_cmd")

	if cmdFn.Type() != lua.LTFunction {
		return ErrMissingField
	}

	if err := L.CallByParam(lua.P{Fn: cmdFn, NRet: 1}); err != nil {
		return fmt.Errorf("command function error: %w", err)
	}

	cmdTable := L.Get(-1)
	if cmdTable.Type() != lua.LTTable {
		return ErrInvalidConfig
	}

	tbl := cmdTable.(*lua.LTable)
	config.SelectorCmd = lua.LVAsString(tbl.RawGet(lua.LString("command")))
	config.Args = parseStringArray(tbl.RawGet(lua.LString("args")))
	L.Pop(1)

	return nil
}

func parseFunctions(L *lua.LState, table *lua.LTable, config *Config) error {
	showFn := L.GetField(table, "format_project_title")
	processFn := L.GetField(table, "extract_path_from_title")

	if showFn.Type() != lua.LTFunction || processFn.Type() != lua.LTFunction {
		return ErrMissingField
	}

	config.Show = createLuaFunction(L, showFn)
	config.Process = createLuaFunction(L, processFn)

	return nil
}

// createLuaFunction creates a cached Lua function wrapper
func createLuaFunction(L *lua.LState, fn lua.LValue) func(string) string {
	return func(input string) string {
		L.Push(fn)
		L.Push(lua.LString(input))
		if err := L.PCall(1, 1, nil); err != nil {
			return input
		}
		result := lua.LVAsString(L.Get(-1))
		L.Pop(1)
		return result
	}
}

func validateConfig(config *Config) error {
	if config.SelectorCmd == "" || config.Show == nil || config.Process == nil {
		return ErrMissingField
	}
	return nil
}

func parseStringArray(v lua.LValue) []string {
	if v.Type() != lua.LTTable {
		return nil
	}

	table := v.(*lua.LTable)
	result := make([]string, 0, table.Len())

	table.ForEach(func(_, value lua.LValue) {
		result = append(result, lua.LVAsString(value))
	})
	return result
}

// defaultConfig returns the default configuration
func defaultConfig() *Config {
	return &Config{
		SelectorCmd: "fuzzel",
		Args:        []string{"--dmenu", "--prompt=Project: "},
		Show:        func(s string) string { return s },
		Process:     func(s string) string { return s },
		EditorCmd: func(dir, title string) (string, []string) {
			dirName := filepath.Base(dir)
			tmuxCmd := fmt.Sprintf("tmux new -c %s -A -s %s nvim %s", dir, dirName, dir)
			return "kitty", []string{"-d", dir, "-T", title, "--class", title, "sh", "-c", tmuxCmd}
		},
	}
}

func parseEditorCommand(L *lua.LState, table *lua.LTable, config *Config) error {
	cmdFn := L.GetField(table, "editor_cmd")
	if cmdFn.Type() != lua.LTFunction {
		return nil
	}

	config.EditorCmd = func(dir, title string) (string, []string) {
		L.Push(cmdFn)
		L.Push(lua.LString(dir))
		L.Push(lua.LString(title))
		if err := L.PCall(2, 1, nil); err != nil {
			return "", nil
		}

		cmdTable := L.Get(-1)
		if cmdTable.Type() != lua.LTTable {
			return "", nil
		}

		tbl := cmdTable.(*lua.LTable)
		cmd := lua.LVAsString(tbl.RawGet(lua.LString("command")))
		args := parseStringArray(tbl.RawGet(lua.LString("args")))
		L.Pop(1)
		return cmd, args
	}

	return nil
}
