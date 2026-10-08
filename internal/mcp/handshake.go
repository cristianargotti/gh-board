package mcp

import "encoding/json"

// _meta keys of the 2026-07-28 revision.
const (
	metaKey        = "_meta"
	metaVersion    = "io.modelcontextprotocol/protocolVersion"
	metaServerInfo = "io.modelcontextprotocol/serverInfo"
)

// Cache hints of the modern list results: the catalog is fixed for the
// life of the binary and private to the person running it.
const (
	listTTLMs      = 3600000
	listCacheScope = "private"
	resultComplete = "complete"
)

// requestVersion reads the protocol version a modern request declares in
// _meta; false when the request carries none, which marks a legacy
// client.
func requestVersion(params json.RawMessage) (string, bool) {
	if len(params) == 0 {
		return "", false
	}
	var p struct {
		Meta map[string]json.RawMessage `json:"_meta"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return "", false
	}
	raw, ok := p.Meta[metaVersion]
	if !ok {
		return "", false
	}
	var version string
	if err := json.Unmarshal(raw, &version); err != nil {
		return string(raw), true
	}
	return version, true
}

// unsupportedVersion is the error a modern client receives for a version
// this server does not speak, with the list it can retry with.
func unsupportedVersion(requested string) *rpcError {
	err := newRPCError(codeUnsupportedVersion, "Unsupported protocol version")
	err.Data = map[string]any{"supported": []string{modernVersion}, "requested": requested}
	return err
}

// initialize answers the legacy handshake. A requested version this
// server speaks is echoed; any other gets the latest legacy version, as
// the lifecycle specification requires.
func (s *Server) initialize(params json.RawMessage) (any, error) {
	var p struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	if err := decodeParams(params, &p); err != nil {
		return nil, err
	}
	negotiated := latestLegacy
	for _, v := range legacyVersions {
		if v == p.ProtocolVersion {
			negotiated = v
		}
	}
	s.initialized = true
	result := map[string]any{
		"protocolVersion": negotiated,
		"capabilities":    capabilities(),
		"serverInfo":      s.serverInfo(),
	}
	if s.Instructions != "" {
		result["instructions"] = s.Instructions
	}
	return result, nil
}

// discover answers the modern probe with the versions, the capabilities
// and the identity, so a dual-era client stays modern.
func (s *Server) discover() map[string]any {
	result := map[string]any{
		"supportedVersions": []string{modernVersion},
		"capabilities":      capabilities(),
		"ttlMs":             listTTLMs,
		"cacheScope":        listCacheScope,
	}
	if s.Instructions != "" {
		result["instructions"] = s.Instructions
	}
	return s.decorate(result, true)
}

// decorate adds what every modern result carries: the result type and
// the server identity. Legacy results stay as their revision defines
// them.
func (s *Server) decorate(result map[string]any, modern bool) map[string]any {
	if !modern {
		return result
	}
	result["resultType"] = resultComplete
	result[metaKey] = map[string]any{metaServerInfo: s.serverInfo()}
	return result
}
