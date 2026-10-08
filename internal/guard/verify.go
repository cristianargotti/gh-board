package guard

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/cristianargotti/gh-board/internal/domain"
)

// SHA256 returns the lowercase hex digest the installer records for an
// artifact and Verify compares.
func SHA256(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// Verify compares every installed artifact with the digest the installer
// recorded, for doctor: a missing, unreadable or modified file is reported
// with its reason. An artifact without a path is a usage error.
func Verify(artifacts []Artifact) ([]VerifyResult, error) {
	out := make([]VerifyResult, 0, len(artifacts))
	for _, a := range artifacts {
		if a.Path == "" {
			return nil, fmt.Errorf("guard verify: artifact without path: %w", domain.ErrUsage)
		}
		out = append(out, verifyOne(a))
	}
	return out, nil
}

func verifyOne(a Artifact) VerifyResult {
	data, err := os.ReadFile(a.Path) //nolint:gosec // the path comes from the installer manifest
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return VerifyResult{Path: a.Path, Reason: "missing"}
	case err != nil:
		return VerifyResult{Path: a.Path, Reason: "unreadable: " + err.Error()}
	}
	got := SHA256(data)
	if !strings.EqualFold(got, a.SHA256) {
		return VerifyResult{Path: a.Path, Reason: fmt.Sprintf("modified: sha256 %s, installed %s", short(got), short(a.SHA256))}
	}
	return VerifyResult{Path: a.Path, OK: true}
}

func short(digest string) string {
	if len(digest) > 12 {
		return digest[:12]
	}
	return digest
}
