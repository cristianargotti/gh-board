package guard

// Match is the rule a command segment violated.
type Match struct {
	// Rule is the rule text in Claude Code form, without the Bash( ) wrapper.
	Rule string
	// Family is the id of the family the rule belongs to.
	Family string
	// Segment is the normalized simple command that matched.
	Segment string
}

// Evaluate tokenizes a command line the way section 6.5 describes, splits
// it on the shell separators, strips the wrappers and reports the first
// segment that matches a rule. Strict adds the opt-in strict family.
func Evaluate(command string, strict bool) (Match, bool, error) {
	patterns, err := Patterns(true)
	if err != nil {
		return Match{}, false, err
	}
	m, denied := evaluate(command, strict, patterns, 0)
	return m, denied, nil
}

func evaluate(command string, strict bool, patterns []Pattern, depth int) (Match, bool) {
	for _, seg := range Tokenize(command) {
		words := canonicalCommand(Normalize(seg))
		wrapper, script, wrapped := shellScript(words)
		// At the nesting limit, use the source's strict wrapper rule rather
		// than silently allowing a script that cannot be inspected further.
		checkStrict := strict || (wrapped && depth >= maxDepth)
		if wrapped && checkStrict {
			words = wrapper
		}
		if m, denied := matchSegment(words.String(), patterns, checkStrict); denied {
			return m, true
		}
		if wrapped && depth < maxDepth {
			if m, denied := evaluate(script, strict, patterns, depth+1); denied {
				return m, true
			}
		}
	}
	return Match{}, false
}

func matchSegment(text string, patterns []Pattern, strict bool) (Match, bool) {
	for _, p := range patterns {
		if (strict || p.Family != "strict") && p.Matches(text) {
			return Match{Rule: p.Text, Family: p.Family, Segment: text}, true
		}
	}
	return Match{}, false
}
