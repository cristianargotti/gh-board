package audit

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
)

// MaxLineBytes bounds one JSONL line; a longer line is a corrupt file.
const MaxLineBytes = 1 << 20

// AppendLine adds one line to a JSONL file, creating the directory with
// DirPerm and the file with FilePerm. The data is written in a single
// call so concurrent appenders never interleave inside a line.
func AppendLine(path string, line []byte) error {
	if _, err := statFile(path); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), DirPerm); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(path), err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, FilePerm) //nolint:gosec // state file under the user directory
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	if !bytes.HasSuffix(line, []byte("\n")) {
		line = append(bytes.Clone(line), '\n')
	}
	_, err = f.Write(line)
	if err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("append %s: %w", path, err)
	}
	return nil
}

// ReadLines returns the non-empty lines of a JSONL file in order. A
// missing file yields no lines and no error; a directory at the path, or
// a file on the way to it, is an error.
func ReadLines(path string) ([][]byte, error) {
	found, err := statFile(path)
	if err != nil || !found {
		return nil, err
	}
	f, err := os.Open(path) //nolint:gosec // state file under the user directory
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), MaxLineBytes)
	var lines [][]byte
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		lines = append(lines, bytes.Clone(line))
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return lines, nil
}
