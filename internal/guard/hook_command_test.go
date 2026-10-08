package guard

import (
	"encoding/base64"
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"unicode/utf16"
)

var hookCommandCases = []struct {
	goos, bin string
	strict    bool
}{
	{"windows", `C:\Users\me\AppData\Local\GitHub CLI\gh-board.exe`, false},
	{"windows", `C:\Users\D'Ávila\GitHub CLI\gh-board.exe`, true},
	{"linux", "/opt/GitHub CLI/gh-board", false},
	{"darwin", "/opt/GitHub CLI/gh-board", true},
}

func TestHookCommandPlatforms(t *testing.T) {
	for _, c := range hookCommandCases {
		got := hookCommand(c.bin, AgentClaude, c.strict, c.goos)
		if c.goos == "windows" {
			checkEncodedHook(t, got, c.bin, c.strict)
			continue
		}
		want := `"` + c.bin + `" guard check --agent claude`
		if c.strict {
			want += " --strict"
		}
		if got != want {
			t.Errorf("%s: %q, want %q", c.goos, got, want)
		}
	}
}

func checkEncodedHook(t *testing.T, command, bin string, strict bool) {
	t.Helper()
	words := strings.Fields(command)
	if len(words) != 9 || words[0] != "powershell.exe" || words[7] != "-EncodedCommand" {
		t.Fatalf("Windows command: %q", command)
	}
	data, err := base64.StdEncoding.DecodeString(words[8])
	if err != nil || len(data)%2 != 0 {
		t.Fatalf("encoded command: %v", err)
	}
	units := make([]uint16, len(data)/2)
	for i := range units {
		units[i] = binary.LittleEndian.Uint16(data[i*2:])
	}
	script := string(utf16.Decode(units))
	for _, needle := range []string{"[Console]::In.ReadToEnd() | & '" + strings.ReplaceAll(bin, "'", "''") + "' guard check --agent claude", "exit $LASTEXITCODE", "exit 2", "UTF8Encoding"} {
		if !strings.Contains(script, needle) {
			t.Errorf("decoded script lacks %q: %s", needle, script)
		}
	}
	if strings.Contains(script, " --strict") != strict {
		t.Errorf("strict=%v in %s", strict, script)
	}
}

func TestHookPOSIXExecution(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell execution is tested on Unix")
	}
	bin := filepath.Join(t.TempDir(), "GitHub CLI $literal `literal` 'quote' gh-board")
	writeExecutable(t, bin, "#!/bin/sh\nprintf '%s\\n' \"$@\"\ncat\nexit 2\n")
	command := hookCommand(bin, AgentClaude, true, "linux")
	for _, shell := range []string{"sh", "bash"} {
		run := exec.Command(shell, "-c", command)
		run.Stdin = strings.NewReader(`{"command":"á"}`)
		got, err := run.CombinedOutput()
		if err == nil || run.ProcessState.ExitCode() != 2 || string(got) != "guard\ncheck\n--agent\nclaude\n--strict\n"+`{"command":"á"}` {
			t.Fatalf("%s output=%s err=%v", shell, got, err)
		}
	}
}

func TestWindowsCommandShellParsing(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX argument roundtrip is tested on Unix")
	}
	stub := filepath.Join(t.TempDir(), "powershell.exe")
	writeExecutable(t, stub, "#!/bin/sh\nprintf '%s\\n' \"$@\"\n")
	command := hookCommand(`C:\GitHub CLI\gh-board.exe`, AgentClaude, false, "windows")
	for _, shell := range []string{"sh", "bash"} {
		args := strings.TrimPrefix(command, "powershell.exe ")
		run := exec.Command(shell, "-c", quoteWord(stub)+" "+args)
		got, err := run.CombinedOutput()
		if err != nil || strings.TrimSpace(string(got)) != strings.Join(strings.Fields(args), "\n") {
			t.Fatalf("%s Windows argv=%s err=%v", shell, got, err)
		}
	}
}

func writeExecutable(t *testing.T, path, script string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil { //nolint:gosec // the shell must execute this temporary test fixture
		t.Fatal(err)
	}
}

func TestHookBinaryRoundTrip(t *testing.T) {
	for _, c := range hookCommandCases {
		got, ok := HookBinary(hookCommand(c.bin, AgentCodex, c.strict, c.goos))
		if !ok || got != c.bin {
			t.Errorf("%s %q: HookBinary = %q, %v", c.goos, c.bin, got, ok)
		}
	}
	legacy := `"/usr/local/bin/gh-board" guard check --agent claude`
	if got, ok := HookBinary(legacy); !ok || got != "/usr/local/bin/gh-board" {
		t.Errorf("legacy form: %q, %v", got, ok)
	}
	for _, bad := range []string{"", "gh board guard check --agent claude", `"`, `"\`, "powershell.exe -EncodedCommand ###", "powershell.exe -EncodedCommand " + encodeUTF16("echo hi"), "powershell.exe -EncodedCommand " + encodeUTF16("| & 'unterminated")} {
		if got, ok := HookBinary(bad); ok {
			t.Errorf("%q: HookBinary = %q, want none", bad, got)
		}
	}
}

func encodeUTF16(script string) string {
	units := utf16.Encode([]rune(script))
	data := make([]byte, len(units)*2)
	for i, unit := range units {
		binary.LittleEndian.PutUint16(data[i*2:], unit)
	}
	return base64.StdEncoding.EncodeToString(data)
}
