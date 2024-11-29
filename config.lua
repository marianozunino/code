return {
	command = function()
		return {
			command = "fzf",
			args = { "--prompt=Project > ", "--height=40%", "--layout=reverse" },
		}
	end,
	show = function(text)
		return "📘 " .. text
	end,
	process_output = function(text)
		return text:gsub("^📘%s*", ""):gsub("\n$", "")
	end,
}
