package main

import (
	"bufio"
	"errors"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var generatedPattern = regexp.MustCompile(`(?m)^// Code generated .* DO NOT EDIT\.$`)

// skippedDirs are not Go packages of the module.
var skippedDirs = map[string]bool{
	".git": true, "vendor": true, "node_modules": true, "testdata": true, "dist": true,
}

// modulePath reads the module line of go.mod under root.
func modulePath(root string) (string, error) {
	f, err := os.Open(filepath.Join(root, "go.mod")) //nolint:gosec // root is a flag of this tool
	if err != nil {
		return "", err
	}
	defer f.Close() //nolint:errcheck // read-only handle
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(scanner.Text()), "module "); ok {
			return strings.TrimSpace(rest), nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", errors.New("go.mod: no module line")
}

// sourceFiles lists, in profile naming (<module>/<relative path>), every
// non-test Go file that declares at least one function with statements.
func sourceFiles(root, module string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return skipDir(root, path, d)
		}
		name, ok, err := sourceName(root, module, path)
		if err != nil || !ok {
			return err
		}
		out = append(out, name)
		return nil
	})
	return out, err
}

func skipDir(root, path string, d fs.DirEntry) error {
	if path != root && skippedDirs[d.Name()] {
		return filepath.SkipDir
	}
	return nil
}

// sourceName returns the profile name of a Go source file that needs
// coverage, or false for tests, generated files, files without statements
// and files the build constraints leave out on this operating system (a
// file compiled only on Windows is covered by the Windows runner).
func sourceName(root, module, path string) (string, bool, error) {
	if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
		return "", false, nil
	}
	if built, err := build.Default.MatchFile(filepath.Dir(path), filepath.Base(path)); err != nil || !built {
		return "", false, err
	}
	needs, err := needsCoverage(path)
	if err != nil || !needs {
		return "", false, err
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return "", false, err
	}
	return module + "/" + filepath.ToSlash(rel), true, nil
}

// needsCoverage reports whether the file has executable statements and is
// not generated.
func needsCoverage(path string) (bool, error) {
	data, err := os.ReadFile(path) //nolint:gosec // the tool reads the module it checks
	if err != nil {
		return false, err
	}
	if generatedPattern.Match(data) {
		return false, nil
	}
	file, err := parser.ParseFile(token.NewFileSet(), path, data, parser.SkipObjectResolution)
	if err != nil {
		return false, err
	}
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Body != nil && len(fn.Body.List) > 0 {
			return true, nil
		}
	}
	return false, nil
}
