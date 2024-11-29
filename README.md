# Code - Project Launcher for Development Directories

A CLI tool tailored for my personal development workflow. It simplifies navigation and launching of projects in Neovim while maintaining a most-recently-used (MRU) list. The tool is optimized for my specific setup, integrating with my preferred terminal emulator and window manager.

---

## Features

- 🔍 **Quick Project Navigation**: Fuzzy finding for rapid access.
- 📋 **MRU Tracking**: Keeps a history of recently accessed projects.
- 🛠️ **Neovim Integration**: Open projects directly in Neovim.
- 🖥️ **Window Manager Support**: Optimized for the Sway window manager.
- 🎨 **Customizable Selector Interface**: Tailored for flexibility in selection methods.
- 💻 **tmux Support**: Manage tmux sessions for projects.

---

## Prerequisites

This tool is built around my workflow and assumes the following are installed:

- **Go** 1.23+
- **Sway** (window manager)
- **kitty** (terminal emulator)
- **Neovim**
- **tmux**
- **fuzzel** or **fzf** (configurable selector)
- **git** (for project detection)

---

## Installation

Install the tool using `go install`:

```bash
go install github.com/marianozunino/code@latest
```

---

### Selector Configuration

The selector interface is customizable with Lua scripts. Below are configurations for **fuzzel** (default) and **fzf**, which I use based on specific contexts.

#### Example: **fuzzel**
```lua
return {
    command = function()
        return {
            command = "fuzzel",
            args = { "--dmenu", "--prompt=Project: " }
        }
    end,
    show = function(text)
        return "📘 " .. text
    end,
    process_output = function(text)
        return text:gsub("^📘%s*", "")
    end
}
```

#### Example: **fzf**
```lua
return {
    command = function()
        return {
            command = "fzf",
            args = { "--prompt=Project > ", "--height=40%", "--layout=reverse" }
        }
    end,
    show = function(text)
        return "📘 " .. text
    end,
    process_output = function(text)
        return text:gsub("^📘%s*", ""):gsub("\n$", "")
    end
}
```

---

## Usage

This tool is designed for simplicity and ease of use in my workflow:

```bash
# Launch the project selector
code

# Specify a custom base directory
code ~/Projects

# Use a custom selector configuration
code -s ~/my-selector.lua
```

---

## Window Management

The tool integrates with the **Sway** window manager to align with my workflow:

- Opens projects in new windows with specific titles.
- Focuses on existing project windows if they are already open.
- Simplifies window layout adjustments.

---

## Project Detection

Projects are identified by the presence of a `.git` directory. The tool scans the base directory recursively to find Git repositories.

---

## License

This project is licensed under the MIT License. See [LICENSE](LICENSE) for details.
