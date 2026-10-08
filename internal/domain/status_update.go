package domain

import "time"

// StatusUpdate is one project status update as GitHub lists it. digest
// post reads the list, newest first, to skip an ISO week that already has
// one (section 6.4).
type StatusUpdate struct {
	ID         string
	Body       string
	Status     string
	Creator    string
	CreatedAt  time.Time
	StartDate  *time.Time
	TargetDate *time.Time
}
