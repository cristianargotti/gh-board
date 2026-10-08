package guard_test

import (
	"testing"

	"github.com/cristianargotti/gh-board/internal/guard"
)

var fuzzSeeds = []string{
	"gh project delete 46 --owner acme",
	`echo "$(gh project delete 46)" | tee out; (true) && timeout 5 gh project list`,
	"gh api graphql -f query=@- <<'EOF'\nmutation { deleteProjectV2 }\nEOF\n",
	"cat <<-EOF $(id) `date`\n\tbody\n\tEOF",
	`gh 'pro\'ject' "de\"lete" \` + "\n" + "x; y || z & w |& v",
	"$((1+2)) ${x:-$(gh project delete 1)} <<< here",
	"((((((((((((((((((((((((((((((((((((((((((((((((((((((((((((((((((((",
	"\"\"''``$()\\",
	`"C:\Program Files\GitHub CLI\GH.EXE" api -X=delete repos/x/y`,
	`C:\Program Files\PowerShell\7\pwsh.exe -NoProfile -Command 'gh project delete 1'`,
	`CMD.EXE /D /C "powershell.exe -NoProfile -Command 'GH.EXE project delete 1'"`,
}

func FuzzTokenize(f *testing.F) {
	for _, seed := range fuzzSeeds {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, command string) {
		segs := guard.Tokenize(command)
		if len(segs) > len(command)+1 {
			t.Fatalf("%d segments from %d bytes", len(segs), len(command))
		}
		checkSegments(t, segs)
		if _, _, err := guard.Evaluate(command, true); err != nil {
			t.Fatal(err)
		}
	})
}

// checkSegments verifies the invariants every segment keeps: no empty
// segment, no empty word, and a normalization that never panics.
func checkSegments(t *testing.T, segs []guard.Segment) {
	t.Helper()
	for _, seg := range segs {
		if len(seg) == 0 {
			t.Fatalf("empty segment in %q", segs)
		}
		for _, w := range seg {
			if w == "" {
				t.Fatalf("empty word in %q", segs)
			}
		}
		_ = guard.Normalize(seg)
	}
}
