package github

import (
	"embed"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"strings"
	"sync"
)

// queries holds every GraphQL document of the adapter. There is no other
// GraphQL text in the kit, which the allowlist test of section 6.2 relies
// on: it reads these files and every Go source.
//
//go:embed queries/*.graphql
var queries embed.FS

const (
	queriesDir    = "queries"
	fragmentsFile = "fragments.graphql"
	documentExt   = ".graphql"
)

var (
	fragmentPattern = regexp.MustCompile(`(?m)^fragment\s+(\w+)\s+on\s+\w+\s*\{`)
	spreadPattern   = regexp.MustCompile(`\.\.\.(\w+)`)

	documentsOnce sync.Once
	documents     map[string]string
	documentsErr  error
)

// document returns the operation stored under queries/<name>.graphql with
// the fragments it spreads, transitively, because GitHub rejects a document
// that defines a fragment it does not use.
func document(name string) (string, error) {
	documentsOnce.Do(loadDocuments)
	if documentsErr != nil {
		return "", documentsErr
	}
	doc, ok := documents[name]
	if !ok {
		return "", fmt.Errorf("graphql document %q is not embedded", name)
	}
	return doc, nil
}

func loadDocuments() {
	raw, err := fs.ReadFile(queries, path.Join(queriesDir, fragmentsFile))
	if err != nil {
		documentsErr = err
		return
	}
	fragments := splitFragments(string(raw))
	entries, err := fs.ReadDir(queries, queriesDir)
	if err != nil {
		documentsErr = err
		return
	}
	documents = make(map[string]string, len(entries))
	for _, entry := range entries {
		if entry.Name() == fragmentsFile || !strings.HasSuffix(entry.Name(), documentExt) {
			continue
		}
		body, err := fs.ReadFile(queries, path.Join(queriesDir, entry.Name()))
		if err != nil {
			documentsErr = err
			return
		}
		documents[strings.TrimSuffix(entry.Name(), documentExt)] = compose(string(body), fragments)
	}
}

// splitFragments cuts fragments.graphql into one text per fragment name by
// matching braces.
func splitFragments(src string) map[string]string {
	out := map[string]string{}
	for _, m := range fragmentPattern.FindAllStringSubmatchIndex(src, -1) {
		start, open := m[0], m[1]-1
		depth := 0
		for i := open; i < len(src); i++ {
			switch src[i] {
			case '{':
				depth++
			case '}':
				depth--
			}
			if depth == 0 {
				out[src[m[2]:m[3]]] = src[start : i+1]
				break
			}
		}
	}
	return out
}

// compose appends to the operation the fragments it spreads and the
// fragments those spread, each once, in first use order.
func compose(body string, fragments map[string]string) string {
	var out strings.Builder
	out.WriteString(strings.TrimSpace(body))
	seen := map[string]bool{}
	queue := spreadNames(body)
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		text, ok := fragments[name]
		if seen[name] || !ok {
			continue
		}
		seen[name] = true
		out.WriteString("\n\n")
		out.WriteString(text)
		queue = append(queue, spreadNames(text)...)
	}
	return out.String()
}

// spreadNames lists the names after "..." in the text; "on" from inline
// fragments comes out too and never matches a named fragment.
func spreadNames(text string) []string {
	var names []string
	for _, m := range spreadPattern.FindAllStringSubmatch(text, -1) {
		names = append(names, m[1])
	}
	return names
}
