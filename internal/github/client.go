package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/cli/go-gh/v2/pkg/auth"

	"github.com/cristianargotti/gh-board/internal/domain"
)

// connect creates the clients on first use. go-gh reads the token from the
// developer's gh authentication; the adapter keeps only its source, which
// doctor reports when an environment variable shadows the keyring (6.6).
func (a *Adapter) connect() (*connection, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.conn != nil {
		return a.conn, nil
	}
	host := a.opts.Host
	if host == "" {
		host, _ = auth.DefaultHost()
	}
	token, source := auth.TokenForHost(host)
	opts := api.ClientOptions{Host: host, AuthToken: token, Transport: a.opts.Transport}
	gql, err := api.NewGraphQLClient(opts)
	if err != nil {
		return nil, authError(host, err)
	}
	rest, err := api.NewRESTClient(opts)
	if err != nil {
		return nil, authError(host, err)
	}
	httpClient, err := api.NewHTTPClient(opts)
	if err != nil {
		return nil, authError(host, err)
	}
	a.conn = &connection{host: host, tokenSource: source, gql: gql, rest: rest, http: httpClient}
	return a.conn, nil
}

// authError reports a client that could not be built, which only happens
// when gh holds no token for the host.
func authError(host string, err error) error {
	return &APIError{
		Operation: "connect",
		Auth:      true,
		Message:   fmt.Sprintf("no gh authentication for %s (%v): run gh auth login", host, err),
		kind:      domain.ErrAPI,
	}
}

// run executes the embedded document with the variables and decodes the
// data into out. go-gh decodes a partial response before it returns the
// error, so a caller may tolerate a path scoped NOT_FOUND.
func (a *Adapter) run(ctx context.Context, name string, vars map[string]any, out any) error {
	conn, err := a.connect()
	if err != nil {
		return err
	}
	doc, err := document(name)
	if err != nil {
		return err
	}
	if err := conn.gql.DoWithContext(ctx, doc, vars, out); err != nil {
		return wrapError(name, err)
	}
	return nil
}

// post sends a REST request with a JSON body and decodes the response.
// It is the only REST verb of the kit (createMilestone).
func (a *Adapter) post(ctx context.Context, path string, body, out any) error {
	conn, err := a.connect()
	if err != nil {
		return err
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("encode %s: %w", path, err)
	}
	if err := conn.rest.DoWithContext(ctx, http.MethodPost, path, bytes.NewReader(payload), out); err != nil {
		return wrapError(path, err)
	}
	return nil
}
