package alerts

import (
	"time"

	"github.com/cristianargotti/gh-board/internal/audit"
)

// Prune drops the audit lines older than audit.Retention, the
// housekeeping every watch run performs (section 6.9), and returns how
// many went. The plan and journal files belong to the plan package, which
// the command prunes with the same retention.
func Prune(stateDir string, now time.Time) (int, error) {
	return audit.Prune(stateDir, now.Add(-audit.Retention))
}
