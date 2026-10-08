package guard

import "strings"

// An unquoted Windows executable path may contain spaces. Recognize it
// before POSIX backslash processing discards the directory separators.
func (s *scanner) windowsProgram() bool {
	if s.open || !s.windowsStart() {
		return false
	}
	for end := s.pos; end <= len(s.src); end++ {
		if end < len(s.src) && !strings.ContainsRune(" \t\r\n;&|", s.src[end]) {
			continue
		}
		path := string(s.src[s.pos:end])
		base := programBase(path)
		if base == "gh" || base == boardProgram || shellPrograms[base] {
			s.word.WriteString(path)
			s.open, s.pos = true, end
			return true
		}
		if end < len(s.src) && isSeparator(s.src[end]) {
			break
		}
	}
	return false
}

func (s *scanner) windowsStart() bool {
	return (s.peek(1) == ':' && (s.peek(2) == '\\' || s.peek(2) == '/')) ||
		(s.peek(0) == '\\' && s.peek(1) == '\\')
}
