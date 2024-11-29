return {
	command = function()
		return {
			command = "fuzzel",
			args = { "--dmenu", "--prompt=Project: " },
		}
	end,

	show = function(input)
		return input
	end,

	process_output = function(input)
		return input
	end,
}
