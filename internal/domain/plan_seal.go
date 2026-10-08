package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// PlanIDLayout is the sortable timestamp a plan id starts with.
const PlanIDLayout = "20060102T150405Z"

// NewPlanID derives a plan id from the creation instant and a seed such as
// the actor and the command: sortable by time and unique for one user.
func NewPlanID(now time.Time, seed string) string {
	sum := sha256.Sum256([]byte(now.UTC().Format(time.RFC3339Nano) + "|" + seed))
	return now.UTC().Format(PlanIDLayout) + "-" + hex.EncodeToString(sum[:4])
}

// Canonical returns the JSON the hash covers: the plan with Hash empty.
func (p Plan) Canonical() ([]byte, error) {
	p.Hash = ""
	return json.Marshal(p)
}

// Seal completes a plan before it is written: the expiry from the creation
// time, the step indexes in order and the hash of the canonical content.
func (p Plan) Seal() (Plan, error) {
	if p.CreatedAt.IsZero() {
		return Plan{}, fmt.Errorf("plan: created_at is required: %w", ErrUsage)
	}
	if len(p.Steps) == 0 {
		return Plan{}, fmt.Errorf("plan: at least one step is required: %w", ErrUsage)
	}
	if p.ExpiresAt.IsZero() {
		p.ExpiresAt = p.CreatedAt.Add(PlanExpiry)
	}
	for i := range p.Steps {
		p.Steps[i].Index = i
	}
	hash, err := p.ComputeHash()
	if err != nil {
		return Plan{}, err
	}
	p.Hash = hash
	return p, nil
}

// Verify recomputes the hash and compares it with the stored one. A
// mismatch means the file was edited and apply refuses it (exit code 6).
func (p Plan) Verify() error {
	hash, err := p.ComputeHash()
	if err != nil {
		return err
	}
	if p.Hash == "" || !strings.EqualFold(hash, p.Hash) {
		return fmt.Errorf("plan %s: hash mismatch, the file was modified: %w", p.ID, ErrApplyRefused)
	}
	return nil
}

// CheckApplicable refuses a plan that expired or whose actor is not the
// current viewer (exit code 6). Both checks run before any write.
func (p Plan) CheckApplicable(now time.Time, actor string) error {
	if p.Expired(now) {
		return fmt.Errorf("plan %s: expired at %s: %w", p.ID, p.ExpiresAt.UTC().Format(time.RFC3339), ErrApplyRefused)
	}
	if !strings.EqualFold(p.Actor, actor) {
		return fmt.Errorf("plan %s: created by %s, current viewer is %s: %w", p.ID, p.Actor, actor, ErrApplyRefused)
	}
	return nil
}
