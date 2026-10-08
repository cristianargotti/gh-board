package domain

import (
	"sort"
	"strings"
	"time"
)

// StandupInput supplies the board snapshot and an optional login filter.
type StandupInput struct {
	Config *Config
	Items  []Item
	Now    time.Time
	For    string
}

// StandupPerson groups observed movement, active work and blockers by assignee.
type StandupPerson struct {
	Login          string        `json:"login"`
	MovedYesterday []ItemSummary `json:"moved_yesterday"`
	ActiveToday    []ItemSummary `json:"active_today"`
	Blocked        []ItemSummary `json:"blocked"`
}

// StandupResult names the calendar dates (YYYY-MM-DD, board zone) behind
// yesterday and today.
type StandupResult struct {
	GeneratedAt time.Time       `json:"generated_at"`
	Yesterday   string          `json:"yesterday"`
	Today       string          `json:"today"`
	People      []StandupPerson `json:"people"`
}

// BuildStandup uses the status field timestamp, never unrelated issue updates,
// as movement evidence. Multi-assignee items appear under each assignee.
func BuildStandup(in StandupInput) StandupResult {
	loc := LocationOf(in.Config)
	today := Today(in.Now, loc)
	yesterday := today.AddDate(0, 0, -1)
	out := StandupResult{GeneratedAt: in.Now, Today: DateText(today), Yesterday: DateText(yesterday), People: []StandupPerson{}}
	people := map[string]*StandupPerson{}
	if in.For != "" {
		people[strings.ToLower(in.For)] = standupPerson(in.For)
	}
	for _, it := range in.Items {
		if it.Archived {
			continue
		}
		status, _ := it.Value(StatusFieldName(in.Config))
		moved := status.Value != "" && !status.UpdatedAt.IsZero() && Today(status.UpdatedAt, loc).Equal(yesterday)
		active := it.IsOpen() && IsActive(in.Config, it)
		blocked := it.IsOpen() && standupBlocked(in.Config, it)
		if moved || active || blocked {
			standupAdd(people, in, it, moved, active, blocked)
		}
	}
	for _, person := range people {
		out.People = append(out.People, *person)
	}
	sort.Slice(out.People, func(i, j int) bool {
		return strings.ToLower(out.People[i].Login) < strings.ToLower(out.People[j].Login)
	})
	return out
}

func standupPerson(login string) *StandupPerson {
	return &StandupPerson{Login: login, MovedYesterday: []ItemSummary{}, ActiveToday: []ItemSummary{}, Blocked: []ItemSummary{}}
}

func standupBlocked(cfg *Config, it Item) bool {
	caps := CapabilitiesOf(cfg)
	return it.Issue.BlockedByCount > 0 || caps.Blocked != nil && strings.TrimSpace(it.Text(caps.Blocked.Field)) != ""
}

func standupAdd(people map[string]*StandupPerson, in StandupInput, it Item, moved, active, blocked bool) {
	seen := map[string]bool{}
	for _, user := range it.Issue.Assignees {
		key := strings.ToLower(user.Login)
		if key == "" || seen[key] || in.For != "" && !strings.EqualFold(in.For, user.Login) {
			continue
		}
		seen[key] = true
		if people[key] == nil {
			people[key] = standupPerson(user.Login)
		}
		person, summary := people[key], SummarizeItem(in.Config, it)
		if moved {
			person.MovedYesterday = append(person.MovedYesterday, summary)
		}
		if active {
			person.ActiveToday = append(person.ActiveToday, summary)
		}
		if blocked {
			person.Blocked = append(person.Blocked, summary)
		}
	}
}
