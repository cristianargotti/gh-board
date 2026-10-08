package main

import (
	"fmt"
	"os"
	"strings"
)

// forbiddenRunes are the dashes the standards ban in every text file,
// written as code points so that this file passes its own check.
var forbiddenRunes = []struct {
	r    rune
	name string
}{
	{0x2014, "em dash"},
	{0x2013, "en dash"},
}

// checkDashes reports every line of every text file holding a dash.
func checkDashes(root string) ([]string, error) {
	var out []string
	err := walkFiles(root, func(path, rel string) error {
		data, err := os.ReadFile(path) //nolint:gosec // the tool reads the repository it checks
		if err != nil {
			return err
		}
		if isBinary(data) {
			return nil
		}
		out = append(out, dashViolations(rel, data)...)
		return nil
	})
	return out, err
}

// dashViolations lists the lines of one file that hold a forbidden rune.
func dashViolations(rel string, data []byte) []string {
	var out []string
	for i, line := range strings.Split(string(data), "\n") {
		for _, f := range forbiddenRunes {
			if strings.ContainsRune(line, f.r) {
				out = append(out, fmt.Sprintf("%s:%d: %s", rel, i+1, f.name))
			}
		}
	}
	return out
}
