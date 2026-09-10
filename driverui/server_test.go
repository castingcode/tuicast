package driverui

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/castingcode/tuicast"
	"github.com/castingcode/tuicast/driver"
	. "github.com/smartystreets/goconvey/convey"
)

func TestInspector(t *testing.T) {
	Convey("Health, readiness, and state expose detached operational data", t, func() {
		source := &fakeSource{snapshot: populatedSnapshot()}
		server, err := New(source)
		So(err, ShouldBeNil)

		health := request(server, http.MethodGet, "/healthz")
		So(health.Code, ShouldEqual, http.StatusOK)
		So(health.Body.String(), ShouldEqual, "ok\n")

		ready := request(server, http.MethodGet, "/readyz")
		So(ready.Code, ShouldEqual, http.StatusOK)
		So(ready.Body.String(), ShouldContainSubstring, `"state":"running"`)

		state := request(server, http.MethodGet, "/api/state")
		So(state.Code, ShouldEqual, http.StatusOK)
		So(state.Header().Get("Cache-Control"), ShouldEqual, "no-store")
		So(state.Body.String(), ShouldContainSubstring, `"protocol":"ssh"`)
		So(state.Body.String(), ShouldContainSubstring, `"terminal":"vt220"`)
	})

	Convey("Readiness fails after the driver closes while liveness remains healthy", t, func() {
		source := &fakeSource{snapshot: driver.Snapshot{State: "closed", Connections: []driver.ConnectionSnapshot{}, Sessions: []driver.SessionSnapshot{}}}
		server, err := New(source)
		So(err, ShouldBeNil)

		So(request(server, http.MethodGet, "/healthz").Code, ShouldEqual, http.StatusOK)
		So(request(server, http.MethodGet, "/readyz").Code, ShouldEqual, http.StatusServiceUnavailable)
	})

	Convey("A screen stream begins with the current complete screen", t, func() {
		screens := make(chan tuicast.Screen, 1)
		screens <- tuicast.Screen{
			Width: 2, Height: 1, Revision: 7,
			Cells:  []tuicast.Cell{{Text: "O", Width: 1}, {Text: "K", Width: 1}},
			Cursor: tuicast.Cursor{Column: 1, Row: 0, Visible: true},
		}
		close(screens)
		source := &fakeSource{snapshot: populatedSnapshot(), screens: screens}
		server, err := New(source)
		So(err, ShouldBeNil)

		response := request(server, http.MethodGet, "/api/sessions/9/screens")
		So(response.Code, ShouldEqual, http.StatusOK)
		So(response.Header().Get("Content-Type"), ShouldEqual, "text/event-stream")
		So(response.Body.String(), ShouldContainSubstring, "event: screen\n")
		So(response.Body.String(), ShouldContainSubstring, `"revision":7`)
		So(response.Body.String(), ShouldContainSubstring, `"text":"OK"`)
	})

	Convey("Unknown sessions and unsupported methods are rejected", t, func() {
		server, err := New(&fakeSource{snapshot: populatedSnapshot(), screenErr: fmt.Errorf("missing")})
		So(err, ShouldBeNil)

		So(request(server, http.MethodGet, "/api/sessions/99/screens").Code, ShouldEqual, http.StatusNotFound)
		method := request(server, http.MethodPost, "/api/state")
		So(method.Code, ShouldEqual, http.StatusMethodNotAllowed)
		So(method.Header().Get("Allow"), ShouldEqual, "GET, HEAD")
	})

	Convey("The embedded application is served with browser security headers", t, func() {
		server, err := New(&fakeSource{snapshot: populatedSnapshot()})
		So(err, ShouldBeNil)

		response := request(server, http.MethodGet, "/")
		So(response.Code, ShouldEqual, http.StatusOK)
		So(response.Body.String(), ShouldContainSubstring, "TUICast Inspector")
		So(response.Header().Get("Content-Security-Policy"), ShouldContainSubstring, "default-src 'self'")
		So(request(server, http.MethodGet, "/unrelated").Code, ShouldEqual, http.StatusNotFound)
	})
}

type fakeSource struct {
	snapshot  driver.Snapshot
	screens   <-chan tuicast.Screen
	screenErr error
}

func (s *fakeSource) DriverSnapshot() driver.Snapshot {
	return s.snapshot
}

func (s *fakeSource) SessionScreens(_ context.Context, _ uint64) (<-chan tuicast.Screen, error) {
	return s.screens, s.screenErr
}

func populatedSnapshot() driver.Snapshot {
	return driver.Snapshot{
		State: "running",
		Connections: []driver.ConnectionSnapshot{{
			ID: 3, Protocol: "ssh", Address: "warehouse.example:22",
		}},
		Sessions: []driver.SessionSnapshot{{
			ID: 9, ConnectionID: 3, Terminal: "vt220", Width: 80, Height: 24, Revision: 7, State: "active",
		}},
	}
}

func request(server *Server, method, path string) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	server.http.Handler.ServeHTTP(response, httptest.NewRequest(method, path, strings.NewReader("")))
	return response
}
