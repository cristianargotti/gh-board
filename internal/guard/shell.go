package guard

import "strings"

// Shell parsing is shared by string and argv inputs so strict mode cannot
// lose the outer wrapper while the command is decoded.
func shellScript(words Segment) (Segment, string, bool) {
	if len(words) < 2 {
		return nil, "", false
	}
	base := programBase(words[0])
	if !shellPrograms[base] {
		return nil, "", false
	}
	for i := 1; i < len(words); i++ {
		if flag := commandFlag(base, words[i]); flag != "" {
			return Segment{base, flag, strings.Join(words[i+1:], " ")}, shellBody(base, words[i+1:]), true
		}
	}
	return nil, "", false
}

func commandFlag(shell, arg string) string {
	flag := strings.ToLower(arg)
	if shell == "cmd" {
		if flag == "/c" {
			return flag
		}
		return ""
	}
	if isPowerShell(shell) {
		if flag == "-command" {
			return "-Command"
		}
		if flag == "-c" {
			return flag
		}
		return ""
	}
	if strings.HasPrefix(flag, "-") && !strings.HasPrefix(flag, "--") && strings.Contains(flag, "c") {
		if flag == "-lc" && (shell == "sh" || shell == "bash" || shell == "zsh") {
			return flag
		}
		return "-c"
	}
	return ""
}

func shellBody(shell string, args []string) string {
	if isPowerShell(shell) {
		args = powerShellOptions(args)
	}
	if len(args) == 0 {
		return ""
	}
	if shell == "cmd" || isPowerShell(shell) {
		return strings.Join(args, " ")
	}
	return args[0]
}

func isPowerShell(shell string) bool {
	return shell == "powershell" || shell == "pwsh"
}

var powerShellSwitches = set("-noprofile", "-nologo", "-noninteractive", "-noexit", "-sta", "-mta")

var powerShellValues = set("-executionpolicy", "-windowstyle", "-inputformat", "-outputformat", "-workingdirectory", "-configurationname", "-version")

// Inspect a command even if launch switches were placed after -Command.
// Strict mode still rejects the wrapper before evaluating its contents.
func powerShellOptions(args []string) []string {
	for len(args) > 0 {
		flag := strings.ToLower(args[0])
		switch {
		case powerShellSwitches[flag]:
			args = args[1:]
		case powerShellValues[flag] && len(args) > 1:
			args = args[2:]
		default:
			return args
		}
	}
	return args
}
