package render

import (
	"encoding/json"
	"fmt"
	"io"
)

// renderJSON encodes the payload when the command set one, otherwise the
// document itself. One line, HTML characters kept, trailing newline. Map
// keys are sorted by the encoder, so the output is deterministic.
func renderJSON(w io.Writer, doc *Document) error {
	var payload any = doc
	if doc.Data != nil {
		payload = doc.Data
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(payload); err != nil {
		return fmt.Errorf("render json: %w", err)
	}
	return nil
}
