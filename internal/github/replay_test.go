package github_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/cristianargotti/gh-board/internal/github"
)

// The replay token go-gh reads from GH_TOKEN; the server asserts it arrives
// as the Authorization header and the tests never print it.
const replayToken = "replay-token"

// testNow is the fixed instant of every replay adapter.
var testNow = time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)

// exchange is one recorded request and its response under testdata/replay.
type exchange struct {
	Request struct {
		Operation string         `json:"operation"`
		Method    string         `json:"method"`
		Path      string         `json:"path"`
		Match     map[string]any `json:"match"`
	} `json:"request"`
	Response struct {
		Status  int               `json:"status"`
		Headers map[string]string `json:"headers"`
		Body    json.RawMessage   `json:"body"`
		// Text is a verbatim body for the asset download fixtures, which
		// are not JSON.
		Text string `json:"text"`
	} `json:"response"`
}

// call is what the adapter sent, kept for assertions on variables.
type call struct {
	Operation string
	Variables map[string]any
	Method    string
	Path      string
	Body      map[string]any
	Auth      string
}

// replay serves recorded exchanges over httptest and records the calls.
type replay struct {
	t         *testing.T
	exchanges []exchange

	mu    sync.Mutex
	calls []call
}

var operationPattern = regexp.MustCompile(`(?m)^\s*(?:query|mutation)\s+(\w+)`)

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

// rewriteTransport sends every request to the replay server, whatever host
// go-gh resolved, after go-gh added its headers.
type rewriteTransport struct{ target *url.URL }

func (rt rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.URL.Scheme = rt.target.Scheme
	clone.URL.Host = rt.target.Host
	clone.Host = rt.target.Host
	return http.DefaultTransport.RoundTrip(clone)
}

// isolateEnv makes go-gh read the replay token from GH_TOKEN and nothing
// from the developer's gh configuration or keyring.
func isolateEnv(t *testing.T) {
	t.Helper()
	t.Setenv("GH_TOKEN", replayToken)
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_HOST", "")
	t.Setenv("GH_DEBUG", "")
	t.Setenv("GH_CONFIG_DIR", t.TempDir())
	t.Setenv("GH_PATH", filepath.Join(t.TempDir(), "gh-not-installed"))
}

func loadExchanges(t *testing.T, dirs ...string) []exchange {
	t.Helper()
	var out []exchange
	for _, dir := range dirs {
		files, err := filepath.Glob(filepath.Join("testdata", "replay", dir, "*.json"))
		if err != nil || len(files) == 0 {
			t.Fatalf("no fixtures under testdata/replay/%s: %v", dir, err)
		}
		for _, f := range files {
			data, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			var ex exchange
			if err := json.Unmarshal(data, &ex); err != nil {
				t.Fatalf("%s: %v", f, err)
			}
			out = append(out, ex)
		}
	}
	return out
}

// newReplay starts a server over the fixtures of dirs and returns an
// adapter that reaches it through the rewriting transport.
func newReplay(t *testing.T, dirs ...string) (*replay, *github.Adapter) {
	t.Helper()
	isolateEnv(t)
	r := &replay{t: t, exchanges: loadExchanges(t, dirs...)}
	server := httptest.NewServer(r)
	t.Cleanup(server.Close)
	return r, newAdapterFor(t, server.URL)
}

func newAdapterFor(t *testing.T, serverURL string) *github.Adapter {
	t.Helper()
	target, err := url.Parse(serverURL)
	if err != nil {
		t.Fatal(err)
	}
	return github.New(github.Options{
		CacheDir:  t.TempDir(),
		Transport: rewriteTransport{target: target},
		Clock:     fixedClock{now: testNow},
	})
}

func (r *replay) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	body, _ := io.ReadAll(req.Body)
	c := call{Method: req.Method, Path: req.URL.Path, Auth: req.Header.Get("Authorization")}
	var ex *exchange
	if req.URL.Path == "/graphql" {
		var payload struct {
			Query     string         `json:"query"`
			Variables map[string]any `json:"variables"`
		}
		_ = json.Unmarshal(body, &payload)
		c.Operation = operationOf(payload.Query)
		c.Variables = payload.Variables
		ex = r.findGraphQL(c.Operation, c.Variables)
	} else {
		_ = json.Unmarshal(body, &c.Body)
		ex = r.findREST(req.Method, req.URL.Path)
	}
	r.record(c)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if ex == nil {
		w.WriteHeader(http.StatusInternalServerError)
		message := fmt.Sprintf("no fixture for %s %s %s %v", req.Method, req.URL.Path, c.Operation, c.Variables)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": message})
		return
	}
	serveExchange(w, ex)
}

// serveExchange writes the recorded response; a text body is served
// verbatim under the fixture headers.
func serveExchange(w http.ResponseWriter, ex *exchange) {
	for k, v := range ex.Response.Headers {
		w.Header().Set(k, v)
	}
	status := ex.Response.Status
	if status == 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)
	if ex.Response.Text != "" {
		_, _ = io.WriteString(w, ex.Response.Text)
		return
	}
	_, _ = w.Write(ex.Response.Body)
}

func operationOf(query string) string {
	if m := operationPattern.FindStringSubmatch(query); m != nil {
		return m[1]
	}
	return ""
}

func (r *replay) findGraphQL(op string, vars map[string]any) *exchange {
	for i := range r.exchanges {
		ex := &r.exchanges[i]
		if ex.Request.Method == "" && ex.Request.Operation == op && matchVars(ex.Request.Match, vars) {
			return ex
		}
	}
	return nil
}

func (r *replay) findREST(method, path string) *exchange {
	for i := range r.exchanges {
		ex := &r.exchanges[i]
		if ex.Request.Method == method && ex.Request.Path == path {
			return ex
		}
	}
	return nil
}

// matchVars compares every recorded variable with the one sent; a missing
// variable counts as null, which is how GitHub reads it.
func matchVars(match, vars map[string]any) bool {
	for k, want := range match {
		if !jsonEqual(want, vars[k]) {
			return false
		}
	}
	return true
}

func jsonEqual(a, b any) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return bytes.Equal(x, y)
}

func (r *replay) record(c call) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, c)
}

// count returns how many requests reached the server.
func (r *replay) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.calls)
}

// last returns the most recent call.
func (r *replay) last() call {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.calls) == 0 {
		r.t.Fatal("no call recorded")
	}
	return r.calls[len(r.calls)-1]
}

// calls returns a copy of every call in order.
func (r *replay) all() []call {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]call(nil), r.calls...)
}
