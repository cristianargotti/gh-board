package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// maxLine bounds one message. Bodies are capped at one megabyte by the
// commands, so a longer line is not a valid message of this server.
const maxLine = 16 << 20

// errLineTooLong marks a message that exceeded maxLine; the rest of the
// line is discarded and the client gets a parse error.
var errLineTooLong = errors.New("message exceeds the line limit")

// Serve reads messages from in until it closes and writes the responses
// to out. It returns nil on end of file, the context error when the
// context ends between two messages, and a read or write error otherwise.
func (s *Server) Serve(ctx context.Context, in io.Reader, out io.Writer) error {
	reader := bufio.NewReaderSize(in, 64<<10)
	writer := bufio.NewWriter(out)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		line, err := readLine(reader)
		switch {
		case errors.Is(err, io.EOF):
			return nil
		case errors.Is(err, errLineTooLong):
			if err := write(writer, errorResponse(nil, newRPCError(codeParse, "%s", err.Error()))); err != nil {
				return err
			}
			continue
		case err != nil:
			return fmt.Errorf("mcp: read: %w", err)
		}
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		if resp, ok := s.handle(ctx, line); ok {
			if err := write(writer, resp); err != nil {
				return err
			}
		}
	}
}

// readLine returns one line without its terminator. A final line without
// a terminator is returned as is; the next call reports end of file.
func readLine(reader *bufio.Reader) ([]byte, error) {
	var line []byte
	for {
		chunk, err := reader.ReadSlice('\n')
		line = append(line, chunk...)
		if len(line) > maxLine {
			return nil, discardLine(reader, err)
		}
		switch {
		case err == nil:
			return line[:len(line)-1], nil
		case errors.Is(err, bufio.ErrBufferFull):
			continue
		case errors.Is(err, io.EOF) && len(line) > 0:
			return line, nil
		default:
			return nil, err
		}
	}
}

// discardLine skips to the end of an oversized line so the next message
// starts clean; err is the result of the read that crossed the limit.
func discardLine(reader *bufio.Reader, err error) error {
	for errors.Is(err, bufio.ErrBufferFull) {
		_, err = reader.ReadSlice('\n')
	}
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return errLineTooLong
}

// write encodes one response on one line. The encoder escapes every line
// break inside strings, so the framing holds whatever the payload is.
func write(writer *bufio.Writer, resp response) error {
	enc := json.NewEncoder(writer)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(resp); err != nil {
		return fmt.Errorf("mcp: encode response: %w", err)
	}
	return writer.Flush()
}

// handle processes one message and reports whether a response is due:
// notifications get none, every request gets exactly one.
func (s *Server) handle(ctx context.Context, line []byte) (response, bool) {
	req, perr := decodeRequest(line)
	if perr != nil {
		if perr.Code == codeParse {
			return errorResponse(nil, perr), true
		}
		return errorResponse(req.ID, perr), true
	}
	if req.isNotification() {
		s.notify(req)
		return response{}, false
	}
	result, err := s.dispatch(ctx, req)
	if err != nil {
		return errorResponse(req.ID, err), true
	}
	return resultResponse(req.ID, result), true
}

// notify handles the notifications the server understands and ignores the
// rest, as the specification requires (a notification never gets a
// response).
func (s *Server) notify(req request) {
	if req.Method == "notifications/initialized" {
		s.initialized = true
	}
}

// dispatch routes a request. A modern request carries its protocol
// version in _meta and needs no handshake; a legacy request must follow
// initialize, except ping and the discover probe.
func (s *Server) dispatch(ctx context.Context, req request) (any, error) {
	version, modern := requestVersion(req.Params)
	if modern && version != modernVersion {
		return nil, unsupportedVersion(version)
	}
	switch req.Method {
	case "initialize":
		return s.initialize(req.Params)
	case "server/discover":
		return s.discover(), nil
	case "ping":
		return s.decorate(map[string]any{}, modern), nil
	case "tools/list":
		if !modern && !s.initialized {
			return nil, newRPCError(codeInvalidRequest, "initialize first")
		}
		return s.decorate(s.listTools(modern), modern), nil
	case "tools/call":
		if !modern && !s.initialized {
			return nil, newRPCError(codeInvalidRequest, "initialize first")
		}
		result, err := s.callTool(ctx, req.Params)
		if err != nil {
			return nil, err
		}
		return s.decorate(result, modern), nil
	}
	return nil, newRPCError(codeMethodNotFound, "method not found: %s", req.Method)
}
