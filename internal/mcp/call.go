package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cristianargotti/gh-board/internal/domain"
)

// applyCommand is the line a person runs for a plan; the tools return it
// verbatim so the agent can quote it.
const applyCommand = "gh board apply "

// Sentences appended to the descriptions, one per tier.
const (
	noteRead  = "Read tool: no side effects."
	noteWrite = "Direct write: journaled and reversible. dry_run shows the change without sending it, reason is stored in the audit line and expect refuses the write when a current value differs (exit code 4). A done status or a bulk above the team threshold returns a plan instead of writing."
	notePlan  = "Plan tool: writes a plan and returns its id with the exact gh board apply command. Nothing changes until a person runs that command in a terminal; apply is never a tool."
)

// listTools builds the tools/list result; modern results add the cache
// hints the 2026-07-28 revision requires.
func (s *Server) listTools(modern bool) map[string]any {
	tools := make([]map[string]any, 0, len(s.Tools))
	for _, t := range s.Tools {
		tools = append(tools, map[string]any{
			"name":        t.Name,
			"title":       t.Title,
			"description": describe(t),
			"inputSchema": t.InputSchema,
			"annotations": annotations(t.Tier),
		})
	}
	result := map[string]any{"tools": tools}
	if modern {
		result["ttlMs"] = listTTLMs
		result["cacheScope"] = listCacheScope
	}
	return result
}

// describe appends the tier sentence to the verb's own help.
func describe(t Tool) string {
	note := map[string]string{TierRead: noteRead, TierWrite: noteWrite, TierPlan: notePlan}[t.Tier]
	if note == "" {
		return t.Description
	}
	return strings.TrimSpace(t.Description) + " " + note
}

// annotations declares the behavior of a tier. Nothing in the kit
// destroys anything (level 1), so no tool is destructive; reads are the
// only idempotent ones, and every tool reaches GitHub.
func annotations(tier string) map[string]any {
	return map[string]any{
		"readOnlyHint":    tier == TierRead,
		"destructiveHint": false,
		"idempotentHint":  tier == TierRead,
		"openWorldHint":   true,
	}
}

// callParams is the params member of tools/call.
type callParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// callTool validates the request, then the arguments, and runs the tool.
// A malformed request or an unknown tool is a protocol error; an argument
// the schema rejects is a tool error the model can correct.
func (s *Server) callTool(ctx context.Context, params json.RawMessage) (map[string]any, error) {
	var p callParams
	if err := decodeParams(params, &p); err != nil {
		return nil, err
	}
	if p.Name == "" {
		return nil, newRPCError(codeInvalidParams, "tool name is required")
	}
	tool, ok := s.tool(p.Name)
	if !ok {
		return nil, newRPCError(codeInvalidParams, "Unknown tool: %s", p.Name)
	}
	args := map[string]any{}
	if len(p.Arguments) > 0 && string(p.Arguments) != "null" {
		if err := json.Unmarshal(p.Arguments, &args); err != nil {
			return nil, newRPCError(codeInvalidParams, "arguments must be an object")
		}
	}
	if problems := validate(tool.InputSchema, args); len(problems) > 0 {
		return toolError(tool.Name, strings.Join(problems, "; ")), nil
	}
	if s.Run == nil {
		return toolError(tool.Name, "no runner configured"), nil
	}
	return buildResult(tool.Name, s.Run(ctx, tool, args)), nil
}

// textContent is one text block of a result.
func textContent(text string) map[string]any {
	return map[string]any{"type": "text", "text": text}
}

// toolError is a tool execution error without an exit code: the call
// never reached the command.
func toolError(name, message string) map[string]any {
	return map[string]any{
		"content": []map[string]any{textContent(name + ": " + message)},
		"structuredContent": map[string]any{
			"tool": name, "status": "invalid-arguments", "message": message,
		},
		"isError": true,
	}
}

// buildResult maps an outcome to a tool result. Exit code 0 is a success,
// exit code 5 with a plan in the output is the designed outcome of a plan
// tool (the id and the apply line travel in the envelope), and any other
// code is a tool error carrying the message and the code, never a
// transport error.
func buildResult(name string, out Outcome) map[string]any {
	envelope := map[string]any{"tool": name, "exit_code": int(out.Code), "status": out.Code.String()}
	data, structured := parseOutput(out.Output)
	if structured {
		envelope["data"] = data
	} else if text := strings.TrimSpace(string(out.Output)); text != "" {
		envelope["output"] = text
	}
	if out.Stderr != "" {
		envelope["stderr"] = strings.TrimSpace(out.Stderr)
	}
	line, isError := summarize(name, out, envelope, data)
	content := []map[string]any{textContent(line)}
	if structured {
		content = append(content, textContent(string(bytes.TrimSpace(out.Output))))
	}
	return map[string]any{"content": content, "structuredContent": envelope, "isError": isError}
}

// summarize writes the short line of the result and fills the plan
// members of the envelope when the command produced a plan.
func summarize(name string, out Outcome, envelope map[string]any, data any) (string, bool) {
	if out.Code == domain.ExitOK {
		return name + ": ok", false
	}
	if id, dryRun, ok := planOf(data); ok && out.Code == domain.ExitPlanRequired {
		envelope["plan_id"], envelope["dry_run"] = id, dryRun
		if dryRun {
			return fmt.Sprintf("%s: dry run, plan %s was not saved; rerun without dry_run to create an applicable plan", name, id), false
		}
		envelope["apply"] = applyCommand + id
		return fmt.Sprintf("%s: plan %s saved; a person must run in a terminal: %s%s", name, id, applyCommand, id), false
	}
	message := strings.TrimSpace(out.Message)
	if message == "" {
		message = out.Code.String()
	}
	envelope["message"] = message
	return fmt.Sprintf("%s: %s (exit code %d, %s)", name, message, int(out.Code), out.Code.String()), true
}

// parseOutput decodes the --json document; false when the output is not
// one JSON value.
func parseOutput(output []byte) (any, bool) {
	trimmed := bytes.TrimSpace(output)
	if len(trimmed) == 0 {
		return nil, false
	}
	var data any
	if err := json.Unmarshal(trimmed, &data); err != nil {
		return nil, false
	}
	return data, true
}

// planOf reads the plan id and the dry run flag of a write or plan
// document.
func planOf(data any) (string, bool, bool) {
	doc, ok := data.(map[string]any)
	if !ok {
		return "", false, false
	}
	p, ok := doc["plan"].(map[string]any)
	if !ok {
		return "", false, false
	}
	id, ok := p["id"].(string)
	if !ok || id == "" {
		return "", false, false
	}
	dryRun, _ := doc["dry_run"].(bool)
	return id, dryRun, true
}
