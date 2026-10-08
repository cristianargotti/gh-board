package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"testing"
)

func TestRunCommand(t *testing.T) {
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	r := runCommand(command{
		program: program, args: []string{"-test.run=^TestHelperProcess$"},
		env: append(os.Environ(), "ACCEPTANCE_HELPER=1"), dir: t.TempDir(), input: "payload",
	})
	if r.err != nil || r.code != 2 || r.stdout != "payload" || r.stderr != "denied" {
		t.Fatalf("%+v", r)
	}
	r = runCommand(command{program: program, args: []string{"-test.run=^$"}})
	if err := success(r); err != nil {
		t.Fatal(err)
	}
	r = runCommand(command{program: "missing-gh-board-acceptance-executable"})
	if r.err == nil || success(r) == nil {
		t.Fatal("missing executable succeeded")
	}
	if err := success(result{code: 3, stderr: "policy refused"}); err == nil {
		t.Fatal("nonzero exit succeeded")
	}
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("ACCEPTANCE_HELPER") != "1" {
		return
	}
	if _, err := io.Copy(os.Stdout, os.Stdin); err != nil {
		t.Fatal(err)
	}
	_, _ = fmt.Fprint(os.Stderr, "denied")
	os.Exit(2)
}

type brokenWriter struct{}

func (brokenWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

func TestReportFailure(t *testing.T) {
	if code := report(brokenWriter{}, []row{{"check", pass, "fine"}}); code != 1 {
		t.Fatal(code)
	}
}
