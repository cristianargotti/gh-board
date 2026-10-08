package render

import "strings"

// undelimited returns a copy of the document with the data delimiters
// removed from every string. The table format is for people, who read
// a title as a title; the agents read compact, md and json, where the
// delimiters stay. The text inside a span is already sanitized and
// truncated, and Delimit broke every bracket pair inside it, so nothing
// a span held can become a tag once the real tags are gone.
func (d *Document) undelimited() *Document {
	return d.mapped(undelimit)
}

// undelimit removes every "[[begin:label]] text [[end:label]]" span of
// the one-line form of Delimit from s, keeping the text. A span without
// its closing tag, or with another label on it, is left as it is.
func undelimit(s string) string {
	if !strings.Contains(s, dataOpen) {
		return s
	}
	var b strings.Builder
	for {
		start := strings.Index(s, dataOpen)
		if start < 0 {
			break
		}
		inner, end, ok := span(s[start:])
		if !ok {
			b.WriteString(s[:start+len(dataOpen)])
			s = s[start+len(dataOpen):]
			continue
		}
		b.WriteString(s[:start])
		b.WriteString(inner)
		s = s[start+end:]
	}
	b.WriteString(s)
	return b.String()
}

// span reads one delimited span at the start of s and returns its text
// and the length of the whole span.
func span(s string) (inner string, end int, ok bool) {
	labelEnd := strings.Index(s, dataEnd)
	if labelEnd < 0 {
		return "", 0, false
	}
	label := s[len(dataOpen):labelEnd]
	closing := dataClose + label + dataEnd
	rest := s[labelEnd+len(dataEnd):]
	closeAt := strings.Index(rest, closing)
	if closeAt < 0 {
		return "", 0, false
	}
	inner = strings.TrimSpace(rest[:closeAt])
	return inner, labelEnd + len(dataEnd) + closeAt + len(closing), true
}
