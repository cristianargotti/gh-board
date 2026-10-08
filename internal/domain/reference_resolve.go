package domain

import (
	"fmt"
	"regexp"
	"strings"
)

// repositoryPattern is the "owner/name" grammar of board.yml repository,
// the same owner and name rules the reference parser accepts, so that a
// resolved reference always parses back.
var repositoryPattern = regexp.MustCompile(`^([A-Za-z0-9](?:[A-Za-z0-9-]{0,37}[A-Za-z0-9])?)/([A-Za-z0-9_.-]{1,100})$`)

// SplitRepository parses the "owner/name" form of board.yml repository.
func SplitRepository(full string) (owner, name string, err error) {
	m := repositoryPattern.FindStringSubmatch(strings.TrimSpace(full))
	if m == nil {
		return "", "", fmt.Errorf("repository %q: expected owner/name: %w", full, ErrUsage)
	}
	return m[1], m[2], nil
}

// ResolveReference completes a number reference (#n or n) with the
// repository configured in board.yml. Without one the short form is a
// usage error, as section 5.2 requires; the other forms pass unchanged.
func ResolveReference(ref Reference, repository string) (Reference, error) {
	if !ref.NeedsRepository() {
		return ref, nil
	}
	if strings.TrimSpace(repository) == "" {
		return Reference{}, fmt.Errorf("reference %q needs repository in board.yml; use owner/repo#%d or the issue URL: %w", ref.Raw, ref.Number, ErrUsage)
	}
	owner, name, err := SplitRepository(repository)
	if err != nil {
		return Reference{}, err
	}
	return ref.WithRepository(owner, name), nil
}
