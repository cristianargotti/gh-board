package alerts_test

import (
	"errors"
	"testing"
	"time"

	"github.com/cristianargotti/gh-board/internal/alerts"
	"github.com/cristianargotti/gh-board/internal/domain"
)

var intervalCases = []struct {
	in   string
	want time.Duration
	err  bool
}{
	{in: "", want: 10 * time.Minute},
	{in: "10m", want: 10 * time.Minute},
	{in: " 1h30m ", want: 90 * time.Minute},
	{in: "15", want: 15 * time.Minute},
	{in: "1m", want: time.Minute},
	{in: "24h", want: 24 * time.Hour},
	{in: "30s", err: true},
	{in: "90s", err: true},
	{in: "0", err: true},
	{in: "-5m", err: true},
	{in: "25h", err: true},
	{in: "soon", err: true},
}

func TestParseInterval(t *testing.T) {
	for _, tc := range intervalCases {
		t.Run(tc.in, func(t *testing.T) {
			got, err := alerts.ParseInterval(tc.in)
			if tc.err {
				if !errors.Is(err, domain.ErrUsage) {
					t.Fatalf("error = %v, want a usage error", err)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("ParseInterval(%q) = %s, %v; want %s", tc.in, got, err, tc.want)
			}
		})
	}
	if alerts.MinInterval != time.Minute || alerts.MaxInterval != 24*time.Hour {
		t.Fatal("bounds changed")
	}
}
