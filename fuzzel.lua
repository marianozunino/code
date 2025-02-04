return {
	selector_cmd = function()
		return {
			command = "fuzzel",
			args = { "--dmenu", "--prompt=Project: " },
		}
	end,
	-- Class should not be changed, used to target the window
	editor_cmd = function(path, class)
		local dirName = path:match("([^/]+)$")
		local tmuxCmd = "tmux new -c " .. path .. " -A -s " .. dirName .. " nvim " .. path
		return {
			command = "kitty",
			args = { "-d", path, "-T", class, "--class", class, "sh", "-c", tmuxCmd },
		}
	end,
	format_project_title = function(path)
		-- Example: work/project_1
		return "📘 " .. path
	end,
	get_path_from_title = function(path)
		-- Return back the path
		return path:gsub("^📘%s*", ""):gsub("\n$", "")
	end,
}
