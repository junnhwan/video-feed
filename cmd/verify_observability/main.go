package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	_ "video-feed/internal/docs"
	apphttp "video-feed/internal/http"
	"video-feed/internal/observability"
)

func main() {
	observability.InitLogger("verify")
	defer observability.Sync()

	observability.CacheHitTotal.WithLabelValues("hot_rank", "redis_merge").Inc()
	observability.CacheMissTotal.WithLabelValues("video_entity", "local").Inc()
	observability.MQPublishTotal.WithLabelValues("video.exchange", "video.published", "success").Inc()
	observability.MQConsumeTotal.WithLabelValues("like.queue", "success").Inc()
	observability.RateLimitRejectTotal.WithLabelValues("login").Inc()
	observability.OutboxPendingGauge.Set(3)

	router := apphttp.NewRouterWithPublishers(nil, nil, nil)
	server := httptest.NewServer(router)
	defer server.Close()

	checks := []struct {
		name string
		path string
		want []string
	}{
		{"health", "/health", []string{`"status":"ok"`}},
		{"metrics", "/metrics", []string{
			"http_requests_total",
			"cache_hit_total",
			"cache_miss_total",
			"mq_publish_total",
			"mq_consume_total",
			"rate_limit_reject_total",
			"outbox_pending_messages",
		}},
		{"swagger-json", "/swagger/doc.json", []string{`"swagger": "2.0"`, `"/feed/listByPopularity"`}},
		{"swagger-ui", "/swagger/index.html", []string{"swagger"}},
	}

	allOK := true
	for _, ck := range checks {
		resp, err := http.Get(server.URL + ck.path)
		if err != nil {
			fmt.Printf("[FAIL] %s: %v\n", ck.name, err)
			allOK = false
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		bodyStr := string(body)
		miss := []string{}
		for _, w := range ck.want {
			if !strings.Contains(bodyStr, w) {
				miss = append(miss, w)
			}
		}
		if resp.StatusCode != 200 {
			fmt.Printf("[FAIL] %s: status=%d, body[:200]=%q\n", ck.name, resp.StatusCode, truncate(bodyStr, 200))
			allOK = false
			continue
		}
		if len(miss) > 0 {
			fmt.Printf("[FAIL] %s: missing tokens %v\n", ck.name, miss)
			allOK = false
			continue
		}
		fmt.Printf("[OK]   %s: status=200, contains all %d tokens\n", ck.name, len(ck.want))
	}

	time.Sleep(50 * time.Millisecond)
	if allOK {
		fmt.Println("\nAll observability endpoints verified.")
	} else {
		fmt.Println("\nSome checks failed.")
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
