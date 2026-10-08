package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"regexp"
	"strings"
)

// generatedPattern is the Go convention for generated files.
var generatedPattern = regexp.MustCompile(`(?m)^// Code generated .* DO NOT EDIT\.$`)

// checkGoFiles reports Go files over the line limit and functions over the
// function limit, tests included and generated files excluded.
func checkGoFiles(root string) ([]string, error) {
	var out []string
	err := walkFiles(root, func(path, rel string) error {
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		data, err := os.ReadFile(path) //nolint:gosec // the tool reads the repository it checks
		if err != nil {
			return err
		}
		if isGenerated(data) {
			return nil
		}
		out = append(out, lengthViolations(rel, data)...)
		return nil
	})
	return out, err
}

func isGenerated(data []byte) bool { return generatedPattern.Match(data) }

// countLines counts the lines of a file, the last one with or without a
// trailing newline.
func countLines(data []byte) int {
	if len(data) == 0 {
		return 0
	}
	n := bytes.Count(data, []byte{'\n'})
	if data[len(data)-1] != '\n' {
		n++
	}
	return n
}

// lengthViolations checks one Go file against both limits.
func lengthViolations(rel string, data []byte) []string {
	var out []string
	if n := countLines(data); n > maxFileLines {
		out = append(out, fmt.Sprintf("%s: %d lines, maximum %d", rel, n, maxFileLines))
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, rel, data, parser.SkipObjectResolution)
	if err != nil {
		return append(out, fmt.Sprintf("%s: parse error: %v", rel, err))
	}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		start := fset.Position(fn.Pos()).Line
		lines := fset.Position(fn.End()).Line - start + 1
		if lines > maxFuncLines {
			out = append(out, fmt.Sprintf("%s:%d: function %s is %d lines, maximum %d",
				rel, start, fn.Name.Name, lines, maxFuncLines))
		}
	}
	return out
}
