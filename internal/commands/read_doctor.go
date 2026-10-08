package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cristianargotti/gh-board/internal/config"
	"github.com/cristianargotti/gh-board/internal/render"
)

// readDoctorReport is the --json form of doctor: every check with its
// finding, and the problems a person should act on.
type readDoctorReport struct {
	GeneratedAt   string                `json:"generated_at"`
	Config        readDoctorConfig      `json:"config"`
	Environment   readDoctorEnvironment `json:"environment"`
	Identity      readDoctorIdentity    `json:"identity"`
	Project       readDoctorProject     `json:"project"`
	Repository    readDoctorRepository  `json:"repository"`
	Mismatches    []readDoctorMismatch  `json:"mismatches"`
	Artifacts     []readDoctorArtifact  `json:"artifacts"`
	ArtifactsNote string                `json:"artifacts_note,omitempty"`
	Hooks         []readDoctorHook      `json:"hooks"`
	HooksNote     string                `json:"hooks_note,omitempty"`
	MCP           []readDoctorHook      `json:"mcp"`
	MCPNote       string                `json:"mcp_note,omitempty"`
	Binary        readDoctorBinary      `json:"binary"`
	Watch         readDoctorWatch       `json:"watch"`
	Problems      []string              `json:"problems"`
}

type readDoctorConfig struct {
	Source         string   `json:"source"`
	Path           string   `json:"path,omitempty"`
	SHA256         string   `json:"sha256,omitempty"`
	Generic        bool     `json:"generic"`
	Project        string   `json:"project,omitempty"`
	Repository     string   `json:"repository,omitempty"`
	DefaultProject string   `json:"default_project,omitempty"`
	DefaultPath    string   `json:"default_path,omitempty"`
	Candidates     []string `json:"candidates,omitempty"`
	Error          string   `json:"error,omitempty"`
}

type readDoctorEnvironment struct {
	ConfigPath  string `json:"config_path,omitempty"`
	Home        string `json:"home,omitempty"`
	GHToken     bool   `json:"gh_token_set"`
	GitHubToken bool   `json:"github_token_set"`
	Shadow      string `json:"shadow,omitempty"`
	ConfigDir   string `json:"config_dir"`
	StateDir    string `json:"state_dir"`
	CacheDir    string `json:"cache_dir"`
}

type readDoctorIdentity struct {
	Login       string `json:"login,omitempty"`
	Host        string `json:"host,omitempty"`
	TokenSource string `json:"token_source,omitempty"`
	Shadowed    bool   `json:"shadowed"`
	Error       string `json:"error,omitempty"`
}

type readDoctorProject struct {
	Ref        string   `json:"ref,omitempty"`
	Title      string   `json:"title,omitempty"`
	ViewerRole string   `json:"viewer_role,omitempty"`
	ItemCount  int      `json:"item_count"`
	OK         bool     `json:"ok"`
	Error      string   `json:"error,omitempty"`
	Notes      []string `json:"notes,omitempty"`
}

type readDoctorRepository struct {
	Name       string `json:"name,omitempty"`
	Permission string `json:"permission,omitempty"`
	CanWrite   bool   `json:"can_write"`
	Error      string `json:"error,omitempty"`
}

type readDoctorMismatch struct {
	Path     string   `json:"path"`
	Value    string   `json:"value"`
	Reason   string   `json:"reason"`
	Commands []string `json:"commands"`
}

type readDoctorArtifact struct {
	Path   string `json:"path"`
	OK     bool   `json:"ok"`
	Reason string `json:"reason,omitempty"`
}

type readDoctorBinary struct {
	Path            string `json:"path,omitempty"`
	SHA256          string `json:"sha256,omitempty"`
	ReleaseChecksum string `json:"release_checksum,omitempty"`
	Verified        *bool  `json:"verified,omitempty"`
	Note            string `json:"note,omitempty"`
}

type readDoctorWatch struct {
	StatePath   string `json:"state_path"`
	Present     bool   `json:"present"`
	GeneratedAt string `json:"generated_at,omitempty"`
	Age         string `json:"age,omitempty"`
	Summary     string `json:"summary,omitempty"`
	Alerts      int    `json:"alerts"`
	Note        string `json:"note,omitempty"`
}

