// Package github is the adapter on go-gh: embedded GraphQL documents, the
// closed mutation list of section 6.2, pagination, the schema cache and
// identity resolution. Every call uses the developer's own gh token through
// go-gh and the adapter never logs or returns it, only its source. Tests
// replay recorded responses over httptest and never reach the network.
package github

import (
	"net/http"
	"sync"
	"time"

	"github.com/cli/go-gh/v2/pkg/api"

	"github.com/cristianargotti/gh-board/internal/domain"
)

// Options configures the adapter. The token is never passed in: go-gh
// resolves it from the developer's gh authentication (principle 1).
type Options struct {
	// Host is the GitHub host; empty means the default host of gh.
	Host string
	// CacheDir holds the schema cache (config.Paths().Cache); empty
	// disables the cache.
	CacheDir string
	// CacheTTL is the schema cache lifetime; zero means one hour.
	CacheTTL time.Duration
	// Transport replaces the HTTP transport. Tests use it to replay
	// recorded responses; nil keeps the default transport of go-gh.
	Transport http.RoundTripper
	// Clock dates discoveries and decides the cache age; nil means the
	// system clock.
	Clock domain.Clock
	// ReleaseRepository is the owner/name that publishes the kit releases,
	// derived from the module path by main; doctor compares the installed
	// binary with the asset digest of its release. Empty disables the check.
	ReleaseRepository string
}

// DefaultCacheTTL is the schema cache lifetime of section 5.1.
const DefaultCacheTTL = time.Hour

// Adapter implements domain.ProjectReader and domain.ProjectWriter on the
// go-gh GraphQL and REST clients. Clients are created lazily on first use
// so that constructing the adapter needs no authentication.
type Adapter struct {
	opts  Options
	clock domain.Clock

	mu   sync.Mutex
	conn *connection
}

// connection holds the clients of one host and what the adapter learned
// about the token: its source, never its value.
type connection struct {
	host        string
	tokenSource string
	gql         *api.GraphQLClient
	rest        *api.RESTClient
}

// New returns an adapter with the options filled with defaults.
func New(opts Options) *Adapter {
	if opts.CacheTTL <= 0 {
		opts.CacheTTL = DefaultCacheTTL
	}
	clock := opts.Clock
	if clock == nil {
		clock = SystemClock{}
	}
	return &Adapter{opts: opts, clock: clock}
}

// Options returns the effective options.
func (a *Adapter) Options() Options { return a.opts }

// SystemClock is the domain.Clock backed by time.Now.
type SystemClock struct{}

// Now returns the current instant in UTC at second precision, the one
// form every stamp of the kit carries.
func (SystemClock) Now() time.Time { return time.Now().UTC().Truncate(time.Second) }

// Compile-time checks that the adapter satisfies the ports.
var (
	_ domain.ProjectReader = (*Adapter)(nil)
	_ domain.ProjectWriter = (*Adapter)(nil)
	_ domain.Clock         = SystemClock{}
)
