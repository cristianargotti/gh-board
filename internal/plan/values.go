package plan

import (
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/cristianargotti/gh-board/internal/domain"
)

// JoinSet encodes a set of names as the After value of a set step:
// sorted, comma separated, no spaces, the form domain.EncodeAssignees and
// domain.EncodeLabels produce. Producers and apply share it.
func JoinSet(values []string) string {
	set := make([]string, 0, len(values))
	for _, v := range values {
		if v = strings.TrimSpace(v); v != "" {
			set = append(set, v)
		}
	}
	sort.Strings(set)
	return strings.Join(set, ",")
}

// SplitSet decodes what JoinSet produced.
func SplitSet(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return strings.Split(JoinSet(strings.Split(value, ",")), ",")
}

// difference returns the values of a that are not in b, ignoring case.
func difference(a, b []string) []string {
	var out []string
	for _, v := range a {
		if !containsFold(b, v) {
			out = append(out, v)
		}
	}
	return out
}

// containsFold reports whether list holds value, ignoring case.
func containsFold(list []string, value string) bool {
	for _, v := range list {
		if strings.EqualFold(v, value) {
			return true
		}
	}
	return false
}

// splitRepo splits "owner/name" into its parts.
func splitRepo(full string) (string, string) {
	owner, name, _ := strings.Cut(full, "/")
	return owner, name
}

// Label names a target for humans: owner/repo#n with the title, or the
// repository, or the node id. Titles are external text, so control
// characters go and long ones are cut.
func Label(t domain.Target) string {
	var ref string
	switch {
	case t.Repository != "" && t.Number > 0:
		ref = t.Repository + "#" + strconv.Itoa(t.Number)
	case t.Repository != "":
		ref = t.Repository
	default:
		ref = t.NodeID
	}
	if title := clean(t.Title, 80); title != "" {
		ref += " " + strconv.Quote(title)
	}
	return ref
}

// clean drops control characters and cuts the text at limit runes.
func clean(text string, limit int) string {
	runes := make([]rune, 0, len(text))
	for _, r := range strings.TrimSpace(text) {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			r = ' '
		}
		runes = append(runes, r)
	}
	if len(runes) > limit {
		runes = append(runes[:limit], []rune("...")...)
	}
	return strings.Join(strings.Fields(string(runes)), " ")
}
