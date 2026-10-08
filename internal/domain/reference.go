package domain

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// RefKind is the syntactic form of a target reference.
type RefKind string

// Reference kinds accepted by every command that takes <ref>.
const (
	RefNodeID RefKind = "node_id" // a GraphQL node id such as I_kwDOAbc123
	RefFull   RefKind = "full"    // owner/repo#n
	RefNumber RefKind = "number"  // #n or n, needs repository in board.yml
	RefURL    RefKind = "url"     // https://github.com/owner/repo/issues/n
)

// Reference is a parsed target. Number forms resolve against the configured
// repository and only when exactly one project item carries that number.
type Reference struct {
	Kind   RefKind `json:"kind"`
	Raw    string  `json:"raw"`
	NodeID string  `json:"node_id,omitempty"`
	Owner  string  `json:"owner,omitempty"`
	Repo   string  `json:"repo,omitempty"`
	Number int     `json:"number,omitempty"`
	URL    string  `json:"url,omitempty"`
}

var (
	// nodeIDPattern matches the modern GitHub node ids (I_kwDO..., PVTI_lA...)
	// and the legacy base64 ones (MDU6SXNzdWU...).
	nodeIDPattern = regexp.MustCompile(`^(?:[A-Z]{1,8}_[A-Za-z0-9_-]{6,}|MD[A-Za-z0-9+/=]{10,})$`)
	fullPattern   = regexp.MustCompile(`^([A-Za-z0-9](?:[A-Za-z0-9-]{0,37}[A-Za-z0-9])?)/([A-Za-z0-9_.-]{1,100})#([1-9][0-9]{0,8})$`)
	numberPattern = regexp.MustCompile(`^#?([1-9][0-9]{0,8})$`)
	urlPathRe     = regexp.MustCompile(`^/([A-Za-z0-9](?:[A-Za-z0-9-]{0,37}[A-Za-z0-9])?)/([A-Za-z0-9_.-]{1,100})/(?:issues|pull)/([1-9][0-9]{0,8})/?$`)
)

// ParseReference parses one of the accepted forms. Malformed input is a
// usage error (exit code 1); resolution failures are the adapter's.
func ParseReference(s string) (Reference, error) {
	raw := strings.TrimSpace(s)
	if raw == "" {
		return Reference{}, fmt.Errorf("empty reference: %w", ErrUsage)
	}
	if m := numberPattern.FindStringSubmatch(raw); m != nil {
		return Reference{Kind: RefNumber, Raw: raw, Number: atoi(m[1])}, nil
	}
	if m := fullPattern.FindStringSubmatch(raw); m != nil {
		return Reference{Kind: RefFull, Raw: raw, Owner: m[1], Repo: m[2], Number: atoi(m[3])}, nil
	}
	if strings.HasPrefix(raw, "https://") || strings.HasPrefix(raw, "http://") {
		return parseURLReference(raw)
	}
	if nodeIDPattern.MatchString(raw) {
		return Reference{Kind: RefNodeID, Raw: raw, NodeID: raw}, nil
	}
	return Reference{}, fmt.Errorf("reference %q: expected a node id, owner/repo#n, #n or an issue URL: %w", raw, ErrUsage)
}

func parseURLReference(raw string) (Reference, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return Reference{}, fmt.Errorf("reference %q: expected an https issue URL: %w", raw, ErrUsage)
	}
	m := urlPathRe.FindStringSubmatch(u.Path)
	if m == nil {
		return Reference{}, fmt.Errorf("reference %q: expected /owner/repo/issues/n: %w", raw, ErrUsage)
	}
	return Reference{
		Kind: RefURL, Raw: raw, URL: raw,
		Owner: m[1], Repo: m[2], Number: atoi(m[3]),
	}, nil
}

func atoi(digits string) int {
	n, _ := strconv.Atoi(digits)
	return n
}

// String returns the canonical textual form of the reference.
func (r Reference) String() string {
	switch r.Kind {
	case RefNodeID:
		return r.NodeID
	case RefFull, RefURL:
		return r.Owner + "/" + r.Repo + "#" + strconv.Itoa(r.Number)
	case RefNumber:
		return "#" + strconv.Itoa(r.Number)
	default:
		return r.Raw
	}
}

// NeedsRepository reports whether the reference depends on the configured
// repository to resolve.
func (r Reference) NeedsRepository() bool { return r.Kind == RefNumber }

// WithRepository completes a number reference with the configured
// repository, turning it into the full form.
func (r Reference) WithRepository(owner, repo string) Reference {
	if r.Kind != RefNumber {
		return r
	}
	return Reference{Kind: RefFull, Raw: r.Raw, Owner: owner, Repo: repo, Number: r.Number}
}
