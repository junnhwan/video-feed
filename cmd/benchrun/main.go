package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"video-feed/internal/benchkit"
	"video-feed/internal/config"
	rediscache "video-feed/internal/middleware/redis"

	goredis "github.com/redis/go-redis/v9"
)

type runOptions struct {
	configPath  string
	manifest    string
	baseURL     string
	outPath     string
	requests    int
	concurrency int
	warmup      int
	timeout     time.Duration
	scenarios   string
}

type authToken struct {
	AccountID uint
	Username  string
	Token     string
}

type scenarioResult struct {
	Status   int
	Latency  time.Duration
	Error    error
	Response []byte
}

type benchReport struct {
	RunID       string                     `json:"run_id"`
	BaseURL     string                     `json:"base_url"`
	StartedAt   time.Time                  `json:"started_at"`
	FinishedAt  time.Time                  `json:"finished_at"`
	Requests    int                        `json:"requests"`
	Concurrency int                        `json:"concurrency"`
	Summaries   []benchkit.ScenarioSummary `json:"summaries"`
	Comparisons []comparison               `json:"comparisons,omitempty"`
}

type comparison struct {
	Name           string  `json:"name"`
	Baseline       string  `json:"baseline"`
	Candidate      string  `json:"candidate"`
	BaselineP95MS  float64 `json:"baseline_p95_ms"`
	CandidateP95MS float64 `json:"candidate_p95_ms"`
	P95ChangePct   float64 `json:"p95_change_pct"`
	BaselineRPS    float64 `json:"baseline_rps"`
	CandidateRPS   float64 `json:"candidate_rps"`
	RPSChangePct   float64 `json:"rps_change_pct"`
}

func main() {
	opts := parseOptions()
	ctx := context.Background()
	manifest, err := benchkit.ReadManifest(opts.manifest)
	if err != nil {
		log.Fatalf("read manifest: %v", err)
	}
	if opts.baseURL == "" {
		opts.baseURL = manifest.BaseURL
	}
	opts.baseURL = strings.TrimRight(opts.baseURL, "/")

	cfg, err := config.Load(opts.configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	cache := rediscache.NewFromConfig(cfg.Redis)
	if err := cache.Ping(ctx); err != nil {
		log.Printf("redis setup disabled: %v", err)
		_ = cache.Close()
		cache = nil
	}
	if cache != nil {
		defer cache.Close()
	}

	client := &http.Client{Timeout: opts.timeout}
	if err := smoke(ctx, client, opts.baseURL, manifest); err != nil {
		log.Fatalf("smoke failed: %v", err)
	}
	tokens := loginUsers(ctx, client, opts.baseURL, manifest)
	if len(tokens) == 0 {
		log.Fatalf("no auth tokens available")
	}

	report := benchReport{
		RunID:       manifest.RunID,
		BaseURL:     opts.baseURL,
		StartedAt:   time.Now().UTC(),
		Requests:    opts.requests,
		Concurrency: opts.concurrency,
	}

	selected := splitScenarios(opts.scenarios)
	for _, name := range selected {
		switch name {
		case "popularity-db":
			if cache != nil {
				clearHotKeys(ctx, cache, manifest.HotAsOf)
			}
			report.Summaries = append(report.Summaries, runPopularity(ctx, client, opts, manifest, "popularity_db_fallback"))
		case "popularity-hot":
			if cache != nil {
				seedHotKeys(ctx, cache, manifest)
			}
			report.Summaries = append(report.Summaries, runPopularity(ctx, client, opts, manifest, "popularity_redis_hot"))
		case "detail-cold":
			if cache != nil {
				clearDetailKeys(ctx, cache, manifest.Videos)
			}
			time.Sleep(4 * time.Second)
			report.Summaries = append(report.Summaries, runDetail(ctx, client, opts, manifest, "detail_cold", minInt(opts.requests, len(manifest.Videos))))
		case "detail-hot":
			report.Summaries = append(report.Summaries, runDetail(ctx, client, opts, manifest, "detail_hot", minInt(opts.requests, len(manifest.Videos))))
		case "latest":
			report.Summaries = append(report.Summaries, runLatest(ctx, client, opts))
		case "comment":
			report.Summaries = append(report.Summaries, runComment(ctx, client, opts, manifest, tokens))
		default:
			log.Printf("unknown scenario %q, skipped", name)
		}
	}
	report.FinishedAt = time.Now().UTC()
	report.Comparisons = buildComparisons(report.Summaries)

	if err := writeReport(opts.outPath, report); err != nil {
		log.Fatalf("write report: %v", err)
	}
	log.Printf("bench report written: %s", opts.outPath)
}

func parseOptions() runOptions {
	opts := runOptions{}
	flag.StringVar(&opts.configPath, "config", "configs/config.yaml", "config file path")
	flag.StringVar(&opts.manifest, "manifest", "", "seed manifest path")
	flag.StringVar(&opts.baseURL, "base-url", "", "API base URL override")
	flag.StringVar(&opts.outPath, "out", "", "report output path")
	flag.IntVar(&opts.requests, "requests", 300, "requests per scenario")
	flag.IntVar(&opts.concurrency, "concurrency", 20, "concurrent workers")
	flag.IntVar(&opts.warmup, "warmup", 20, "warmup requests before measured run")
	flag.DurationVar(&opts.timeout, "timeout", 5*time.Second, "HTTP request timeout")
	flag.StringVar(&opts.scenarios, "scenarios", "popularity-db,popularity-hot,detail-cold,detail-hot,latest,comment", "comma-separated scenarios")
	flag.Parse()
	if opts.manifest == "" {
		log.Fatal("-manifest is required")
	}
	if opts.outPath == "" {
		opts.outPath = filepath.Join("bench", "results", "bench-"+time.Now().Format("20060102-150405")+".json")
	}
	if opts.requests <= 0 {
		log.Fatal("requests must be > 0")
	}
	if opts.concurrency <= 0 {
		log.Fatal("concurrency must be > 0")
	}
	return opts
}

func smoke(ctx context.Context, client *http.Client, baseURL string, manifest benchkit.Manifest) error {
	if len(manifest.Videos) == 0 {
		return fmt.Errorf("manifest has no videos")
	}
	if len(manifest.Users) == 0 {
		return fmt.Errorf("manifest has no users")
	}
	status, _, err := doJSON(ctx, client, http.MethodGet, baseURL+"/health", "", nil)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("GET /health status=%d", status)
	}
	status, _, err = doJSON(ctx, client, http.MethodPost, baseURL+"/feed/listLatest", "", map[string]any{"limit": 5})
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("POST /feed/listLatest status=%d", status)
	}
	status, _, err = doJSON(ctx, client, http.MethodPost, baseURL+"/video/getDetail", "", map[string]any{"id": manifest.Videos[0].ID})
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("POST /video/getDetail status=%d", status)
	}
	return nil
}

