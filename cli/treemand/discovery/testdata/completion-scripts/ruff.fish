# Print an optspec for argparse to handle cmd's options that are independent of any subcommand.
function __fish_ruff_global_optspecs
	string join \n v/verbose q/quiet s/silent config= isolated color= h/help V/version
end

function __fish_ruff_needs_command
	# Figure out if the current invocation already has a command.
	set -l cmd (commandline -opc)
	set -e cmd[1]
	argparse -s (__fish_ruff_global_optspecs) -- $cmd 2>/dev/null
	or return
	if set -q argv[1]
		# Also print the command, so this can be used to figure out what it is.
		echo $argv[1]
		return 1
	end
	return 0
end

function __fish_ruff_using_subcommand
	set -l cmd (__fish_ruff_needs_command)
	test -z "$cmd"
	and return 1
	contains -- $cmd[1] $argv
end

complete -c ruff -n "__fish_ruff_needs_command" -l config -d 'Either a path to a TOML configuration file (`pyproject.toml` or `ruff.toml`), or a TOML `<KEY> = <VALUE>` pair (such as you might find in a `ruff.toml` configuration file) overriding a specific configuration option (e.g., `--config "line-length = 100"` or `--config "format.quote-style = \'single\'"`). Overrides of individual settings using this option always take precedence over all configuration files, including configuration files that were also specified using `--config`' -r
complete -c ruff -n "__fish_ruff_needs_command" -l color -d 'Control when colored output is used' -r -f -a "auto\t'Use color when writing to a terminal, unless overridden by the environment'
always\t'Always display colors'
never\t'Never display colors'"
complete -c ruff -n "__fish_ruff_needs_command" -s v -l verbose -d 'Enable verbose logging'
complete -c ruff -n "__fish_ruff_needs_command" -s q -l quiet -d 'Print diagnostics, but nothing else'
complete -c ruff -n "__fish_ruff_needs_command" -s s -l silent -d 'Disable all logging (but still exit with status code "1" upon detecting diagnostics)'
complete -c ruff -n "__fish_ruff_needs_command" -l isolated -d 'Ignore all configuration files'
complete -c ruff -n "__fish_ruff_needs_command" -s h -l help -d 'Print help (see more with \'--help\')'
complete -c ruff -n "__fish_ruff_needs_command" -s V -l version -d 'Print version'
complete -c ruff -n "__fish_ruff_needs_command" -f -a "check" -d 'Run Ruff on the given files or directories'
complete -c ruff -n "__fish_ruff_needs_command" -f -a "rule" -d 'Explain a rule (or all rules)'
complete -c ruff -n "__fish_ruff_needs_command" -f -a "config" -d 'List or describe the available configuration options'
complete -c ruff -n "__fish_ruff_needs_command" -f -a "linter" -d 'List all supported upstream linters'
complete -c ruff -n "__fish_ruff_needs_command" -f -a "clean" -d 'Clear any caches in the current directory and any subdirectories'
complete -c ruff -n "__fish_ruff_needs_command" -f -a "generate-shell-completion" -d 'Generate shell completion'
complete -c ruff -n "__fish_ruff_needs_command" -f -a "format" -d 'Run the Ruff formatter on the given files or directories'
complete -c ruff -n "__fish_ruff_needs_command" -f -a "server" -d 'Run the language server'
complete -c ruff -n "__fish_ruff_needs_command" -f -a "analyze" -d 'Run analysis over Python source code'
complete -c ruff -n "__fish_ruff_needs_command" -f -a "version" -d 'Display Ruff\'s version'
complete -c ruff -n "__fish_ruff_needs_command" -f -a "help" -d 'Print this message or the help of the given subcommand(s)'
complete -c ruff -n "__fish_ruff_using_subcommand analyze; and not __fish_seen_subcommand_from graph help" -s v -l verbose -d 'Enable verbose logging'
complete -c ruff -n "__fish_ruff_using_subcommand analyze; and not __fish_seen_subcommand_from graph help" -f -a "graph" -d 'Generate a map of Python file dependencies or dependents'
complete -c ruff -n "__fish_ruff_using_subcommand analyze; and not __fish_seen_subcommand_from graph help" -f -a "help" -d 'Print this message or the help of the given subcommand(s)'
complete -c ruff -n "__fish_ruff_using_subcommand analyze; and __fish_seen_subcommand_from graph" -l min-dots -d 'The minimum number of dots in a string import to consider it a valid import' -r
complete -c ruff -n "__fish_ruff_using_subcommand analyze; and __fish_seen_subcommand_from graph" -l python -d 'Path to a virtual environment to use for resolving additional dependencies' -r -F
