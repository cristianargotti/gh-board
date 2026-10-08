package guard

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/cristianargotti/gh-board/internal/domain"
)

//go:embed patterns.json
var patternsJSON []byte

// Family groups the patterns of one kind of destructive command.
type Family struct {
	ID          string   `json:"id"`
	Description string   `json:"description"`
	Patterns    []string `json:"patterns"`
}

// Pattern is one compiled rule in Claude Code form without the Bash( )
// wrapper, for example "gh project delete *".
type Pattern struct {
	Family string
	Text   string
	re     *regexp.Regexp
}

type patternFile struct {
	Families []Family `json:"families"`
	Strict   Family   `json:"strict"`
}

type patternSet struct {
	families  []Family
	strict    Family
	normal    []Pattern
	strictSet []Pattern
	mutations []string
}

var loadSet = sync.OnceValues(func() (*patternSet, error) { return parse(patternsJSON) })

// parse decodes a patterns document and compiles every rule, refusing
// empty and duplicated rules so that the embedded source stays canonical.
func parse(data []byte) (*patternSet, error) {
	var file patternFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("guard patterns: %w", err)
	}
	set := &patternSet{families: file.Families, strict: file.Strict}
	seen := map[string]bool{}
	for _, fam := range file.Families {
		if err := set.add(fam, false, seen); err != nil {
			return nil, err
		}
	}
	if err := set.add(file.Strict, true, seen); err != nil {
		return nil, err
	}
	return set, nil
}

func (set *patternSet) add(fam Family, strict bool, seen map[string]bool) error {
	if fam.ID == "" || len(fam.Patterns) == 0 {
		return fmt.Errorf("guard patterns: family %q without id or patterns: %w", fam.ID, domain.ErrUsage)
	}
	for _, text := range fam.Patterns {
		if strings.TrimSpace(text) == "" || seen[text] {
			return fmt.Errorf("guard patterns: empty or duplicated rule %q in family %s: %w", text, fam.ID, domain.ErrUsage)
		}
		seen[text] = true
		p := compile(fam.ID, text)
		if strict {
			set.strictSet = append(set.strictSet, p)
			continue
		}
		set.normal = append(set.normal, p)
		if name, ok := mutationName(text); ok {
			set.mutations = append(set.mutations, name)
		}
	}
	return nil
}

// compile turns a Claude Code rule into a regular expression with the
// semantics the permissions documentation describes (verified against
// it): a star stands in for any text, spaces are literal, and a
// rule whose only wildcard is a trailing " *" also matches the bare
// command. So "gh project delete *" matches "gh project delete" while
// "git log * main" does not match "git log main".
func compile(family, text string) Pattern {
	parts := strings.Split(text, "*")
	var b strings.Builder
	b.WriteString("(?s)^")
	if len(parts) == 2 && parts[1] == "" && strings.HasSuffix(parts[0], " ") {
		b.WriteString(regexp.QuoteMeta(strings.TrimSuffix(parts[0], " ")))
		b.WriteString("(?: .*)?")
	} else {
		for i, part := range parts {
			if i > 0 {
				b.WriteString(".*")
			}
			b.WriteString(regexp.QuoteMeta(part))
		}
	}
	b.WriteString("$")
	return Pattern{Family: family, Text: text, re: regexp.MustCompile(b.String())}
}

const apiPrefix = "gh api *"

// mutationName extracts the mutation name of a "gh api *name*" rule.
func mutationName(text string) (string, bool) {
	if !strings.HasPrefix(text, apiPrefix) || !strings.HasSuffix(text, "*") {
		return "", false
	}
	name := strings.TrimSuffix(strings.TrimPrefix(text, apiPrefix), "*")
	if name == "" || strings.ContainsAny(name, "* -=@") {
		return "", false
	}
	return name, true
}

// Matches reports whether a normalized command segment matches the rule.
func (p Pattern) Matches(segment string) bool {
	return p.re.MatchString(segment)
}

// Patterns returns the compiled rules: the three families, plus the strict
// set when strict is true, in the order of patterns.json.
func Patterns(strict bool) ([]Pattern, error) {
	set, err := loadSet()
	if err != nil {
		return nil, err
	}
	out := append([]Pattern(nil), set.normal...)
	if strict {
		out = append(out, set.strictSet...)
	}
	return out, nil
}

// Rules returns the rule texts in Claude Code form without the Bash( )
// wrapper, in the same order as Patterns.
func Rules(strict bool) ([]string, error) {
	patterns, err := Patterns(strict)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(patterns))
	for _, p := range patterns {
		out = append(out, p.Text)
	}
	return out, nil
}

// Families returns the pattern families for documentation, with the
// strict family last.
func Families() ([]Family, error) {
	set, err := loadSet()
	if err != nil {
		return nil, err
	}
	out := append([]Family(nil), set.families...)
	return append(out, set.strict), nil
}

// ForbiddenMutations lists the GraphQL mutation names the api family
// denies, in rule order.
func ForbiddenMutations() ([]string, error) {
	set, err := loadSet()
	if err != nil {
		return nil, err
	}
	return append([]string(nil), set.mutations...), nil
}