func loginUsers(ctx context.Context, client *http.Client, baseURL string, manifest benchkit.Manifest) []authToken {
	tokens := make([]authToken, 0, len(manifest.Users))
	for _, user := range manifest.Users {
		status, body, err := doJSON(ctx, client, http.MethodPost, baseURL+"/account/login", "", map[string]any{
			"username": user.Username,
			"password": manifest.Password,
		})
		if err != nil || status != http.StatusOK {
			log.Printf("login skipped username=%s status=%d err=%v", user.Username, status, err)
			continue
		}
		var resp struct {
			Token     string `json:"token"`
			AccountID uint   `json:"account_id"`
			Username  string `json:"username"`
		}
		if err := json.Unmarshal(body, &resp); err != nil || resp.Token == "" {
			log.Printf("login decode skipped username=%s err=%v", user.Username, err)
			continue
		}
		tokens = append(tokens, authToken{AccountID: resp.AccountID, Username: resp.Username, Token: resp.Token})
	}
	return tokens
}

func runPopularity(ctx context.Context, client *http.Client, opts runOptions, manifest benchkit.Manifest, name string) benchkit.ScenarioSummary {
	request := func(ctx context.Context, i int) scenarioResult {
		offset := (i % 10) * 20
		status, body, latency, err := postTimed(ctx, client, opts.baseURL+"/feed/listByPopularity", "", map[string]any{
			"limit":  20,
			"as_of":  manifest.HotAsOf,
			"offset": offset,
		})
		return scenarioResult{Status: status, Latency: latency, Error: err, Response: body}
	}
	warmup(ctx, opts.warmup, request)
	return runScenario(ctx, name, opts.requests, opts.concurrency, request)
}

func runDetail(ctx context.Context, client *http.Client, opts runOptions, manifest benchkit.Manifest, name string, requests int) benchkit.ScenarioSummary {
	request := func(ctx context.Context, i int) scenarioResult {
		videoID := manifest.Videos[i%len(manifest.Videos)].ID
		status, body, latency, err := postTimed(ctx, client, opts.baseURL+"/video/getDetail", "", map[string]any{"id": videoID})
		return scenarioResult{Status: status, Latency: latency, Error: err, Response: body}
	}
	warmup(ctx, minInt(opts.warmup, requests), request)
	return runScenario(ctx, name, requests, opts.concurrency, request)
}

