# Code - Project Launcher

A fast CLI tool for navigating and launching development projects with fuzzy finding and MRU tracking.

## Features

- 🔍 Fast project discovery
- 📋 MRU tracking
- 🛠️ Neovim + tmux integration
- 🖥️ Sway window management
- 🎨 Multiple selectors (rofi, fuzzel, fzf)

## Installation

```bash
go build -o code .
```

## Usage

```bash
# Basic usage
./code ~/Dev

# With specific selector
./code ~/Dev -s rofi.yaml
```

## Configuration

The tool uses simple YAML files. Three configurations are included:

- `rofi.yaml` - Rofi selector with enhanced tmux sessions
- `fuzzel.yaml` - Fuzzel selector (default)
- `fzf.yaml` - FZF selector

### Example Configuration

```yaml
selector:
  command: rofi
  args: ["-dmenu", "-i", "-p", "Project: "]

editor:
  command: kitty
  args: "-d {{.Dir}} -T {{.Title}} --class {{.Title}} sh -c \"tmux new -c {{.Dir}} -A -s {{.Name}} nvim {{.Dir}}\""

format:
  project_title: "📘 {{.Path}}"
  extract_path: "{{.Title | trimPrefix \"📘 \"}}"
```

## Template Variables

- `{{.Dir}}` - Full project path
- `{{.Title}}` - Window title
- `{{.Name}}` - Project name
- `{{.SanitizedName}}` - Sanitized for tmux
- `{{.Path}}` - Relative path

## Requirements

- Go 1.23+
- Sway window manager
- kitty terminal
- Neovim
- tmux
- A selector tool (rofi/fuzzel/fzf)

## License

MIT License