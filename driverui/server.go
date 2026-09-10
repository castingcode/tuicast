// Package driverui provides the read-only TUICast Inspector HTTP application.
package driverui

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/castingcode/tuicast"
	"github.com/castingcode/tuicast/driver"
	"github.com/castingcode/tuicast/workbench"
)

// SnapshotSource supplies detached driver state and session screens.
type SnapshotSource interface {
	DriverSnapshot() driver.Snapshot
	SessionScreens(context.Context, uint64) (<-chan tuicast.Screen, error)
}

// Server serves the TUICast Inspector.
type Server struct {
	http      *http.Server
	source    SnapshotSource
	workbench *workbench.Manager
}

// Option configures an optional Inspector capability.
type Option func(*Server) error

// WithWorkbench enables interactive recording and replay routes.
func WithWorkbench(manager *workbench.Manager) Option {
	return func(server *Server) error {
		if manager == nil {
			return fmt.Errorf("configuring TUICast Workbench: manager is required")
		}
		server.workbench = manager
		return nil
	}
}

//go:embed assets/*
var assets embed.FS

// New creates a read-only Inspector server.
func New(source SnapshotSource, options ...Option) (*Server, error) {
	if source == nil {
		return nil, fmt.Errorf("creating TUICast Inspector: snapshot source is required")
	}
	server := &Server{source: source}
	for _, option := range options {
		if option == nil {
			return nil, fmt.Errorf("creating TUICast Inspector: option is required")
		}
		if err := option(server); err != nil {
			return nil, err
		}
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", server.health)
	mux.HandleFunc("GET /readyz", server.ready)
	mux.HandleFunc("GET /api/state", server.state)
	mux.HandleFunc("GET /api/capabilities", server.capabilities)
	mux.HandleFunc("GET /api/sessions/{id}/screens", server.screens)
	mux.HandleFunc("GET /sessions/{id}", server.application)
	if server.workbench != nil {
		server.registerWorkbenchRoutes(mux)
	}
	mux.HandleFunc("GET /", server.application)
	static, err := fs.Sub(assets, "assets")
	if err != nil {
		return nil, fmt.Errorf("loading TUICast Inspector assets: %w", err)
	}
	mux.Handle("GET /assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(static))))
	server.http = &http.Server{
		Handler:           securityHeaders(mux),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	return server, nil
}

func (s *Server) capabilities(response http.ResponseWriter, _ *http.Request) {
	response.Header().Set("Content-Type", "application/json")
	response.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(response).Encode(map[string]bool{"workbench": s.workbench != nil})
}

// Serve serves requests until Shutdown is called.
func (s *Server) Serve(listener net.Listener) error {
	if listener == nil {
		return fmt.Errorf("serving TUICast Inspector: listener is required")
	}
	if err := s.http.Serve(listener); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("serving TUICast Inspector: %w", err)
	}
	return nil
}

// Shutdown gracefully stops the Inspector.
func (s *Server) Shutdown(ctx context.Context) error {
	err := s.http.Shutdown(ctx)
	if s.workbench != nil {
		s.workbench.Close()
	}
	if err != nil {
		return fmt.Errorf("shutting down TUICast Inspector: %w", err)
	}
	return nil
}

func (s *Server) health(response http.ResponseWriter, _ *http.Request) {
	response.Header().Set("Content-Type", "text/plain; charset=utf-8")
	response.Header().Set("Cache-Control", "no-store")
	response.WriteHeader(http.StatusOK)
	_, _ = response.Write([]byte("ok\n"))
}

func (s *Server) ready(response http.ResponseWriter, _ *http.Request) {
	response.Header().Set("Content-Type", "application/json")
	response.Header().Set("Cache-Control", "no-store")
	snapshot := s.source.DriverSnapshot()
	status := http.StatusOK
	if snapshot.State != "running" {
		status = http.StatusServiceUnavailable
	}
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(map[string]string{"state": snapshot.State})
}

func (s *Server) state(response http.ResponseWriter, _ *http.Request) {
	response.Header().Set("Content-Type", "application/json")
	response.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(response).Encode(s.source.DriverSnapshot())
}

func (s *Server) screens(response http.ResponseWriter, request *http.Request) {
	id, err := parseSessionID(request.PathValue("id"))
	if err != nil {
		http.NotFound(response, request)
		return
	}
	screens, err := s.source.SessionScreens(request.Context(), id)
	if err != nil {
		http.NotFound(response, request)
		return
	}
	flusher, ok := response.(http.Flusher)
	if !ok {
		http.Error(response, "streaming is unavailable", http.StatusInternalServerError)
		return
	}
	response.Header().Set("Content-Type", "text/event-stream")
	response.Header().Set("Cache-Control", "no-store")
	response.Header().Set("Connection", "keep-alive")
	response.WriteHeader(http.StatusOK)
	flusher.Flush()

	keepalive := time.NewTicker(15 * time.Second)
	defer keepalive.Stop()
	for {
		select {
		case <-request.Context().Done():
			return
		case screen, open := <-screens:
			if !open {
				_, _ = fmt.Fprint(response, "event: end\ndata: {}\n\n")
				flusher.Flush()
				return
			}
			encoded, err := json.Marshal(makeScreen(screen))
			if err != nil {
				return
			}
			if _, err := fmt.Fprintf(response, "event: screen\ndata: %s\n\n", encoded); err != nil {
				return
			}
			flusher.Flush()
		case <-keepalive.C:
			if _, err := fmt.Fprint(response, ": keepalive\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func (s *Server) application(response http.ResponseWriter, request *http.Request) {
	if request.URL.Path != "/" {
		if _, err := parseSessionID(request.PathValue("id")); err != nil {
			http.NotFound(response, request)
			return
		}
	}
	content, err := assets.ReadFile("assets/index.html")
	if err != nil {
		http.Error(response, "Inspector application is unavailable", http.StatusInternalServerError)
		return
	}
	response.Header().Set("Content-Type", "text/html; charset=utf-8")
	response.Header().Set("Cache-Control", "no-store")
	_, _ = response.Write(content)
}

func parseSessionID(value string) (uint64, error) {
	id, err := strconv.ParseUint(value, 10, 64)
	if err != nil || id == 0 {
		return 0, fmt.Errorf("parsing session ID %q", value)
	}
	return id, nil
}

type screenResult struct {
	Width    int          `json:"width"`
	Height   int          `json:"height"`
	Cells    []cellResult `json:"cells"`
	Cursor   cursorResult `json:"cursor"`
	Revision uint64       `json:"revision"`
	Text     string       `json:"text"`
}

type cellResult struct {
	Text       string `json:"text"`
	Width      int    `json:"width"`
	Foreground int16  `json:"foreground"`
	Background int16  `json:"background"`
	Attributes uint16 `json:"attributes"`
}

type cursorResult struct {
	Column  int  `json:"column"`
	Row     int  `json:"row"`
	Visible bool `json:"visible"`
}

func makeScreen(screen tuicast.Screen) screenResult {
	cells := make([]cellResult, len(screen.Cells))
	for index, cell := range screen.Cells {
		cells[index] = cellResult{
			Text:       cell.Text,
			Width:      cell.Width,
			Foreground: int16(cell.Foreground),
			Background: int16(cell.Background),
			Attributes: uint16(cell.Attributes),
		}
	}
	return screenResult{
		Width:  screen.Width,
		Height: screen.Height,
		Cells:  cells,
		Cursor: cursorResult{
			Column:  screen.Cursor.Column,
			Row:     screen.Cursor.Row,
			Visible: screen.Cursor.Visible,
		},
		Revision: screen.Revision,
		Text:     screen.Text(),
	}
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Security-Policy", "default-src 'self'; connect-src 'self'; script-src 'self'; style-src 'self'")
		response.Header().Set("Referrer-Policy", "no-referrer")
		response.Header().Set("X-Content-Type-Options", "nosniff")
		response.Header().Set("X-Frame-Options", "DENY")
		if strings.HasPrefix(request.URL.Path, "/api/") {
			response.Header().Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(response, request)
	})
}
