package driverui

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/castingcode/tuicast"
	"github.com/castingcode/tuicast/workbench"
)

func (s *Server) registerWorkbenchRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /workbench/sessions/{id}", s.application)
	mux.HandleFunc("GET /api/workbench/sessions/{id}/recording", s.workbenchRecording)
	mux.HandleFunc("POST /api/workbench/sessions/{id}/recording/start", s.startWorkbenchRecording)
	mux.HandleFunc("POST /api/workbench/sessions/{id}/recording/stop", s.stopWorkbenchRecording)
	mux.HandleFunc("POST /api/workbench/sessions/{id}/type", s.workbenchType)
	mux.HandleFunc("POST /api/workbench/sessions/{id}/press", s.workbenchPress)
	mux.HandleFunc("POST /api/workbench/sessions/{id}/assert", s.workbenchAssert)
	mux.HandleFunc("POST /api/workbench/sessions/{id}/replay", s.workbenchReplay)
}

func (s *Server) startWorkbenchRecording(response http.ResponseWriter, request *http.Request) {
	if !sameOrigin(request) {
		http.Error(response, "cross-origin Workbench requests are forbidden", http.StatusForbidden)
		return
	}
	id, ok := workbenchSessionID(response, request)
	if !ok {
		return
	}
	var metadata *workbench.Session
	for _, session := range s.source.DriverSnapshot().Sessions {
		if session.ID == id {
			if session.State != "active" {
				writeWorkbenchError(response, fmt.Errorf("starting Workbench recording: session %d has ended", id))
				return
			}
			metadata = &workbench.Session{ID: id, Terminal: session.Terminal, Width: session.Width, Height: session.Height}
			break
		}
	}
	if metadata == nil {
		http.NotFound(response, request)
		return
	}
	recording, err := s.workbench.Start(*metadata)
	writeWorkbenchResult(response, recording, true, err)
}

func (s *Server) stopWorkbenchRecording(response http.ResponseWriter, request *http.Request) {
	if !sameOrigin(request) {
		http.Error(response, "cross-origin Workbench requests are forbidden", http.StatusForbidden)
		return
	}
	id, ok := workbenchSessionID(response, request)
	if !ok {
		return
	}
	recording, err := s.workbench.Stop(id)
	writeWorkbenchResult(response, recording, false, err)
}

func (s *Server) workbenchRecording(response http.ResponseWriter, request *http.Request) {
	id, ok := workbenchSessionID(response, request)
	if !ok {
		return
	}
	recording, active, err := s.workbench.Recording(id)
	writeWorkbenchResult(response, recording, active, err)
}

func (s *Server) workbenchType(response http.ResponseWriter, request *http.Request) {
	if !sameOrigin(request) {
		http.Error(response, "cross-origin Workbench requests are forbidden", http.StatusForbidden)
		return
	}
	id, ok := workbenchSessionID(response, request)
	if !ok {
		return
	}
	var params struct {
		Text      string `json:"text"`
		Parameter string `json:"parameter,omitempty"`
	}
	if !decodeWorkbenchRequest(response, request, &params) {
		return
	}
	recording, err := s.workbench.Type(id, params.Text, params.Parameter)
	writeWorkbenchResult(response, recording, true, err)
}

func (s *Server) workbenchPress(response http.ResponseWriter, request *http.Request) {
	if !sameOrigin(request) {
		http.Error(response, "cross-origin Workbench requests are forbidden", http.StatusForbidden)
		return
	}
	id, ok := workbenchSessionID(response, request)
	if !ok {
		return
	}
	var params struct {
		Key       string   `json:"key"`
		Modifiers []string `json:"modifiers,omitempty"`
	}
	if !decodeWorkbenchRequest(response, request, &params) {
		return
	}
	modifiers, err := workbenchModifiers(params.Modifiers)
	if err != nil {
		writeWorkbenchError(response, err)
		return
	}
	recording, err := s.workbench.Press(id, tuicast.Key(params.Key), modifiers...)
	writeWorkbenchResult(response, recording, true, err)
}

func (s *Server) workbenchAssert(response http.ResponseWriter, request *http.Request) {
	if !sameOrigin(request) {
		http.Error(response, "cross-origin Workbench requests are forbidden", http.StatusForbidden)
		return
	}
	id, ok := workbenchSessionID(response, request)
	if !ok {
		return
	}
	var params struct {
		Text                string `json:"text"`
		TimeoutMilliseconds int64  `json:"timeoutMilliseconds"`
	}
	if !decodeWorkbenchRequest(response, request, &params) {
		return
	}
	recording, err := s.workbench.AssertText(id, params.Text, time.Duration(params.TimeoutMilliseconds)*time.Millisecond)
	writeWorkbenchResult(response, recording, true, err)
}

func (s *Server) workbenchReplay(response http.ResponseWriter, request *http.Request) {
	if !sameOrigin(request) {
		http.Error(response, "cross-origin Workbench requests are forbidden", http.StatusForbidden)
		return
	}
	id, ok := workbenchSessionID(response, request)
	if !ok {
		return
	}
	var params struct {
		Parameters map[string]string `json:"parameters,omitempty"`
	}
	if !decodeWorkbenchRequest(response, request, &params) {
		return
	}
	if err := s.workbench.Replay(request.Context(), id, params.Parameters); err != nil {
		writeWorkbenchError(response, err)
		return
	}
	response.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(response).Encode(map[string]bool{"replayed": true})
}

func workbenchSessionID(response http.ResponseWriter, request *http.Request) (uint64, bool) {
	id, err := parseSessionID(request.PathValue("id"))
	if err != nil {
		http.NotFound(response, request)
		return 0, false
	}
	return id, true
}

func decodeWorkbenchRequest(response http.ResponseWriter, request *http.Request, destination any) bool {
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		http.Error(response, "invalid Workbench request", http.StatusBadRequest)
		return false
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		http.Error(response, "invalid Workbench request", http.StatusBadRequest)
		return false
	}
	return true
}

func writeWorkbenchResult(response http.ResponseWriter, recording workbench.Recording, active bool, err error) {
	if err != nil {
		writeWorkbenchError(response, err)
		return
	}
	response.Header().Set("Content-Type", "application/json")
	response.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(response).Encode(map[string]any{"active": active, "recording": recording})
}

func writeWorkbenchError(response http.ResponseWriter, err error) {
	response.Header().Set("Content-Type", "application/json")
	response.Header().Set("Cache-Control", "no-store")
	response.WriteHeader(http.StatusConflict)
	_ = json.NewEncoder(response).Encode(map[string]string{"error": err.Error()})
}

func sameOrigin(request *http.Request) bool {
	origin := request.Header.Get("Origin")
	return origin == "" || origin == "http://"+request.Host || origin == "https://"+request.Host
}

func workbenchModifiers(names []string) ([]tuicast.KeyModifier, error) {
	result := make([]tuicast.KeyModifier, len(names))
	for index, name := range names {
		switch name {
		case "Shift":
			result[index] = tuicast.ModifierShift
		case "Alt", "Meta":
			result[index] = tuicast.ModifierAlt
		case "Control":
			result[index] = tuicast.ModifierControl
		default:
			return nil, fmt.Errorf("unknown key modifier %q", name)
		}
	}
	return result, nil
}