func runLatest(ctx context.Context, client *http.Client, opts runOptions) benchkit.ScenarioSummary {
	request := func(ctx context.Context, i int) scenarioResult {
		status, body, latency, err := postTimed(ctx, client, opts.baseURL+"/feed/listLatest", "", map[string]any{"limit": 20})
		return scenarioResult{Status: status, Latency: latency, Error: err, Response: body}
	}
	warmup(ctx, opts.warmup, request)
	return runScenario(ctx, "feed_latest", opts.requests, opts.concurrency, request)
}

func runComment(ctx context.Context, client *http.Client, opts runOptions, manifest benchkit.Manifest, tokens []authToken) benchkit.ScenarioSummary {
	request := func(ctx context.Context, i int) scenarioResult {
		token := tokens[i%len(tokens)]
		videoID := manifest.Videos[i%len(manifest.Videos)].ID
		status, body, latency, err := postTimed(ctx, client, opts.baseURL+"/comment/publish", token.Token, map[string]any{
			"video_id": videoID,
			"content":  fmt.Sprintf("bench comment %s %d", time.Now().Format("150405.000"), i),
		})
		return scenarioResult{Status: status, Latency: latency, Error: err, Response: body}
	}
	warmup(ctx, minInt(opts.warmup, len(tokens)), request)
	return runScenario(ctx, "comment_write", opts.requests, opts.concurrency, request)
}

func runScenario(ctx context.Context, name string, requests int, concurrency int, fn func(context.Context, int) scenarioResult) benchkit.ScenarioSummary {
	jobs := make(chan int)
	results := make(chan scenarioResult, requests)
	var wg sync.WaitGroup
	workerCount := minInt(concurrency, requests)
	start := time.Now()
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				results <- fn(ctx, job)
			}
		}()
	}
	for i := 0; i < requests; i++ {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	close(results)
	end := time.Now()

	durations := make([]time.Duration, 0, requests)
	statusCodes := make(map[int]int)
	errors := 0
	for result := range results {
		if result.Error != nil {
			errors++
			statusCodes[0]++
			continue
		}
		durations = append(durations, result.Latency)
		statusCodes[result.Status]++
	}
	summary := benchkit.SummarizeDurations(name, durations, statusCodes, start, end)
	summary.Errors += errors
	if errors > 0 {
		summary.Notes = map[string]any{"transport_errors": errors}
	}
	log.Printf("%s requests=%d p95=%.2fms rps=%.2f statuses=%v", name, summary.Requests, summary.P95MS, summary.RPS, summary.StatusCodes)
	return summary
}

func warmup(ctx context.Context, n int, fn func(context.Context, int) scenarioResult) {
	for i := 0; i < n; i++ {
		_ = fn(ctx, i)
	}
}

func doJSON(ctx context.Context, client *http.Client, method string, url string, token string, payload any) (int, []byte, error) {
	status, body, _, err := doJSONTimed(ctx, client, method, url, token, payload)
	return status, body, err
}

func postTimed(ctx context.Context, client *http.Client, url string, token string, payload any) (int, []byte, time.Duration, error) {
	return doJSONTimed(ctx, client, http.MethodPost, url, token, payload)
}

func doJSONTimed(ctx context.Context, client *http.Client, method string, url string, token string, payload any) (int, []byte, time.Duration, error) {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return 0, nil, 0, err
		}
		body = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return 0, nil, 0, err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	start := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(start)
	if err != nil {
		return 0, nil, latency, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return resp.StatusCode, nil, latency, err
	}
	return resp.StatusCode, respBody, latency, nil
}

func clearHotKeys(ctx context.Context, cache *rediscache.Client, asOf int64) {
	if cache == nil || asOf == 0 {
		return
	}
	base := time.Unix(asOf, 0).UTC().Truncate(time.Minute)
	for i := 0; i < 60; i++ {
		window := base.Add(-time.Duration(i) * time.Minute)
		_ = cache.Del(ctx, cache.Key("hot:video:1m:%s", window.Format("200601021504")))
	}
	_ = cache.Del(ctx, cache.Key("hot:video:merge:1m:%s", base.Format("200601021504")))
}

