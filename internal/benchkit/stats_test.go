package benchkit

import (
	"testing"
	"time"
)

func TestSummarizeDurationsCalculatesPercentilesAndRPS(t *testing.T) {
	start := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	end := start.Add(2 * time.Second)
	summary := SummarizeDurations("feed_hot", []time.Duration{
		10 * time.Millisecond,
		20 * time.Millisecond,
		30 * time.Millisecond,
		40 * time.Millisecond,
		50 * time.Millisecond,
	}, map[int]int{200: 5}, start, end)

	if summary.Name != "feed_hot" {
		t.Fatalf("Name = %q", summary.Name)
	}
	if summary.Requests != 5 {
		t.Fatalf("Requests = %d", summary.Requests)
	}
	if summary.P50MS != 30 {
		t.Fatalf("P50MS = %f", summary.P50MS)
	}
	if summary.P95MS != 50 {
		t.Fatalf("P95MS = %f", summary.P95MS)
	}
	if summary.RPS != 2.5 {
		t.Fatalf("RPS = %f", summary.RPS)
	}
	if summary.StatusCodes[200] != 5 {
		t.Fatalf("status code count = %d", summary.StatusCodes[200])
	}
}
