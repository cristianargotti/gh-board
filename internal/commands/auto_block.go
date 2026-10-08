package commands

import "bytes"

// Markers of the block the kit keeps in AGENTS.md (section 8.3). Everything
// between them is the kit's; everything else is the team's.
const (
	autoBlockBegin = "<!-- gh-board:begin -->"
	autoBlockEnd   = "<!-- gh-board:end -->"
)

// autoMarkers delimit the kit's block in a file somebody else owns; the
// comment syntax follows the file.
type autoMarkers struct {
	begin, end string
}

// Marker sets: HTML comments for Markdown files, hash comments for TOML.
var (
	autoHTMLMarkers = autoMarkers{begin: autoBlockBegin, end: autoBlockEnd}
	autoTOMLMarkers = autoMarkers{begin: "# gh-board:begin", end: "# gh-board:end"}
)

// autoBlock wraps a body in the Markdown markers, one per line.
func autoBlock(body []byte) []byte {
	return autoBlockWith(body, autoHTMLMarkers)
}

// autoBlockWith wraps a body in the markers, one per line.
func autoBlockWith(body []byte, m autoMarkers) []byte {
	var buf bytes.Buffer
	buf.WriteString(m.begin)
	buf.WriteByte('\n')
	buf.Write(body)
	if !bytes.HasSuffix(body, []byte("\n")) {
		buf.WriteByte('\n')
	}
	buf.WriteString(m.end)
	buf.WriteByte('\n')
	return buf.Bytes()
}

// autoFindBlockWith returns the byte range of the marked block, including
// the line of the end marker. A begin marker without an end extends to
// the end of the text, so a damaged block is replaced instead of
// duplicated.
func autoFindBlockWith(text []byte, m autoMarkers) (int, int, bool) {
	start := bytes.Index(text, []byte(m.begin))
	if start < 0 {
		return 0, 0, false
	}
	rel := bytes.Index(text[start:], []byte(m.end))
	if rel < 0 {
		return start, len(text), true
	}
	end := start + rel + len(m.end)
	if end < len(text) && text[end] == '\n' {
		end++
	}
	return start, end, true
}

// autoBlockBody returns the body of an asset that may already carry the
// markers, so that the embedded block file and a bare text install alike.
func autoBlockBody(asset []byte) []byte {
	return autoBlockBodyWith(asset, autoHTMLMarkers)
}

func autoBlockBodyWith(asset []byte, m autoMarkers) []byte {
	start := bytes.Index(asset, []byte(m.begin))
	end := bytes.LastIndex(asset, []byte(m.end))
	if start < 0 || end < start {
		return asset
	}
	body := asset[start+len(m.begin) : end]
	return bytes.TrimLeft(body, "\n")
}

// autoUpsertBlock replaces the marked block in text, or appends one after
// a blank line.
func autoUpsertBlock(text, body []byte) []byte {
	return autoUpsertBlockWith(text, body, autoHTMLMarkers)
}

func autoUpsertBlockWith(text, body []byte, m autoMarkers) []byte {
	block := autoBlockWith(autoBlockBodyWith(body, m), m)
	start, end, ok := autoFindBlockWith(text, m)
	if ok {
		return autoConcat(text[:start], block, text[end:])
	}
	if len(bytes.TrimSpace(text)) == 0 {
		return block
	}
	head := text
	if !bytes.HasSuffix(head, []byte("\n")) {
		head = append(append([]byte{}, head...), '\n')
	}
	return autoConcat(head, []byte("\n"), block)
}

// autoRemoveBlock removes the marked block and the blank line that
// separated it from the text above.
func autoRemoveBlock(text []byte) []byte {
	return autoRemoveBlockWith(text, autoHTMLMarkers)
}

func autoRemoveBlockWith(text []byte, m autoMarkers) []byte {
	start, end, ok := autoFindBlockWith(text, m)
	if !ok {
		return text
	}
	head := text[:start]
	if bytes.HasSuffix(head, []byte("\n\n")) {
		head = head[:len(head)-1]
	}
	return autoConcat(head, text[end:])
}

func autoConcat(parts ...[]byte) []byte {
	var out []byte
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}
