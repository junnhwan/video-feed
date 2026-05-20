package benchkit

import (
	"math"
	"sort"
	"time"
)

type ScenarioSummary struct {
	Name        string         `json:"name"`
	Requests    int            `json:"requests"`
	Errors      int            `json:"errors"`
	DurationMS  float64        `json:"duration_ms"`
	RPS         float64        `json:"rps"`
	AvgMS       float64        `json:"avg_ms"`
	MinMS       float64        `json:"min_ms"`
	MaxMS       float64        `json:"max_ms"`
	P50MS       float64        `json:"p50_ms"`
	P90MS       float64        `json:"p90_ms"`
	P95MS       float64        `json:"p95_ms"`
	P99MS       float64        `json:"p99_ms"`
	StatusCodes map[int]int    `json:"status_codes"`
	Notes       map[string]any `json:"notes,omitempty"`
}

func SummarizeDurations(name string, durations []time.Duration, statusCodes map[int]int, start time.Time, end time.Time) ScenarioSummary {
	summary := ScenarioSummary{
		Name:        name,
		Requests:    len(durations),
		DurationMS:  durationMillis(end.Sub(start)),
		StatusCodes: cloneStatusCodes(statusCodes),
	}
	for code, count := range statusCodes {
		if code < 200 || code >= 400 {
			summary.Errors += count
		}
	}
	elapsed := end.Sub(start).Seconds()
	if elapsed > 0 {
		summary.RPS = float64(summary.Requests) / elapsed
	}
	if len(durations) == 0 {
		return summary
	}

	values := append([]time.Duration(nil), durations...)
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })

	var total time.Duration
	for _, duration := range values {
		total += duration
	}
	summary.AvgMS = durationMillis(total / time.Duration(len(values)))
	summary.MinMS = durationMillis(values[0])
	summary.MaxMS = durationMillis(values[len(values)-1])
	summary.P50MS = durationMillis(percentile(values, 0.50))
	summary.P90MS = durationMillis(percentile(values, 0.90))
	summary.P95MS = durationMillis(percentile(values, 0.95))
	summary.P99MS = durationMillis(percentile(values, 0.99))
	return summary
}

func percentile(values []time.Duration, p float64) time.Duration {
	if len(values) == 0 {
		return 0
	}
	if p <= 0 {
		return values[0]
	}
	if p >= 1 {
		return values[len(values)-1]
	}
	index := int(math.Ceil(float64(len(values))*p)) - 1
	if index < 0 {
		index = 0
	}
	if index >= len(values) {
		index = len(values) - 1
	}
	return values[index]
}

func durationMillis(duration time.Duration) float64 {
	return float64(duration.Microseconds()) / 1000
}

func cloneStatusCodes(statusCodes map[int]int) map[int]int {
	if len(statusCodes) == 0 {
		return map[int]int{}
	}
	cloned := make(map[int]int, len(statusCodes))
	for code, count := range statusCodes {
		cloned[code] = count
	}
	return cloned
}
