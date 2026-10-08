package guard

import "regexp"

var (
	assignmentRe  = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(\[[^\]]*\])?\+?=`)
	redirectionRe = regexp.MustCompile(`^(&>>?|[0-9]*(>>?|>\||<<?<?|<>|>&|<&))`)
)

// keywords are the shell reserved words that may precede a command in
// command position; the shell runs the command that follows them.
var keywords = set("{", "}", "!", "if", "then", "else", "elif", "fi", "while", "until", "do", "done")

// Normalize removes what the shell evaluates before the program runs and
// what wrapper programs add in front of it: leading variable assignments,
// reserved words, redirections and the wrappers of wrappers.go. A rule
// written for the program then matches "FOO=1 timeout 30 gh project delete 1"
// as well as the bare command.
func Normalize(seg Segment) Segment {
	words := []string(seg)
	for len(words) > 0 {
		next, changed := stripOne(words)
		if !changed {
			break
		}
		words = next
	}
	return Segment(words)
}

func stripOne(words []string) ([]string, bool) {
	head := words[0]
	switch {
	case assignmentRe.MatchString(head), keywords[head]:
		return words[1:], true
	case redirectionRe.MatchString(head):
		return dropRedirection(words), true
	}
	w, ok := wrappers[head]
	if !ok {
		return words, false
	}
	return w.strip(words[1:])
}

// dropRedirection removes an operator with an attached target ("2>/dev/null")
// or an operator and the separate word that follows it ("> out.txt").
func dropRedirection(words []string) []string {
	loc := redirectionRe.FindStringIndex(words[0])
	if loc[1] < len(words[0]) || len(words) == 1 {
		return words[1:]
	}
	return words[2:]
}
