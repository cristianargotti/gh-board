package github_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/github"
)

type errorCase struct {
	name     string
	status   int
	headers  map[string]string
	body     string
	wantKind error
	wantCode domain.ExitCode
	rate     bool
	auth     bool
	contains string
}

var errorCases = []errorCase{
	{
		name: "graphql rate limited", status: 200,
		body:     `{"data":null,"errors":[{"type":"RATE_LIMITED","message":"API rate limit exceeded for user ID 1."}]}`,
		wantKind: domain.ErrAPI, wantCode: domain.ExitAPI, rate: true, contains: "(rate limited)",
	},
	{
		name: "graphql forbidden", status: 200,
		body:     `{"data":null,"errors":[{"type":"FORBIDDEN","path":["viewer"],"message":"Resource not accessible by integration"}]}`,
		wantKind: domain.ErrAPI, wantCode: domain.ExitAPI, contains: "Resource not accessible",
	},
	{
		name: "graphql every path missing", status: 200,
		body:     `{"data":{"viewer":null},"errors":[{"type":"NOT_FOUND","path":["viewer"],"message":"gone"}]}`,
		wantKind: domain.ErrNotFound, wantCode: domain.ExitNotFound, contains: "target not found: viewer: gone",
	},
	{
		name: "http unauthorized", status: 401,
		body:     `{"message":"Bad credentials","documentation_url":"https://docs.github.com/rest","status":"401"}`,
		wantKind: domain.ErrAPI, wantCode: domain.ExitAPI, auth: true, contains: "HTTP 401: Bad credentials (authentication failed)",
	},
	{
		name: "http secondary rate limit", status: 403,
		headers:  map[string]string{"Retry-After": "60"},
		body:     `{"message":"You have exceeded a secondary rate limit. Please wait a few minutes before you try again."}`,
		wantKind: domain.ErrAPI, wantCode: domain.ExitAPI, rate: true,
	},
	{
		name: "http primary rate limit with reset", status: 429,
		headers:  map[string]string{"X-RateLimit-Remaining": "0", "X-RateLimit-Reset": "1791000000"},
		body:     `{"message":"API rate limit exceeded"}`,
		wantKind: domain.ErrAPI, wantCode: domain.ExitAPI, rate: true, contains: "resets at 2026-10-03T04:00:00Z",
	},
	{
		name: "http forbidden without rate limit", status: 403,
		body:     `{"message":"Must have admin rights to Repository."}`,
		wantKind: domain.ErrAPI, wantCode: domain.ExitAPI, contains: "HTTP 403",
	},
	{
		name: "http not found", status: 404,
		body:     `{"message":"Not Found"}`,
		wantKind: domain.ErrNotFound, wantCode: domain.ExitNotFound,
	},
	{
		name: "http gateway without json", status: 502,
		headers:  map[string]string{"Content-Type": "text/html"},
		body:     `<html>bad gateway</html>`,
		wantKind: domain.ErrAPI, wantCode: domain.ExitAPI, contains: "502 Bad Gateway",
	},
}

func TestErrorMapping(t *testing.T) {
	for _, tc := range errorCases {
		t.Run(tc.name, func(t *testing.T) {
			isolateEnv(t)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				for k, v := range tc.headers {
					w.Header().Set(k, v)
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			t.Cleanup(server.Close)
			_, err := newAdapterFor(t, server.URL).Viewer(context.Background())
			assertAPIError(t, err, tc)
		})
	}
}

func assertAPIError(t *testing.T, err error, tc errorCase) {
	t.Helper()
	var apiErr *github.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error %v is not an APIError", err)
	}
	if !errors.Is(err, tc.wantKind) || domain.CodeOf(err) != tc.wantCode {
		t.Fatalf("kind of %v: code %d, want %v", err, domain.CodeOf(err), tc.wantKind)
	}
	if apiErr.RateLimit != tc.rate || apiErr.Auth != tc.auth {
		t.Fatalf("rate %v auth %v, want %v %v", apiErr.RateLimit, apiErr.Auth, tc.rate, tc.auth)
	}
	if tc.contains != "" && !strings.Contains(err.Error(), tc.contains) {
		t.Fatalf("%q does not contain %q", err.Error(), tc.contains)
	}
	if apiErr.Operation != "viewer" {
		t.Fatalf("operation = %q", apiErr.Operation)
	}
}

func TestTransportFailureIsAPIError(t *testing.T) {
	isolateEnv(t)
	server := httptest.NewServer(http.NotFoundHandler())
	url := server.URL
	server.Close()
	_, err := newAdapterFor(t, url).Viewer(context.Background())
	if !errors.Is(err, domain.ErrAPI) || domain.CodeOf(err) != domain.ExitAPI {
		t.Fatalf("expected ErrAPI, got %v", err)
	}
}

func TestOnlyNotFoundAt(t *testing.T) {
	e := &github.APIError{Errors: []github.ErrorItem{
		{Type: "NOT_FOUND", Path: "user"},
		{Type: "NOT_FOUND", Path: "organization.projectV2"},
	}}
	if !e.OnlyNotFoundAt("organization", "user") {
		t.Fatal("both paths are under the roots")
	}
	if e.OnlyNotFoundAt("organization") {
		t.Fatal("user is not under organization")
	}
	e.Errors = append(e.Errors, github.ErrorItem{Type: "FORBIDDEN", Path: "user"})
	if e.OnlyNotFoundAt("organization", "user") {
		t.Fatal("a FORBIDDEN item is never tolerated")
	}
	if (&github.APIError{}).OnlyNotFoundAt("user") {
		t.Fatal("no items means nothing to tolerate")
	}
}

func TestConnectWithoutAnyToken(t *testing.T) {
	isolateEnv(t)
	t.Setenv("GH_TOKEN", "")
	_, err := github.New(github.Options{Host: "github.com"}).Viewer(context.Background())
	var apiErr *github.APIError
	if !errors.As(err, &apiErr) || !apiErr.Auth || !errors.Is(err, domain.ErrAPI) {
		t.Fatalf("expected an authentication APIError, got %v", err)
	}
	if !strings.Contains(err.Error(), "gh auth login") || strings.Contains(err.Error(), replayToken) {
		t.Fatalf("message must point to gh auth login and never carry a token: %q", err.Error())
	}
}
