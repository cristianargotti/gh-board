package commands

import (
	"context"
	"io"
	"os"
	"strings"

	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/plan"
	"github.com/spf13/cobra"
)

const writeBodyLimit = 1 << 20

func writeCommentCommand(deps *Deps) *cobra.Command {
	cmd := writeCommand("comment <ref[,ref...]> <text|file|->", "Add a comment from text, a file, or standard input", cobra.ExactArgs(2),
		func(cmd *cobra.Command, args []string) error {
			return writeRun(cmd, deps, func(s *writeSession) error {
				body, err := writeBody(cmd, args[1])
				if err != nil {
					return err
				}
				if strings.TrimSpace(body) == "" {
					return domain.Errorf(domain.ExitUsage, "comment must not be empty")
				}
				return s.writeTargets(args[0], func(it domain.Item) error { s.writeComment(it, body); return nil })
			})
		})
	cmd.Annotations = map[string]string{mcpArgsAnnotation: "ref: Issue references separated by commas: owner/repo#n, a GitHub URL or a node id; #n alone works when board.yml names a repository and one item carries that number\n" +
		"text: Comment body in PT-BR, Markdown allowed; @path reads a file"}
	return cmd
}

// writeBody reads the body of a comment or a new issue: "-" is standard
// input, "@path" is a file that must exist, an argument that names an
// existing regular file is that file, and anything else is the text
// itself, whatever characters it holds, so the rule is the same on every
// operating system.
func writeBody(cmd *cobra.Command, source string) (string, error) {
	if source == "-" {
		return writeReadBody(cmd.InOrStdin())
	}
	if path, ok := strings.CutPrefix(source, "@"); ok {
		return writeReadFile(path)
	}
	if info, err := os.Stat(source); err == nil && info.Mode().IsRegular() {
		return writeReadFile(source)
	}
	if len(source) > writeBodyLimit {
		return "", domain.Errorf(domain.ExitUsage, "body exceeds %d bytes", writeBodyLimit)
	}
	return source, nil
}

func writeReadFile(path string) (string, error) {
	file, err := os.Open(path) //nolint:gosec // the user explicitly supplies comment or body content, never configuration
	if err != nil {
		return "", domain.Errorf(domain.ExitUsage, "read body file: %v", err)
	}
	defer func() { _ = file.Close() }()
	return writeReadBody(file)
}

// writeReadBody reads a body from a stream, without the trailing line
// breaks an editor or a pipe leaves, so the plan marker follows the text
// after exactly one blank line.
func writeReadBody(reader io.Reader) (string, error) {
	body, err := io.ReadAll(io.LimitReader(reader, writeBodyLimit+1))
	if err != nil {
		return "", err
	}
	if len(body) > writeBodyLimit {
		return "", domain.Errorf(domain.ExitUsage, "body exceeds %d bytes", writeBodyLimit)
	}
	return strings.TrimRight(string(body), " \t\r\n"), nil
}

func (s *writeSession) writeComment(it domain.Item, body string) {
	index := len(s.actions)
	s.writeAdd(it, domain.OpAddComment, "", body, "Adicionar comentário.", func(ctx context.Context, current domain.Item) error {
		_, err := s.deps.Writer.AddComment(ctx, current.Issue.NodeID, body+"\n\n"+plan.Marker(s.plan.ID, index))
		return err
	})
}
