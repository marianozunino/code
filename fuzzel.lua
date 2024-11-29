return {
	command = function()
		return {
			command = "fuzzel",
			args = { "--dmenu", "--prompt=Project: " },
		}
	end,
	show = function(text)
		return "📘 " .. text
	end,
	process_output = function(text)
		return text:gsub("^📘%s*", "")
	end,
}
