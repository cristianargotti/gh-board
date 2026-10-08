package plan

import (
	"os"
	"testing"
)

func TestIsTerminalRejectsNullDevice(t *testing.T) {
	f, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if IsTerminal(f) {
		t.Fatal("the null character device is not an interactive terminal")
	}
}

func TestIsTerminalRejectsPipeAndClosedFile(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close(); _ = w.Close() }()
	if IsTerminal(r) || IsTerminal(w) {
		t.Fatal("pipes are not interactive terminals")
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	if IsTerminal(r) {
		t.Fatal("a closed file is not an interactive terminal")
	}
}

func TestTerminalDetectorUsesInput(t *testing.T) {
	for _, want := range []bool{false, true} {
		called := false
		detect := func(f *os.File) bool {
			called = true
			if f != os.Stdin {
				t.Fatal("detector did not receive stdin")
			}
			return want
		}
		if isTerminal(nil, detect) || called {
			t.Fatal("nil input must bypass the detector")
		}
		if got := isTerminal(os.Stdin, detect); got != want || !called {
			t.Fatalf("terminal = %v, want %v, called = %v", got, want, called)
		}
	}
}

var cygwinNameCases = []struct {
	name string
	want bool
}{
	{`\msys-1888ae32e00d56aa-pty0-from-master`, true},
	{`\cygwin-1888ae32e00d56aa-pty3-to-master`, true},
	{`\Device\NamedPipe\msys-1888ae32e00d56aa-pty0-from-master`, true},
	{`\msys--pty0-from-master`, false},
	{`\msys-1888-tty0-from-master`, false},
	{`\msys-1888-pty0-sideways-master`, false},
	{`\msys-1888-pty0-from-slave`, false},
	{`\pipe\other-1888-pty0-from-master`, false},
	{`short`, false},
}

func TestCygwinPtyName(t *testing.T) {
	for _, tc := range cygwinNameCases {
		if got := cygwinPtyName(tc.name); got != tc.want {
			t.Errorf("cygwinPtyName(%q) = %v, want %v", tc.name, got, tc.want)
		}
	}
	if isCygwinTerminal(nil) {
		t.Fatal("nil is not a terminal")
	}
}
