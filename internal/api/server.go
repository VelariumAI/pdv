package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/velariumai/pdv/internal/config"
	"github.com/velariumai/pdv/internal/download"
)

type Server struct {
	addr         string
	engine       *download.Engine
	cfg          *config.Config
	httpServer   *http.Server
	staticDir    string
	shutdownHook func(context.Context)
}

func NewServer(addr string, engine *download.Engine, cfg *config.Config) *Server {
	s := &Server{
		addr:   addr,
		engine: engine,
		cfg:    cfg,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/status", s.handleStatus)
	mux.HandleFunc("/api/v1/health", s.handleHealth)
	mux.HandleFunc("/", s.handleRoot)
	s.httpServer = &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	return s
}

func (s *Server) SetStaticDir(dir string) {
	s.staticDir = dir
}

func (s *Server) SetShutdownHook(fn func(context.Context)) {
	s.shutdownHook = fn
}

func (s *Server) Start(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		errCh <- s.httpServer.ListenAndServe()
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errCh:
		if err == nil || err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}

func (s *Server) Stop(ctx context.Context) error {
	if s.shutdownHook != nil {
		s.shutdownHook = nil
	}
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("pdv api\n"))
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data":    map[string]any{"status": "ok"},
		"error":   "",
	})
}

func (s *Server) handleStatus(w http.ResponseWriter, _ *http.Request) {
	workers := 0
	if s.engine != nil {
		workers = len(s.engine.Workers())
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data": map[string]any{
			"status":  "ok",
			"workers": workers,
			"addr":    s.addr,
		},
		"error": "",
	})
}

func writeJSON(w http.ResponseWriter, status int, payload map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		http.Error(w, fmt.Sprintf(`{"success":false,"data":{},"error":"%v"}`, err), http.StatusInternalServerError)
	}
}