func (r *readDoctorReport) problem(text string) {
	r.Problems = append(r.Problems, text)
}

func readDoctorCommand(deps *Deps) *cobra.Command {
	return &cobra.Command{
		Use:     "doctor",
		Short:   "Check configuration, identity, permissions, board.yml, guard artifacts, binary and watch",
		GroupID: GroupRead,
		Args:    cobra.NoArgs,
		RunE:    func(cmd *cobra.Command, _ []string) error { return readDoctor(cmd.Context(), deps) },
	}
}

// readDoctor runs every check and reports; a failing check is a finding,
// not an exit code, so the whole picture is always printed.
func readDoctor(ctx context.Context, deps *Deps) error {
	rep := &readDoctorReport{
		GeneratedAt: readStamp(deps.Now()), Mismatches: []readDoctorMismatch{},
		Artifacts: []readDoctorArtifact{}, Hooks: []readDoctorHook{}, MCP: []readDoctorHook{}, Problems: []string{},
	}
	cfg := readDoctorConfigCheck(deps, rep)
	rep.Environment = readDoctorEnvironmentOf(config.Environ(), deps.Dirs)
	readDoctorIdentityCheck(ctx, deps, rep)
	project := readDoctorProjectCheck(ctx, deps, cfg, rep)
	readDoctorMismatchCheck(ctx, deps, cfg, project, rep)
	readDoctorRepositoryCheck(ctx, deps, cfg, rep)
	readDoctorArtifactsCheck(deps, rep)
	readDoctorHooksCheck(deps, rep)
	readDoctorMCPCheck(deps, rep)
	readDoctorBinaryCheck(ctx, deps, rep)
	readDoctorWatchCheck(deps, rep)
	doc := readDoctorDocument(rep)
	doc.Data = rep
	return readRender(deps, doc)
}

func readDoctorDocument(r *readDoctorReport) *render.Document {
	doc := render.NewDocument("Doctor")
	readDoctorConfigSection(doc.AddSection("Configuration"), r)
	readDoctorEnvironmentSection(doc.AddSection("Environment"), r.Environment)
	identity := doc.AddSection("Identity")
	identity.AddKeyValue("Login", r.Identity.Login).AddKeyValue("Host", r.Identity.Host)
	identity.AddKeyValue("Token source", r.Identity.TokenSource)
	readDoctorNote(identity, r.Identity.Error)
	if r.Identity.Shadowed {
		identity.AddNote("an environment token shadows the stored gh credentials: writes are attributed to that token")
	}
	project := doc.AddSection(readKeyProject)
	project.AddKeyValue("Ref", r.Project.Ref).AddKeyValue(readKeyTitle, r.Project.Title)
	project.AddKeyValue(readKeyRole, r.Project.ViewerRole).AddKeyValue(readKeyItems, fmt.Sprint(r.Project.ItemCount))
	readDoctorNote(project, r.Project.Error)
	for _, note := range r.Project.Notes {
		project.AddNote(note)
	}
	repo := doc.AddSection("Repository")
	repo.AddKeyValue(readKeyName, r.Repository.Name).AddKeyValue("Permission", r.Repository.Permission)
	repo.AddKeyValue("Can write", readYesNo(r.Repository.CanWrite))
	readDoctorNote(repo, r.Repository.Error)
	readDoctorTables(doc, r)
	readDoctorState(doc, r)
	return doc
}

func readDoctorConfigSection(sec *render.Section, r *readDoctorReport) {
	mode := "team (board.yml)"
	if r.Config.Generic {
		mode = "generic (no board.yml)"
	}
	sec.AddKeyValue("Source", r.Config.Source).AddKeyValue("File", r.Config.Path).AddKeyValue("SHA-256", r.Config.SHA256)
	sec.AddKeyValue("Mode", mode).AddKeyValue(readKeyProject, r.Config.Project).AddKeyValue("Repository", r.Config.Repository)
	sec.AddKeyValue("Default project", r.Config.DefaultProject).AddKeyValue("Default file", r.Config.DefaultPath)
	if r.Config.DefaultProject == "" && r.Config.Error == "" {
		sec.AddNote("no default project recorded: gh board use owner/number records one for every command")
	}
	if len(r.Config.Candidates) > 0 {
		sec.AddNote("per-user files without a default: " + strings.Join(r.Config.Candidates, ", "))
	}
	readDoctorNote(sec, r.Config.Error)
}

