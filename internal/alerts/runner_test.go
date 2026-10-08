package alerts_test

import (
	"context"
	"runtime"
	"strings"
	"testing"

	"github.com/cristianargotti/gh-board/internal/alerts"
)

var commandStringCases = []struct {
	cmd  alerts.Command
	want string
}{
	{cmd: alerts.Command{Name: "launchctl", Args: []string{"bootout", "gui/501/x"}}, want: "launchctl bootout gui/501/x"},
	{cmd: alerts.Command{Name: "a", Args: []string{"b c", "", "d'e", "f\tg", "h"}}, want: `a "b c" "" "d'e" "f\tg" h`},
	{cmd: alerts.Command{Name: "crontab", Args: []string{"-"}, Stdin: []byte("x")}, want: "crontab -"},
}

func TestCommandString(t *testing.T) {
	for _, tc := range commandStringCases {
		if got := tc.cmd.String(); got != tc.want {
			t.Errorf("String() = %q, want %q", got, tc.want)
		}
	}
}

// TestExecRunner runs the go tool, which is on the PATH wherever go test
// runs: a harmless program that proves the runner launches and reports.
func TestExecRunner(t *testing.T) {
	ctx := context.Background()
	r := alerts.ExecRunner{}
	goTool, err := r.LookPath("go")
	if err != nil {
		t.Skipf("go is not on the PATH: %v", err)
	}
	out, err := r.Run(ctx, alerts.Command{Name: goTool, Args: []string{"env", "GOOS"}})
	if err != nil || strings.TrimSpace(string(out)) != runtime.GOOS {
		t.Fatalf("go env GOOS = %q, %v", out, err)
	}
	_, err = r.Run(ctx, alerts.Command{Name: goTool, Args: []string{"help", "no-such-topic"}, Stdin: []byte("ignored")})
	if err == nil || !strings.Contains(err.Error(), "no-such-topic") {
		t.Fatalf("a failing program must report its output, got %v", err)
	}
	if _, err := r.LookPath("gh-board-no-such-program"); err == nil {
		t.Fatal("a missing program must not resolve")
	}
	if _, err := r.Run(ctx, alerts.Command{Name: "gh-board-no-such-program"}); err == nil {
		t.Fatal("a missing program must not run")
	}
}
