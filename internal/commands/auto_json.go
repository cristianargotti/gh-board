package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"

	"github.com/cristianargotti/gh-board/internal/domain"
)

// autoJSONObject decodes a JSON object. Empty input is an empty object, so
// a file the installer creates starts from nothing; anything that is not an
// object is refused rather than overwritten.
func autoJSONObject(data []byte, path string) (map[string]any, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return map[string]any{}, nil
	}
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, fmt.Errorf("%s is not a JSON object: %v: %w", path, err, domain.ErrUsage)
	}
	if obj == nil {
		return nil, fmt.Errorf("%s is not a JSON object: %w", path, domain.ErrUsage)
	}
	return obj, nil
}

// autoJSONEncode writes an object the way the agents' own editors do: two
// space indentation, no HTML escaping and a final newline.
func autoJSONEncode(obj map[string]any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(obj); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// autoFragment turns any value into decoded JSON, so that fragments built
// from Go values compare by deep equality with what the files hold.
func autoFragment(v any) (map[string]any, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return autoJSONObject(data, "fragment")
}

// autoJSONDelta returns the part of fragment that base lacks: absent keys,
// array entries absent by deep equality and objects recursively. nil means
// base already holds the fragment. A key present with another type is a
// conflict, left alone and reported by autoJSONConflicts.
func autoJSONDelta(base, fragment map[string]any) map[string]any {
	delta := map[string]any{}
	for _, key := range autoSortedKeys(fragment) {
		if missing, ok := autoDeltaOf(base, key, fragment[key]); ok {
			delta[key] = missing
		}
	}
	if len(delta) == 0 {
		return nil
	}
	return delta
}

// autoDeltaOf returns what base lacks of one fragment value.
func autoDeltaOf(base map[string]any, key string, want any) (any, bool) {
	have, present := base[key]
	if !present {
		return autoJSONClone(want), true
	}
	switch w := want.(type) {
	case map[string]any:
		if h, ok := have.(map[string]any); ok {
			if sub := autoJSONDelta(h, w); sub != nil {
				return sub, true
			}
		}
	case []any:
		if h, ok := have.([]any); ok {
			if missing := autoMissingEntries(h, w); len(missing) > 0 {
				return missing, true
			}
		}
	}
	return nil, false
}

// autoJSONConflicts lists the keys whose value in base differs from the
// fragment and cannot be merged: scalars with another value and keys of
// another type. The installer leaves them as they are and says so.
func autoJSONConflicts(base, fragment map[string]any, prefix string) []string {
	var out []string
	for _, key := range autoSortedKeys(fragment) {
		if have, present := base[key]; present {
			out = append(out, autoConflictsOf(have, fragment[key], prefix+key)...)
		}
	}
	return out
}

// autoConflictsOf compares one present value with the fragment's.
func autoConflictsOf(have, want any, path string) []string {
	switch w := want.(type) {
	case map[string]any:
		if h, ok := have.(map[string]any); ok {
			return autoJSONConflicts(h, w, path+".")
		}
	case []any:
		if _, ok := have.([]any); ok {
			return nil
		}
	default:
		if reflect.DeepEqual(have, want) {
			return nil
		}
	}
	return []string{path}
}

// autoJSONMerge applies a delta in place: objects recurse, array entries
// append, anything else is set. It returns base for chaining.
func autoJSONMerge(base, delta map[string]any) map[string]any {
	for key, want := range delta {
		have, present := base[key]
		switch w := want.(type) {
		case map[string]any:
			if h, ok := have.(map[string]any); ok && present {
				autoJSONMerge(h, w)
				continue
			}
		case []any:
			if h, ok := have.([]any); ok && present {
				base[key] = append(h, autoMissingEntries(h, w)...)
				continue
			}
		}
		base[key] = autoJSONClone(want)
	}
	return base
}

// autoJSONUnmerge removes a delta in place: array entries by deep equality,
// objects recursively, anything else when it still equals what the
// installer wrote. A container left empty goes away with its key, which
// every agent reads the same as an absent key.
func autoJSONUnmerge(base, delta map[string]any) {
	for key, want := range delta {
		have, present := base[key]
		if !present {
			continue
		}
		if kept, keep := autoUnmergeOf(have, want); keep {
			base[key] = kept
		} else {
			delete(base, key)
		}
	}
}

// autoUnmergeOf removes a delta value from a present value and reports
// whether anything remains.
func autoUnmergeOf(have, want any) (any, bool) {
	switch w := want.(type) {
	case map[string]any:
		h, ok := have.(map[string]any)
		if !ok {
			return have, true
		}
		autoJSONUnmerge(h, w)
		return h, len(h) > 0
	case []any:
		h, ok := have.([]any)
		if !ok {
			return have, true
		}
		kept := autoWithoutEntries(h, w)
		return kept, len(kept) > 0
	}
	return have, !reflect.DeepEqual(have, want)
}

// autoMissingEntries returns the entries of want absent from have, without
// duplicates, in the order of want.
func autoMissingEntries(have, want []any) []any {
	var missing []any
	for _, w := range want {
		if !autoContainsEntry(have, w) && !autoContainsEntry(missing, w) {
			missing = append(missing, autoJSONClone(w))
		}
	}
	return missing
}

// autoWithoutEntries returns have without the entries of drop.
func autoWithoutEntries(have, drop []any) []any {
	kept := make([]any, 0, len(have))
	for _, h := range have {
		if !autoContainsEntry(drop, h) {
			kept = append(kept, h)
		}
	}
	return kept
}

func autoContainsEntry(list []any, entry any) bool {
	for _, e := range list {
		if reflect.DeepEqual(e, entry) {
			return true
		}
	}
	return false
}

// autoJSONClone deep-copies a decoded JSON value so that receipts and
// files never share memory.
func autoJSONClone(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[k] = autoJSONClone(val)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			out[i] = autoJSONClone(val)
		}
		return out
	}
	return v
}

func autoSortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
