//go:build linux || darwin

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"testing"

	"github.com/castingcode/tuicast/driver"
	. "github.com/smartystreets/goconvey/convey"
)

func TestPTYComposition(t *testing.T) {
	Convey("A pty connection runs a local program through the JSON-RPC driver", t, func() {
		input, requests := io.Pipe()
		responses, output := io.Pipe()
		finished := make(chan error, 1)
		go func() {
			finished <- run(nil, input, output, slog.New(slog.NewTextHandler(io.Discard, nil)))
			output.Close()
		}()
		lines := bufio.NewScanner(responses)
		lines.Buffer(nil, 4<<20) // screen results include every cell
		call := func(id int, method string, params any) map[string]any {
			encoded, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
			So(err, ShouldBeNil)
			_, err = fmt.Fprintf(requests, "%s\n", encoded)
			So(err, ShouldBeNil)
			So(lines.Scan(), ShouldBeTrue)
			var response map[string]any
			So(json.Unmarshal(lines.Bytes(), &response), ShouldBeNil)
			So(response["error"], ShouldBeNil)
			return response["result"].(map[string]any)
		}

		connection := call(1, "connection.open", map[string]any{
			"protocol":    "pty",
			"command":     "/bin/sh",
			"arguments":   []string{"-c", `printf '%s %s\n' "$GREETING" "$TERM"; read line; echo "GOT $line"`},
			"environment": map[string]string{"GREETING": "tight-lines"},
		})
		session := call(2, "session.open", map[string]any{
			"connectionId": connection["connectionId"], "terminal": "xterm-256color", "width": 80, "height": 24,
		})
		call(3, "session.wait", map[string]any{
			"sessionId": session["sessionId"], "matcher": map[string]any{"contains": "tight-lines xterm-256color"},
			"timeoutMilliseconds": 5000,
		})
		call(4, "session.send", map[string]any{"sessionId": session["sessionId"], "text": "lpn 7\r"})
		call(5, "session.wait", map[string]any{
			"sessionId": session["sessionId"], "matcher": map[string]any{"contains": "GOT lpn 7"},
			"timeoutMilliseconds": 5000,
		})
		state := call(6, "session.waitForEnd", map[string]any{"sessionId": session["sessionId"], "timeoutMilliseconds": 5000})
		So(state["state"], ShouldEqual, "ended")
		So(state["error"], ShouldBeNil)

		So(requests.Close(), ShouldBeNil)
		So(<-finished, ShouldBeNil)
	})

	Convey("pty and network options cannot be mixed", t, func() {
		_, err := connector(driver.ConnectionOptions{Protocol: "pty", Command: "/bin/sh", Address: "localhost:23"})
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldContainSubstring, "address is not valid for pty")

		_, err = connector(driver.ConnectionOptions{Protocol: "pty", Command: "/bin/sh", Password: "secret"})
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldContainSubstring, "SSH options are not valid for pty")

		_, err = connector(driver.ConnectionOptions{Protocol: "telnet", Address: "localhost:23", Command: "/bin/sh"})
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldContainSubstring, "valid only for pty")

		_, err = connector(driver.ConnectionOptions{Protocol: "pty"})
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldContainSubstring, "command is required")

		_, err = connector(driver.ConnectionOptions{Protocol: "pty", Command: "/bin/sh", Environment: map[string]string{"A=B": "c"}})
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldContainSubstring, "environment variable name")
	})
}
