package commands

import (
	"github.com/cristianargotti/gh-board/internal/guard"
)

// autoSetupRun is one agent of a setup command.
type autoSetupRun struct {
	deps   *Deps
	ws     *autoWorkspace
	spec   autoSetupSpec
	agent  guard.Agent
	scope  autoScope
	strict bool
	mcp    bool
	roots  autoRoots
}

// plan lists the artifacts of the run: the guard and skill layers and,
// when asked, the MCP server entry.
func (r autoSetupRun) plan(strict, mcp bool) ([]autoArtifact, error) {
	artifacts, err := autoPlanArtifacts(r.agent, r.scope, strict, r.roots)
	if err != nil || !mcp {
		return artifacts, err
	}
	entry, err := autoPlanMCP(r.agent, r.scope, r.roots)
	if err != nil {
		return nil, err
	}
	return append(artifacts, entry...), nil
}

// execute installs or uninstalls the artifacts of the agent and keeps the
// receipt in step with the files.
func (r autoSetupRun) execute() ([]autoResult, error) {
	path := autoReceiptPath(r.deps.Dirs, r.agent, r.scope)
	receipt, found, err := autoReadReceipt(path)
	if err != nil {
		return nil, err
	}
	if r.spec.Install {
		return r.install(path, receipt)
	}
	if !found {
		if receipt, err = r.fallbackReceipt(); err != nil {
			return nil, err
		}
	}
	return r.uninstall(path, receipt)
}

// install applies the artifacts of the selected kinds and records them.
func (r autoSetupRun) install(path string, receipt autoReceipt) ([]autoResult, error) {
	artifacts, err := r.plan(r.strict, r.mcp)
	if err != nil {
		return nil, err
	}
	receipt.Agent, receipt.Scope = r.agent, r.scope
	receipt.Strict = receipt.Strict || r.strict
	receipt.Version, receipt.InstalledAt = r.deps.Version, r.deps.Now()
	var results []autoResult
	for _, a := range artifacts {
		if !autoKindIn(r.spec.Kinds, a.Kind) {
			continue
		}
		res, err := r.ws.install(a)
		if err != nil {
			return nil, err
		}
		receipt.upsert(res.Entry)
		results = append(results, res)
	}
	if r.ws.dryRun {
		return results, nil
	}
	return results, autoWriteReceipt(path, receipt)
}

// uninstall reverts the recorded entries of the selected kinds and drops
// the receipt when nothing of the agent remains.
func (r autoSetupRun) uninstall(path string, receipt autoReceipt) ([]autoResult, error) {
	chosen, rest := receipt.split(r.spec.Kinds)
	created := receipt.createdFiles()
	var results []autoResult
	for _, e := range chosen {
		e.Created = created[e.Path]
		res, err := r.ws.uninstall(e)
		if err != nil {
			return nil, err
		}
		results = append(results, res)
	}
	if r.ws.dryRun {
		return results, nil
	}
	if len(rest) == 0 {
		return results, autoDeleteReceipt(path)
	}
	receipt.Entries = rest
	receipt.refreshWrittenHashes(r.ws)
	return results, autoWriteReceipt(path, receipt)
}

// fallbackReceipt rebuilds what an install would have added, in normal
// and in strict mode, when no receipt exists: every fragment is the kit's
// own, so removing the whole footprint removes nothing of the user. Files
// count as pre-existing, so an emptied settings file stays in place, and
// top-level scalars stay too: a file-level setting such as the version of
// Cursor's hooks.json is the agent's, whoever wrote it first.
func (r autoSetupRun) fallbackReceipt() (autoReceipt, error) {
	receipt := autoReceipt{Agent: r.agent, Scope: r.scope, Strict: true}
	for _, strict := range []bool{false, true} {
		artifacts, err := r.plan(strict, true)
		if err != nil {
			return autoReceipt{}, err
		}
		for _, a := range artifacts {
			e := autoReceiptEntry{Kind: a.Kind, Type: a.Type, Path: a.Path, OwnDir: a.OwnDir}
			if a.Fragment != nil {
				e.Delta = autoWithoutScalars(a.Fragment)
			}
			receipt.upsert(e)
		}
	}
	return receipt, nil
}

// autoWithoutScalars copies a fragment without its top-level scalars.
func autoWithoutScalars(fragment map[string]any) map[string]any {
	out := autoJSONClone(fragment).(map[string]any)
	for key, value := range out {
		switch value.(type) {
		case map[string]any, []any:
		default:
			delete(out, key)
		}
	}
	return out
}
