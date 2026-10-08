package guard

import "strings"

// heredoc is a pending "<<" operator whose body starts after the current
// line. The body joins the segment that reads it, so a mutation written in
// a heredoc is visible to the rules of the command that sends it.
type heredoc struct {
	delim    string
	stripTab bool
	owner    int
	ownerCur bool
}

// noteHeredoc records a completed word that opens a heredoc. The
// delimiter may be attached ("<<EOF", "<<-EOF") or the next word ("<< EOF");
// quotes around it were already resolved by the tokenizer.
func (s *scanner) noteHeredoc(word string) {
	if s.await {
		s.await = false
		s.pending = append(s.pending, heredoc{delim: word, stripTab: s.awaitStrip, ownerCur: true})
		return
	}
	if strings.HasPrefix(word, "<<<") || !strings.HasPrefix(word, "<<") {
		return
	}
	rest := strings.TrimPrefix(word, "<<")
	strip := strings.HasPrefix(rest, "-")
	rest = strings.TrimPrefix(rest, "-")
	if rest == "" {
		s.await, s.awaitStrip = true, strip
		return
	}
	s.pending = append(s.pending, heredoc{delim: rest, stripTab: strip, ownerCur: true})
}

// claimOwner binds every heredoc still waiting for its segment to the
// segment that was just flushed at index i.
func (s *scanner) claimOwner(i int) {
	for k := range s.pending {
		if s.pending[k].ownerCur {
			s.pending[k].ownerCur = false
			s.pending[k].owner = i
		}
	}
}

// readHeredocs consumes the bodies of the pending heredocs, in order,
// starting at the current position (just after the newline that ended the
// command line).
func (s *scanner) readHeredocs() {
	for _, h := range s.pending {
		s.readBody(h)
	}
	s.pending = nil
	s.await = false
}

func (s *scanner) readBody(h heredoc) {
	for s.pos < len(s.src) {
		end := s.pos
		for end < len(s.src) && s.src[end] != '\n' {
			end++
		}
		line := string(s.src[s.pos:end])
		s.pos = end
		if end < len(s.src) {
			s.pos++
		}
		if h.stripTab {
			line = strings.TrimLeft(line, "\t")
		}
		if strings.TrimRight(line, "\r") == h.delim {
			return
		}
		s.attach(h, strings.Fields(line))
	}
}

func (s *scanner) attach(h heredoc, words []string) {
	if h.ownerCur || h.owner >= len(s.segs) {
		s.cur = append(s.cur, words...)
		s.open = s.open || len(words) > 0
		return
	}
	s.segs[h.owner] = append(s.segs[h.owner], words...)
}
