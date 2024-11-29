package ui

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"

	lua "github.com/yuin/gopher-lua"
)

var (
	ErrInvalidConfig = errors.New("invalid configuration")
	ErrMissingField  = errors.New("missing required field")
)

type Config struct {
	Command string
	Args    []string
	Show    func(string) string
	Process func(string) string
}

type ProjectSelector struct {
	config *Config
	state  *lua.LState
	mu     sync.Mutex
}

func NewProjectSelector(configFile string) (*ProjectSelector, error) {
	state := lua.NewState()
	if configFile != "" {
		if err := state.DoFile(configFile); err != nil {
			state.Close()
			return nil, fmt.Errorf("lua file error: %w", err)
		}
	}

	config, err := parseConfig(state)
	if err != nil {
		state.Close()
		return &ProjectSelector{config: defaultConfig()}, nil
	}

	return &ProjectSelector{
		config: config,
		state:  state,
	}, nil
}

func (ps *ProjectSelector) Close() {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	if ps.state != nil {
		ps.state.Close()
		ps.state = nil
	}
}

func (ps *ProjectSelector) Select(projects []string) (string, error) {
	if len(projects) == 0 {
		return "", errors.New("no projects provided")
	}

	formatted := make([]string, len(projects))
	for i, project := range projects {
		formatted[i] = ps.config.Show(project)
	}

	cmd := exec.Command(ps.config.Command, ps.config.Args...)
	cmd.Stdin = strings.NewReader(strings.Join(formatted, "\n"))

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("command execution failed: %w", err)
	}

	result := ps.config.Process(strings.TrimSpace(string(output)))
	if result == "" {
		return "", errors.New("no project selected")
	}

	return result, nil
}

func parseConfig(L *lua.LState) (*Config, error) {
	returnValue := L.Get(-1)
	if returnValue.Type() != lua.LTTable {
		return nil, ErrInvalidConfig
	}

	table := returnValue.(*lua.LTable)
	config := &Config{}

	if err := parseCommand(L, table, config); err != nil {
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

func parseCommand(L *lua.LState, table *lua.LTable, config *Config) error {
	cmdFn := L.GetField(table, "command")
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
	config.Command = lua.LVAsString(tbl.RawGet(lua.LString("command")))
	config.Args = parseStringArray(tbl.RawGet(lua.LString("args")))
	L.Pop(1)

	return nil
}

func parseFunctions(L *lua.LState, table *lua.LTable, config *Config) error {
	showFn := L.GetField(table, "show")
	processFn := L.GetField(table, "process_output")

	if showFn.Type() != lua.LTFunction || processFn.Type() != lua.LTFunction {
		return ErrMissingField
	}

	config.Show = createLuaFunction(L, showFn)
	config.Process = createLuaFunction(L, processFn)

	return nil
}

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
	if config.Command == "" || config.Show == nil || config.Process == nil {
		return ErrMissingField
	}
	return nil
}

func parseStringArray(v lua.LValue) []string {
	if v.Type() != lua.LTTable {
		return nil
	}

	var result []string
	v.(*lua.LTable).ForEach(func(_, value lua.LValue) {
		result = append(result, lua.LVAsString(value))
	})
	return result
}

func defaultConfig() *Config {
	return &Config{
		Command: "fuzzel",
		Args:    []string{"--dmenu", "--prompt=Project: "},
		Show:    func(s string) string { return s },
		Process: func(s string) string { return s },
	}
}

