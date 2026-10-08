package commands

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/guard"
)

// autoStdinLimit bounds a hook payload; the agents send a few kilobytes.
const autoStdinLimit = 1 << 20

// autoGuardCommand builds "guard check|install|uninstall".
func autoGuardCommand(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "guard",
		Short:   "Check a command against the guard, or install and remove the guard layer alone",
		GroupID: GroupAuto,
		Args:    cobra.NoArgs,
		RunE:    func(c *cobra.Command, _ []string) error { return c.Help() },
	}
	cmd.AddGroup(&cobra.Group{ID: GroupAuto, Title: "Automation:"})
	guardKinds := []autoKind{autoKindGuard}
	cmd.AddCommand(
		autoGuardCheckCommand(deps),
		autoSetupCommand(deps, autoSetupSpec{
			Use:     "install",
			Short:   "Install only the guard layer: deny rules, hooks and rules files",
			Long:    "guard install writes the destructive-command guards of section 6.5 without the skill or the plugin pointer. agent install includes it.",
			Kinds:   guardKinds,
			Install: true,
		}),
		autoSetupCommand(deps, autoSetupSpec{
			Use:   "uninstall",
			Short: "Remove the guard layer that guard install or agent install added",
			Kinds: guardKinds,
		}),
	)
	return cmd
}

// autoGuardCheckCommand builds "guard check --agent <name> [--strict]",
// the command the installed hooks run: the agent's JSON on stdin, the
// answer on stdout and the exit code the agent expects.
func autoGuardCheckCommand(deps *Deps) *cobra.Command {
	var agentName string
	var strict bool
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Evaluate a hook payload from stdin and answer in the agent's shape",
		Long: "guard check reads on stdin the JSON an agent passes to its hook, tokenizes the command, splits it on the " +
			"shell separators, strips the wrappers Claude Code strips and denies when any segment matches a destructive " +
			"pattern. The answer goes to stdout in the agent's documented shape and the process exits with the code the " +
			"agent treats as a block.",
		GroupID: GroupAuto,
		Args:    cobra.NoArgs,
		RunE:    func(c *cobra.Command, _ []string) error { return autoRunGuardCheck(c, deps, agentName, strict) },
	}
	cmd.Flags().StringVar(&agentName, "agent", "", "agent whose hook format to read and answer: claude, cursor or codex")
	cmd.Flags().BoolVar(&strict, "strict", false, "also apply the strict family, as the hooks installed with --strict request")
	_ = cmd.MarkFlagRequired("agent")
	return cmd
}

// autoRunGuardCheck answers one hook call.
func autoRunGuardCheck(cmd *cobra.Command, deps *Deps, agentName string, strict bool) error {
	agent, err := guard.ParseAgent(agentName)
	if err != nil {
		return err
	}
	stdin, err := io.ReadAll(io.LimitReader(cmd.InOrStdin(), autoStdinLimit))
	if err != nil {
		return fmt.Errorf("read the hook payload: %w", err)
	}
	answer, denied, err := autoGuardCheck(agent, stdin, guard.CheckOptions{Strict: strict})
	if err != nil {
		return err
	}
	if len(answer.Body) > 0 {
		if _, err := deps.Out.Write(answer.Body); err != nil {
			return err
		}
	}
	if answer.ExitCode == 0 {
		return nil
	}
	return domain.NewError(domain.ExitCode(answer.ExitCode), autoDenyReason(denied, answer))
}

// autoDenyReason is the error main prints on stderr, which Claude Code and
// Codex feed back to the model as the reason of the block.
func autoDenyReason(denied bool, answer guard.Answer) error {
	text := strings.TrimSpace(answer.Message)
	if text == "" {
		text = strings.TrimSpace(string(answer.Body))
	}
	switch {
	case denied && text != "":
		return errors.New(text)
	case denied:
		return errors.New("command denied by the gh-board guard")
	case text != "":
		return fmt.Errorf("the gh-board guard could not evaluate the command: %s", text)
	}
	return errors.New("the gh-board guard could not evaluate the command")
}
