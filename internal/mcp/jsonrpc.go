// Package mcp serves the kit over the Model Context Protocol on standard
// input and output (ADR-005): JSON-RPC 2.0, one message per line, with the
// standard library only. It knows nothing about GitHub: every tool call
// runs a command of the kit in process and maps its exit code to a result.
package mcp

import (
	"encoding/json"
	"errors"
	"fmt"
)

// JSON-RPC 2.0 error codes, plus the MCP code for an unsupported
// protocol version (specification 2026-07-28).
const (
	codeParse              = -32700
	codeInvalidRequest     = -32600
	codeMethodNotFound     = -32601
	codeInvalidParams      = -32602
	codeInternal           = -32603
	codeUnsupportedVersion = -32022
)

// request is one incoming JSON-RPC message. ID is kept raw so that the
// response carries it back unchanged, a number or a string alike; a nil
// ID marks a notification.
type request struct {
	Version string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

// isNotification reports whether the message carries no id. JSON null is
// treated as an id of null, which JSON-RPC allows for a request.
func (r request) isNotification() bool {
	return len(r.ID) == 0
}

// response is one outgoing JSON-RPC message: a result or an error.
type response struct {
	Version string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// rpcError is the error member of a response.
type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// Error implements the error interface so handlers can return an rpcError
// and the loop turns it into the error response.
func (e *rpcError) Error() string {
	return fmt.Sprintf("jsonrpc %d: %s", e.Code, e.Message)
}

// newRPCError builds a protocol error.
func newRPCError(code int, format string, args ...any) *rpcError {
	return &rpcError{Code: code, Message: fmt.Sprintf(format, args...)}
}

// nullID is the id of a response to a message whose id could not be read.
var nullID = json.RawMessage("null")

// resultResponse pairs a result with the request id.
func resultResponse(id json.RawMessage, result any) response {
	if len(id) == 0 {
		id = nullID
	}
	return response{Version: jsonrpcVersion, ID: id, Result: result}
}

// errorResponse pairs an error with the request id. Any error that is not
// an rpcError becomes an internal error, so a handler bug never produces
// a malformed message.
func errorResponse(id json.RawMessage, err error) response {
	if len(id) == 0 {
		id = nullID
	}
	var coded *rpcError
	if !errors.As(err, &coded) {
		coded = newRPCError(codeInternal, "%s", err.Error())
	}
	return response{Version: jsonrpcVersion, ID: id, Error: coded}
}

// jsonrpcVersion is the only protocol version of JSON-RPC.
const jsonrpcVersion = "2.0"

// decodeRequest parses one line. A JSON array is a batch, which the MCP
// specification removed in 2025-06-18; it is refused as an invalid
// request rather than processed element by element.
func decodeRequest(line []byte) (request, *rpcError) {
	if len(line) > 0 && line[0] == '[' {
		return request{}, newRPCError(codeInvalidRequest, "batch requests are not supported")
	}
	var req request
	if err := json.Unmarshal(line, &req); err != nil {
		return request{}, newRPCError(codeParse, "parse error: %s", err.Error())
	}
	if req.Version != jsonrpcVersion {
		return req, newRPCError(codeInvalidRequest, "jsonrpc must be %q", jsonrpcVersion)
	}
	if req.Method == "" {
		return req, newRPCError(codeInvalidRequest, "method is required")
	}
	return req, nil
}

// decodeParams reads the params member into v; an absent member is an
// empty object. A params member that is not an object is invalid.
func decodeParams(raw json.RawMessage, v any) *rpcError {
	if len(raw) == 0 || string(raw) == "null" {
		raw = json.RawMessage("{}")
	}
	if raw[0] != '{' {
		return newRPCError(codeInvalidParams, "invalid params: expected an object")
	}
	if err := json.Unmarshal(raw, v); err != nil {
		return newRPCError(codeInvalidParams, "invalid params: %s", err.Error())
	}
	return nil
}
