package guard

import (
	"regexp"
	"strings"
)

var methodValue = regexp.MustCompile(`(?:-X[ =]?|--method[ =])([A-Z]+)`)

// The policy stays in patterns.json. Expansion only adapts its spelling
// to rule engines without case folding or executable suffix handling.
func expandedRules(rules []string) []string {
	var out []string
	for _, rule := range rules {
		for _, method := range methodVariants(rule) {
			out = append(out, method)
			if exe := executableVariant(method); exe != method {
				out = append(out, exe)
			}
		}
	}
	return out
}

func executableVariant(rule string) string {
	words := strings.Split(rule, " ")
	for i, word := range words {
		base := programBase(word)
		if base == "gh" || base == boardProgram || (i == 0 && shellPrograms[base]) {
			words[i] += ".exe"
		}
	}
	return strings.Join(words, " ")
}

func methodVariants(rule string) []string {
	loc := methodValue.FindStringSubmatchIndex(rule)
	if loc == nil {
		return []string{rule}
	}
	start, end := loc[2], loc[3]
	variants := []string{rule}
	for i := start; i < end; i++ {
		for _, variant := range append([]string(nil), variants...) {
			variants = append(variants, variant[:i]+strings.ToLower(variant[i:i+1])+variant[i+1:])
		}
	}
	return variants
}
