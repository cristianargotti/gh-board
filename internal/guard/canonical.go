package guard

import "strings"

const boardProgram = "gh-board"

func programBase(program string) string {
	base := program[strings.LastIndexAny(program, `/\`)+1:]
	return strings.TrimSuffix(strings.ToLower(base), ".exe")
}

// Preserve paths so the strict path rule still rejects reads through them.
func canonicalCommand(seg Segment) Segment {
	words := append(Segment(nil), seg...)
	if len(words) == 0 {
		return words
	}
	base := programBase(words[0])
	if base != "gh" && base != boardProgram {
		return words
	}
	path := strings.ReplaceAll(words[0], `\`, "/")
	words[0] = path[:strings.LastIndex(path, "/")+1] + base
	if len(words) > 1 && base == "gh" && words[1] == "api" {
		normalizeMethods(words[2:])
	}
	return words
}

// Only method values fold case: subcommands and GraphQL names do not.
func normalizeMethods(args []string) {
	for i, arg := range args {
		switch {
		case (arg == "-X" || arg == "--method") && i+1 < len(args):
			args[i+1] = strings.ToUpper(args[i+1])
		case strings.HasPrefix(arg, "-X"):
			args[i] = "-X" + strings.ToUpper(arg[2:])
		case strings.HasPrefix(arg, "--method="):
			args[i] = "--method=" + strings.ToUpper(strings.TrimPrefix(arg, "--method="))
		}
	}
}
