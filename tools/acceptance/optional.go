package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

func (s *suite) checkOptional() {
	s.optional(codexAgent, "codex execpolicy", s.checkPolicy)
	s.optional(claudeAgent, "claude plugin validate", func(program string) error {
		return success(s.execute(command{program: program, args: []string{"plugin", "validate", "agents/claude"}, dir: s.root, env: s.env}))
	})
}

func (s *suite) optional(cli, name string, check func(string) error) {
	program, err := s.lookup(cli)
	if errors.Is(err, exec.ErrNotFound) {
		s.rows = append(s.rows, row{name, skipped, cli + " is absent from PATH"})
		return
	}
	if err == nil {
		err = check(program)
	}
	s.record(name, err, "verified")
}

func (s *suite) checkPolicy(program string) error {
	rules := filepath.Join(s.home, ".codex", "rules", "gh-board.rules")
	args := append([]string{"execpolicy", "check", "--rules", rules}, strings.Fields(destructive)...)
	r := s.execute(command{program: program, args: args, dir: s.home, env: s.env})
	if err := success(r); err != nil {
		return err
	}
	var answer struct {
		Decision string `json:"decision"`
	}
	if err := json.Unmarshal([]byte(r.stdout), &answer); err != nil {
		return fmt.Errorf("invalid execpolicy JSON: %w", err)
	}
	if answer.Decision != "forbidden" {
		return fmt.Errorf("execpolicy decision %q, want forbidden", answer.Decision)
	}
	return nil
}
