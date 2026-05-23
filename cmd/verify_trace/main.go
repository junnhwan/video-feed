package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	apphttp "video-feed/internal/http"
	"video-feed/internal/observability"
)

func main() {
	observability.InitLogger("verify-trace")
	defer observability.Sync()

	shutdown, err := observability.InitTracer("verify-trace")
	if err != nil {
		fmt.Println("init tracer err:", err)
		return
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = shutdown(ctx)
	}()

	router := apphttp.NewRouterWithPublishers(nil, nil, nil)
	server := httptest.NewServer(router)
	defer server.Close()

	for i := 0; i < 30; i++ {
		resp, err := http.Get(server.URL + "/health")
		if err != nil {
			fmt.Println("http get err:", err)
			return
		}
		resp.Body.Close()
	}
	fmt.Println("--- 30 requests sent, expect at least one span on stderr below ---")
	time.Sleep(1 * time.Second)
}
