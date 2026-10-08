package commands

import (
	"strings"

	"github.com/cristianargotti/gh-board/internal/config"
	"github.com/cristianargotti/gh-board/internal/domain"
)

func (s *writeSession) writeCheckStatusConfig() error {
	mismatches, err := config.Validate(s.cfg, s.project)
	if err != nil {
		return err
	}
	for _, mismatch := range mismatches {
		if strings.HasPrefix(mismatch.Path, "capabilities.status") || strings.HasPrefix(mismatch.Path, "policy") {
			return domain.Errorf(domain.ExitUsage, "board.yml %s: %s", mismatch.Path, mismatch.Reason)
		}
	}
	return nil
}
