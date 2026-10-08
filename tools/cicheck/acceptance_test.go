package cicheck_test

import (
	"strings"
	"testing"
)

func TestAcceptanceJob(t *testing.T) {
	job := lookup(t, readYAML(t, "../../.github/workflows/ci.yml"), "jobs.acceptance")
	if got := scalarList(lookup(t, job, "strategy.matrix.os")); got != "ubuntu-latest,macos-latest,windows-latest" {
		t.Fatalf("acceptance matrix: %s", got)
	}
	step := stepNamed(t, lookup(t, job, "steps"), "Run agent acceptance")
	if got := lookup(t, step, "run").Value; got != "go run ./tools/acceptance" {
		t.Fatalf("acceptance command: %s", got)
	}
	var fields map[string]any
	if err := job.Decode(&fields); err != nil {
		t.Fatal(err)
	}
	if _, found := fields["defaults"]; found {
		t.Fatal("acceptance must use the native runner shell")
	}
	if !strings.Contains(readText(t, "../../Makefile"), "acceptance:\n\tgo run ./tools/acceptance\n") {
		t.Fatal("make acceptance must execute the same Go runner")
	}
}
