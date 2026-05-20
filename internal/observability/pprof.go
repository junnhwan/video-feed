package observability

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/pprof"
	"time"
)

type PprofServer struct {
	name            string
	server          *http.Server
	shutdownTimeout time.Duration
}

func NewPprofMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	return mux
}

func NewPprofServer(name string, enabled bool, addr string) (*PprofServer, error) {
	if !enabled || addr == "" {
		return nil, nil
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("start %s pprof on %s: %w", name, addr, err)
	}
	server := &PprofServer{
		name:            name,
		shutdownTimeout: 3 * time.Second,
	}
	server.server = &http.Server{
		Addr:              addr,
		Handler:           NewPprofMux(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		log.Printf("%s pprof listening on %s", name, addr)
		if err := server.server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("%s pprof error: %v", name, err)
		}
	}()
	return server, nil
}

func (s *PprofServer) Close() error {
	if s == nil || s.server == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), s.shutdownTimeout)
	defer cancel()
	return s.server.Shutdown(ctx)
}
