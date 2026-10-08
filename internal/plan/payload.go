package plan

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/cristianargotti/gh-board/internal/domain"
)

// RefPrefix starts a reference to the node an earlier step created:
// "step:0" in a target or a payload id names what step 0 produced.
const RefPrefix = "step:"

// Ref writes the reference to the node created by the step at index.
func Ref(index int) string {
	return RefPrefix + strconv.Itoa(index)
}

// isRef reports whether the id is a step reference.
func isRef(id string) bool {
	return strings.HasPrefix(id, RefPrefix)
}

// parseRef returns the step index a reference names.
func parseRef(id string) (int, bool) {
	if !isRef(id) {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimPrefix(id, RefPrefix))
	return n, err == nil && n >= 0
}

// EncodePayload encodes the domain input a creation step carries in
// After, the form decodePayload reads back at apply.
func EncodePayload(in any) (string, error) {
	data, err := json.Marshal(in)
	if err != nil {
		return "", fmt.Errorf("encode payload: %w", err)
	}
	return string(data), nil
}

// decodePayload reads the domain input a creation step carries in After.
func decodePayload(step domain.Step, out any) error {
	if err := json.Unmarshal([]byte(step.After), out); err != nil {
		return fmt.Errorf("step %d: %s payload is not valid JSON: %w", step.Index, step.Operation, domain.ErrUsage)
	}
	return nil
}
