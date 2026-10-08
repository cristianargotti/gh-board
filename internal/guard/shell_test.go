package guard

import "testing"

var shellFlagCases = []struct {
	shell, arg, want string
}{
	{"cmd", "/c", "/c"},
	{"cmd", "/C", "/c"},
	{"cmd", "/d", ""},
	{"powershell", "-Command", "-Command"},
	{"pwsh", "-COMMAND", "-Command"},
	{"powershell", "-c", "-c"},
	{"pwsh", "-NoProfile", ""},
	{"bash", "-lc", "-lc"},
	{"zsh", "-ic", "-c"},
	{"sh", "-C", "-c"},
	{"sh", "--norc", ""},
	{"sh", "script", ""},
}

func TestCommandFlag(t *testing.T) {
	for _, c := range shellFlagCases {
		if got := commandFlag(c.shell, c.arg); got != c.want {
			t.Errorf("%s %s = %s, want %s", c.shell, c.arg, got, c.want)
		}
	}
}

func TestShellWithoutBody(t *testing.T) {
	for _, words := range []Segment{{"sh", "-c"}, {"cmd", "/c"}, {"pwsh", "-c", "-NoProfile"}} {
		_, script, wrapped := shellScript(words)
		if !wrapped || script != "" {
			t.Fatalf("%v: script=%q wrapped=%v", words, script, wrapped)
		}
	}
}

func TestWrapperDepthLimit(t *testing.T) {
	patterns, err := Patterns(true)
	if err != nil {
		t.Fatal(err)
	}
	m, denied := evaluate(`sh -c 'gh project list'`, false, patterns, maxDepth)
	if !denied || m.Rule != "sh -c *" {
		t.Fatalf("depth limit: match=%+v denied=%v", m, denied)
	}
}
