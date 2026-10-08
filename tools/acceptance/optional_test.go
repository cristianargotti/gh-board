package main

import (
	"bytes"
	"errors"
	"os/exec"
	"strings"
	"testing"
)

func TestMissingOptionalCLIs(t *testing.T) {
	var out bytes.Buffer
	code := run(&out, simulate, func(name string) (string, error) {
		if name == codexAgent || name == claudeAgent {
			return "", exec.ErrNotFound
		}
		return name, nil
	})
	if code != 0 || strings.Count(out.String(), skipped) != 2 || strings.Contains(out.String(), fail) {
		t.Fatalf("code=%d: %s", code, out.String())
	}
}

func TestOptionalLookupFailure(t *testing.T) {
	s := suite{lookup: func(string) (string, error) { return "", errors.New("access denied") }}
	s.checkOptional()
	for _, r := range s.rows {
		if r.status != fail || r.detail != "access denied" {
			t.Fatal(r)
		}
	}
}

var policyCases = []result{
	{code: 1, stderr: "invalid rules"},
	{stdout: "invalid JSON"},
	{stdout: `{"decision":"allow"}`},
	{stdout: `{}`},
}

func TestPolicyFailures(t *testing.T) {
	for _, reply := range policyCases {
		s := suite{execute: func(command) result { return reply }}
		if err := s.checkPolicy(codexAgent); err == nil {
			t.Fatalf("accepted %+v", reply)
		}
	}
}
