package github_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The mutation allowlist of section 6.2. createMilestone is the REST POST.
var allowedMutations = []string{
	"createIssue", "updateIssue", "addAssigneesToAssignable", "removeAssigneesFromAssignable",
	"addLabelsToLabelable", "removeLabelsFromLabelable", "addComment", "closeIssue", "reopenIssue",
	"addSubIssue", "updateIssueIssueType", "setIssueFieldValue", "updateIssueFieldValue",
	"addProjectV2ItemById", "updateProjectV2ItemFieldValue", "unarchiveProjectV2Item",
	"createProjectV2StatusUpdate", "createProjectV2Field", "createLabel", "copyProjectV2",
	"linkProjectV2ToRepository",
}

// The mutations section 6.2 names as absent from the binary.
var forbiddenMutations = []string{
	"deleteProjectV2", "deleteProjectV2Item", "deleteProjectV2Field", "deleteProjectV2View",
	"deleteProjectV2Workflow", "deleteProjectV2StatusUpdate", "archiveProjectV2Item",
	"clearProjectV2ItemFieldValue", "updateProjectV2", "updateProjectV2Field",
	"updateProjectV2Collaborators", "unlinkProjectV2FromRepository", "unmarkProjectV2AsTemplate",
	"deleteIssue", "transferIssue", "removeSubIssue", "deleteIssueComment", "updateIssueComment",
	"deleteIssueField", "deleteIssueFieldValue", "deleteIssueType", "deleteLabel", "deleteMilestone",
}

var (
	// mutationRoot captures the root field of a mutation operation, in a
	// document or in a Go string: mutation Name($v: T) { field(input: ...).
	mutationRoot = regexp.MustCompile(`(?s)\bmutation\b\s*\w*\s*(?:\([^)]*\))?\s*\{\s*(\w+)\s*\(`)
	// restDelete matches a REST request built with the DELETE method.
	restDelete = regexp.MustCompile(`(?:Do|DoWithContext|Request|RequestWithContext|NewRequest|NewRequestWithContext)\(.*(?:"DELETE"|http\.MethodDelete)|\.Delete\(`)
)

type source struct {
	path string
	text string
}

// moduleSources reads every Go source that ships in the binary and every
// GraphQL document under the module root. Test files are left out: they do
// not reach the binary and the guard tests hold destructive commands as
// data to prove they are denied.
func moduleSources(t *testing.T) (goFiles, documents []source) {
	t.Helper()
	root := moduleRoot(t)
	for _, path := range sourcePaths(t, root) {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		rel, _ := filepath.Rel(root, path)
		switch filepath.Ext(path) {
		case ".go":
			goFiles = append(goFiles, source{rel, string(data)})
		case ".graphql":
			documents = append(documents, source{rel, string(data)})
		}
	}
	if len(goFiles) == 0 || len(documents) == 0 {
		t.Fatalf("scanned %d go files and %d documents", len(goFiles), len(documents))
	}
	return goFiles, documents
}

// sourcePaths lists the Go sources and GraphQL documents under root, in
// walk order, skipping vendored code and fixtures.
func sourcePaths(t *testing.T, root string) []string {
	t.Helper()
	var paths []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() && (name == ".git" || name == "vendor" || name == "testdata" || name == "node_modules") {
			return filepath.SkipDir
		}
		if d.IsDir() || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		if ext := filepath.Ext(name); ext == ".go" || ext == ".graphql" {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return paths
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found above the package")
		}
		dir = parent
	}
}

func TestMutationRootsAreAllowlisted(t *testing.T) {
	goFiles, documents := moduleSources(t)
	found := 0
	for _, src := range append(documents, goFiles...) {
		for _, m := range mutationRoot.FindAllStringSubmatch(src.text, -1) {
			found++
			if !contains(allowedMutations, m[1]) {
				t.Errorf("%s runs mutation %s, which is outside the allowlist of section 6.2", src.path, m[1])
			}
		}
	}
	if found != len(allowedMutations) {
		t.Fatalf("found %d mutation documents, want one per allowlisted GraphQL mutation (%d)", found, len(allowedMutations))
	}
}

func TestForbiddenMutationsAreAbsent(t *testing.T) {
	goFiles, documents := moduleSources(t)
	for _, name := range forbiddenMutations {
		t.Run(name, func(t *testing.T) {
			word := regexp.MustCompile(`\b` + name + `\b`)
			call := regexp.MustCompile(`\b` + name + `\s*\(`)
			for _, doc := range documents {
				if word.MatchString(doc.text) {
					t.Errorf("%s names %s", doc.path, name)
				}
			}
			for _, src := range goFiles {
				if call.MatchString(src.text) {
					t.Errorf("%s calls %s", src.path, name)
				}
			}
		})
	}
}

func TestNoRESTDelete(t *testing.T) {
	goFiles, _ := moduleSources(t)
	for _, src := range goFiles {
		for i, line := range strings.Split(src.text, "\n") {
			if restDelete.MatchString(line) {
				t.Errorf("%s:%d builds a DELETE request: %s", src.path, i+1, strings.TrimSpace(line))
			}
		}
	}
}

func contains(list []string, value string) bool {
	for _, v := range list {
		if v == value {
			return true
		}
	}
	return false
}
