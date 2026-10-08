package github

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cli/go-gh/v2/pkg/api"

	"github.com/cristianargotti/gh-board/internal/domain"
)

// GraphQL error types GitHub reports.
const (
	errorNotFound    = "NOT_FOUND"
	errorRateLimited = "RATE_LIMITED"
)

// APIError is a failure reported by GitHub or by the transport. It unwraps
// to domain.ErrNotFound when GitHub reports every target missing, to
// domain.ErrAmbiguous when a short reference matches several items, and
// to domain.ErrAPI (exit code 7) otherwise. It never carries the token.
type APIError struct {
	// Operation is the document name or the REST path that failed.
	Operation string
	// Status is the HTTP status when the failure was an HTTP error.
	Status int
	// Message is the text GitHub or the transport reported.
	Message string
	// Errors are the GraphQL error items, when any.
	Errors []ErrorItem
	// RateLimit reports a primary or secondary rate limit.
	RateLimit bool
	// Auth reports a missing or rejected token.
	Auth bool

	kind error
}

// ErrorItem is one GraphQL error with its type and dotted path.
type ErrorItem struct {
	Type    string
	Path    string
	Message string
}

// Error describes the failure, kind first, so the exit code is visible.
func (e *APIError) Error() string {
	var b strings.Builder
	b.WriteString(e.kind.Error())
	b.WriteString(": ")
	b.WriteString(e.Operation)
	if e.Status != 0 {
		b.WriteString(": HTTP ")
		b.WriteString(strconv.Itoa(e.Status))
	}
	if e.Message != "" {
		b.WriteString(": ")
		b.WriteString(e.Message)
	}
	switch {
	case e.RateLimit:
		b.WriteString(" (rate limited)")
	case e.Auth:
		b.WriteString(" (authentication failed)")
	}
	return b.String()
}

// Unwrap exposes the kind so that domain.CodeOf maps the exit code.
func (e *APIError) Unwrap() error { return e.kind }

// OnlyNotFoundAt reports whether every error is a NOT_FOUND under one of
// the paths, which a reader tolerates when another path answered.
func (e *APIError) OnlyNotFoundAt(paths ...string) bool {
	if len(e.Errors) == 0 {
		return false
	}
	for _, item := range e.Errors {
		if item.Type != errorNotFound || !underPath(item.Path, paths) {
			return false
		}
	}
	return true
}

func underPath(p string, roots []string) bool {
	for _, root := range roots {
		if p == root || strings.HasPrefix(p, root+".") {
			return true
		}
	}
	return false
}

// wrapError maps a go-gh error to an APIError.
func wrapError(op string, err error) error {
	var gqlErr *api.GraphQLError
	if errors.As(err, &gqlErr) {
		return fromGraphQL(op, gqlErr)
	}
	var httpErr *api.HTTPError
	if errors.As(err, &httpErr) {
		return fromHTTP(op, httpErr)
	}
	return &APIError{Operation: op, Message: err.Error(), kind: domain.ErrAPI}
}

func fromGraphQL(op string, gqlErr *api.GraphQLError) *APIError {
	e := &APIError{Operation: op, kind: domain.ErrAPI}
	notFound := len(gqlErr.Errors) > 0
	messages := make([]string, 0, len(gqlErr.Errors))
	for _, item := range gqlErr.Errors {
		e.Errors = append(e.Errors, ErrorItem{Type: item.Type, Path: joinPath(item.Path), Message: item.Message})
		messages = append(messages, item.Message)
		if item.Type != errorNotFound {
			notFound = false
		}
		if item.Type == errorRateLimited {
			e.RateLimit = true
		}
	}
	if notFound {
		e.kind = domain.ErrNotFound
	}
	e.Message = strings.Join(messages, "; ")
	return e
}

func joinPath(parts []any) string {
	names := make([]string, 0, len(parts))
	for _, p := range parts {
		names = append(names, fmt.Sprint(p))
	}
	return strings.Join(names, ".")
}

func fromHTTP(op string, httpErr *api.HTTPError) *APIError {
	e := &APIError{Operation: op, Status: httpErr.StatusCode, Message: httpErr.Message, kind: domain.ErrAPI}
	switch {
	case httpErr.StatusCode == http.StatusUnauthorized:
		e.Auth = true
	case httpErr.StatusCode == http.StatusNotFound:
		e.kind = domain.ErrNotFound
	case rateLimited(httpErr):
		e.RateLimit = true
		if reset := resetTime(httpErr.Headers); reset != "" {
			e.Message += " (resets at " + reset + ")"
		}
	}
	return e
}

// rateLimited recognizes the primary limit (403 or 429 with the remaining
// counter at zero), the secondary limit (Retry-After) and the message.
func rateLimited(httpErr *api.HTTPError) bool {
	if httpErr.StatusCode != http.StatusForbidden && httpErr.StatusCode != http.StatusTooManyRequests {
		return false
	}
	return httpErr.Headers.Get("Retry-After") != "" ||
		httpErr.Headers.Get("X-RateLimit-Remaining") == "0" ||
		strings.Contains(strings.ToLower(httpErr.Message), "rate limit")
}

func resetTime(h http.Header) string {
	epoch, err := strconv.ParseInt(h.Get("X-RateLimit-Reset"), 10, 64)
	if err != nil {
		return ""
	}
	return time.Unix(epoch, 0).UTC().Format(time.RFC3339)
}

// notFound is the error of a target that resolved to nothing.
func notFound(op, what string) error {
	return &APIError{Operation: op, Message: what, kind: domain.ErrNotFound}
}

// ambiguous is the error of a short reference that matched several items.
func ambiguous(op, what string) error {
	return &APIError{Operation: op, Message: what, kind: domain.ErrAmbiguous}
}

// usage is a malformed input the adapter refuses before any request.
func usage(format string, args ...any) error {
	return fmt.Errorf(format+": %w", append(args, domain.ErrUsage)...)
}