func seedHotKeys(ctx context.Context, cache *rediscache.Client, manifest benchkit.Manifest) {
	if cache == nil || manifest.HotAsOf == 0 {
		return
	}
	base := time.Unix(manifest.HotAsOf, 0).UTC().Truncate(time.Minute)
	_ = cache.Del(ctx, cache.Key("hot:video:merge:1m:%s", base.Format("200601021504")))
	members := make([]goredis.Z, 0, len(manifest.Videos))
	for _, item := range manifest.Videos {
		members = append(members, goredis.Z{
			Score:  float64(item.Popularity),
			Member: fmt.Sprintf("%d", item.ID),
		})
	}
	if err := cache.ZAdd(ctx, cache.Key("hot:video:1m:%s", base.Format("200601021504")), members...); err != nil {
		log.Printf("seed hot keys failed: %v", err)
	}
}

func clearDetailKeys(ctx context.Context, cache *rediscache.Client, videos []benchkit.ManifestVideo) {
	if cache == nil {
		return
	}
	limit := minInt(1000, len(videos))
	for i := 0; i < limit; i++ {
		_ = cache.Del(ctx, cache.Key("video:detail:id=%d", videos[i].ID))
	}
}

func buildComparisons(summaries []benchkit.ScenarioSummary) []comparison {
	byName := make(map[string]benchkit.ScenarioSummary, len(summaries))
	for _, summary := range summaries {
		byName[summary.Name] = summary
	}
	pairs := []struct {
		name      string
		baseline  string
		candidate string
	}{
		{name: "hot feed Redis snapshot vs DB fallback", baseline: "popularity_db_fallback", candidate: "popularity_redis_hot"},
		{name: "video detail hot cache vs cold read", baseline: "detail_cold", candidate: "detail_hot"},
	}
	out := make([]comparison, 0, len(pairs))
	for _, pair := range pairs {
		base, ok1 := byName[pair.baseline]
		candidate, ok2 := byName[pair.candidate]
		if !ok1 || !ok2 {
			continue
		}
		out = append(out, comparison{
			Name:           pair.name,
			Baseline:       pair.baseline,
			Candidate:      pair.candidate,
			BaselineP95MS:  base.P95MS,
			CandidateP95MS: candidate.P95MS,
			P95ChangePct:   percentChange(base.P95MS, candidate.P95MS),
			BaselineRPS:    base.RPS,
			CandidateRPS:   candidate.RPS,
			RPSChangePct:   percentChange(base.RPS, candidate.RPS),
		})
	}
	return out
}

func percentChange(baseline float64, candidate float64) float64 {
	if baseline == 0 {
		return 0
	}
	return (candidate - baseline) / baseline * 100
}

func writeReport(path string, report benchReport) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, append(payload, '\n'), 0o644); err != nil {
		return err
	}
	return writeMarkdownReport(strings.TrimSuffix(path, filepath.Ext(path))+".md", report)
}

func writeMarkdownReport(path string, report benchReport) error {
	var b strings.Builder
	fmt.Fprintf(&b, "# Benchmark Report\n\n")
	fmt.Fprintf(&b, "- run_id: `%s`\n", report.RunID)
	fmt.Fprintf(&b, "- base_url: `%s`\n", report.BaseURL)
	fmt.Fprintf(&b, "- requests_per_scenario: `%d`\n", report.Requests)
	fmt.Fprintf(&b, "- concurrency: `%d`\n\n", report.Concurrency)
	fmt.Fprintf(&b, "## Scenarios\n\n")
	fmt.Fprintf(&b, "| scenario | requests | errors | rps | avg_ms | p50_ms | p95_ms | p99_ms | statuses |\n")
	fmt.Fprintf(&b, "|---|---:|---:|---:|---:|---:|---:|---:|---|\n")
	for _, item := range report.Summaries {
		fmt.Fprintf(&b, "| `%s` | %d | %d | %.2f | %.2f | %.2f | %.2f | %.2f | `%v` |\n",
			item.Name, item.Requests, item.Errors, item.RPS, item.AvgMS, item.P50MS, item.P95MS, item.P99MS, item.StatusCodes)
	}
	if len(report.Comparisons) > 0 {
		fmt.Fprintf(&b, "\n## Comparisons\n\n")
		fmt.Fprintf(&b, "| comparison | baseline | candidate | p95_change | rps_change |\n")
		fmt.Fprintf(&b, "|---|---|---|---:|---:|\n")
		for _, item := range report.Comparisons {
			fmt.Fprintf(&b, "| %s | `%s` %.2fms | `%s` %.2fms | %.2f%% | %.2f%% |\n",
				item.Name, item.Baseline, item.BaselineP95MS, item.Candidate, item.CandidateP95MS, item.P95ChangePct, item.RPSChangePct)
		}
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func splitScenarios(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func init() {
	log.SetOutput(os.Stdout)
}
