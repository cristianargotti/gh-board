package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunHelp(t *testing.T) {
	t.Setenv("GH_BOARD_HOME", t.TempDir())
	var out, errOut bytes.Buffer
	if code := run([]string{"--help"}, &out, &errOut); code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, errOut.String())
	}
	if !strings.Contains(out.String(), "gh board") {
		t.Fatalf("help = %q", out.String())
	}
}

func TestRunUsageError(t *testing.T) {
	t.Setenv("GH_BOARD_HOME", t.TempDir())
	var out, errOut bytes.Buffer
	if code := run([]string{"no-such-command"}, &out, &errOut); code != 1 {
		t.Fatalf("exit code = %d", code)
	}
	if !strings.HasPrefix(errOut.String(), "gh board:") {
		t.Fatalf("stderr = %q", errOut.String())
	}
}
