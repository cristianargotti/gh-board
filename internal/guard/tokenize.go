package guard

import "strings"

// Segment is one simple command: the words of a pipeline element after
// splitting on shell separators and extracting substitutions.
type Segment []string

// String joins the words with single spaces, the form the rules match.
func (seg Segment) String() string {
	return strings.Join(seg, " ")
}

// maxDepth bounds the nesting of subshells and substitutions so that a
// hostile command line cannot exhaust the stack.
const maxDepth = 64

type scanner struct {
	src        []rune
	pos        int
	depth      int
	segs       []Segment
	cur        Segment
	word       strings.Builder
	open       bool
	pending    []heredoc
	await      bool
	awaitStrip bool
}

// Tokenize splits a command line into segments the way the guard needs to
// see them: quotes and backslash escapes are resolved, the separators ";",
// "&", "&&", "|", "||", "|&" and newlines end a segment, subshells, "$( )"
// and backtick substitutions become segments of their own, and heredoc
// bodies join the command that reads them. Expansions stay verbatim: the
// guard matches program names and arguments, it does not run the shell.
func Tokenize(command string) []Segment {
	s := &scanner{src: []rune(command)}
	s.scan(0)
	s.flush()
	return s.segs
}

func (s *scanner) scan(stop rune) {
	for s.pos < len(s.src) {
		c := s.src[s.pos]
		if stop != 0 && c == stop {
			s.pos++
			s.flush()
			return
		}
		s.step(c)
	}
}

func (s *scanner) step(c rune) {
	switch {
	case s.windowsProgram():
	case c == '\'':
		s.pos++
		s.singleQuoted()
	case c == '"':
		s.pos++
		s.doubleQuoted()
	case c == '\\':
		s.escape()
	case c == '`':
		s.pos++
		s.nested('`')
	case c == '$' && s.peek(1) == '(':
		s.pos += 2
		s.nested(')')
	case c == '(':
		s.pos++
		s.flush()
		s.nested(')')
	case c == ')':
		s.pos++
		s.flush()
	case isSeparator(c):
		s.separator()
	case c == ' ' || c == '\t':
		s.pos++
		s.endWord()
	default:
		s.pos++
		s.word.WriteRune(c)
		s.open = true
	}
}

// nested scans a substitution as a command list of its own. The outer word
// continues without the substituted text, which the shell replaces anyway.
// Heredocs opened by the outer line keep waiting for the outer newline.
func (s *scanner) nested(stop rune) {
	if s.depth >= maxDepth {
		return
	}
	outerCur, outerWord, outerOpen := s.cur, s.word.String(), s.open
	outerPending := s.pending
	s.cur, s.open, s.pending = nil, false, nil
	s.word.Reset()
	s.depth++
	s.scan(stop)
	s.depth--
	s.flush()
	s.cur, s.open = outerCur, outerOpen
	s.pending = append(outerPending, s.pending...)
	s.word.Reset()
	s.word.WriteString(outerWord)
}

func (s *scanner) singleQuoted() {
	s.open = true
	for s.pos < len(s.src) && s.src[s.pos] != '\'' {
		s.word.WriteRune(s.src[s.pos])
		s.pos++
	}
	s.pos++
}

func (s *scanner) doubleQuoted() {
	s.open = true
	for s.pos < len(s.src) {
		c := s.src[s.pos]
		switch {
		case c == '"':
			s.pos++
			return
		case c == '\\' && strings.ContainsRune("\"\\$`\n", s.peek(1)):
			s.word.WriteRune(s.peek(1))
			s.pos += 2
		case c == '$' && s.peek(1) == '(':
			s.pos += 2
			s.nested(')')
		case c == '`':
			s.pos++
			s.nested('`')
		default:
			s.word.WriteRune(c)
			s.pos++
		}
	}
}

func (s *scanner) escape() {
	next := s.peek(1)
	s.pos += 2
	if next == 0 || next == '\n' {
		return
	}
	s.word.WriteRune(next)
	s.open = true
}

// separator ends the current segment. A newline also starts the bodies of
// the heredocs opened on the line that just ended.
func (s *scanner) separator() {
	s.flush()
	for s.pos < len(s.src) && isSeparator(s.src[s.pos]) {
		c := s.src[s.pos]
		s.pos++
		if c == '\n' && len(s.pending) > 0 {
			s.readHeredocs()
			return
		}
	}
}

func (s *scanner) peek(n int) rune {
	if s.pos+n >= len(s.src) {
		return 0
	}
	return s.src[s.pos+n]
}

func (s *scanner) endWord() {
	if !s.open {
		return
	}
	if w := s.word.String(); w != "" {
		s.noteHeredoc(w)
		s.cur = append(s.cur, w)
	}
	s.word.Reset()
	s.open = false
}

func (s *scanner) flush() {
	s.endWord()
	if len(s.cur) == 0 {
		return
	}
	s.segs = append(s.segs, s.cur)
	s.cur = nil
	s.claimOwner(len(s.segs) - 1)
}

func isSeparator(c rune) bool {
	return c == ';' || c == '&' || c == '|' || c == '\n' || c == '\r'
}