func readDoctorEnvironmentSection(sec *render.Section, e readDoctorEnvironment) {
	sec.AddKeyValue(config.EnvConfig, e.ConfigPath).AddKeyValue(config.EnvHome, e.Home)
	sec.AddKeyValue(config.EnvGHToken+" set", readYesNo(e.GHToken)).AddKeyValue(config.EnvGitHubToken+" set", readYesNo(e.GitHubToken))
	sec.AddKeyValue("Config dir", e.ConfigDir).AddKeyValue("State dir", e.StateDir).AddKeyValue("Cache dir", e.CacheDir)
	if e.Shadow != "" {
		sec.AddNote(e.Shadow + " takes precedence over the keyring token in gh")
	}
}

// readDoctorTables writes the mismatch and artifact tables.
func readDoctorTables(doc *render.Document, r *readDoctorReport) {
	mismatches := doc.AddSection("board.yml mismatches")
	t := mismatches.SetTable(readColPath, "VALUE", readColReason, "COMMANDS")
	for _, m := range r.Mismatches {
		t.AddRow(m.Path, m.Value, m.Reason, strings.Join(m.Commands, ", "))
	}
	if len(r.Mismatches) == 0 {
		mismatches.AddNote(readNoteNone)
	}
	artifacts := doc.AddSection("Agent artifacts")
	at := artifacts.SetTable(readColPath, readColStatus, readColReason)
	for _, a := range r.Artifacts {
		status := "ok"
		if !a.OK {
			status = "FAIL"
		}
		at.AddRow(a.Path, status, a.Reason)
	}
	readDoctorNote(artifacts, r.ArtifactsNote)
	readDoctorBinaryTable(doc, "Agent hooks", r.Hooks, r.HooksNote)
	readDoctorBinaryTable(doc, "MCP servers", r.MCP, r.MCPNote)
}

// readDoctorBinaryTable lists hook commands or MCP server entries with the
// binary each one names.
func readDoctorBinaryTable(doc *render.Document, title string, rows []readDoctorHook, note string) {
	sec := doc.AddSection(title)
	t := sec.SetTable("AGENT", "SCOPE", readColPath, "BINARY", readColStatus, readColReason)
	for _, h := range rows {
		status := "ok"
		if !h.OK {
			status = "FAIL"
		}
		t.AddRow(h.Agent, h.Scope, h.Path, h.Binary, status, h.Reason)
	}
	readDoctorNote(sec, note)
}

// readDoctorState writes the binary and watch state and the problems.
func readDoctorState(doc *render.Document, r *readDoctorReport) {
	binary := doc.AddSection("Binary")
	binary.AddKeyValue("Path", r.Binary.Path).AddKeyValue("SHA-256", r.Binary.SHA256)
	binary.AddKeyValue("Release checksum", r.Binary.ReleaseChecksum)
	if r.Binary.Verified != nil {
		binary.AddKeyValue("Verified", readYesNo(*r.Binary.Verified))
	}
	readDoctorNote(binary, r.Binary.Note)
	watch := doc.AddSection("Watch")
	watch.AddKeyValue("State file", r.Watch.StatePath).AddKeyValue("Present", readYesNo(r.Watch.Present))
	watch.AddKeyValue(readKeyGenerated, r.Watch.GeneratedAt).AddKeyValue("Age", r.Watch.Age)
	watch.AddKeyValue("Summary", r.Watch.Summary).AddKeyValue("Alerts", fmt.Sprint(r.Watch.Alerts))
	readDoctorNote(watch, r.Watch.Note)
	problems := doc.AddSection("Problems")
	for _, p := range r.Problems {
		problems.AddItem(p)
	}
	if len(r.Problems) == 0 {
		problems.AddNote(readNoteNone)
	}
	doc.AddSection("").AddKeyValue(readKeyGenerated, r.GeneratedAt)
}

func readDoctorNote(sec *render.Section, note string) {
	if note != "" {
		sec.AddNote(note)
	}
}
