package driver

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/castingcode/tuicast"
	"github.com/castingcode/tuicast/memory"
	"github.com/castingcode/tuicast/vt220"
	. "github.com/smartystreets/goconvey/convey"
)

func TestDriverProtocol(t *testing.T) {
	Convey("Ping and shutdown are served through JSON-RPC", t, func() {
		server := newTestDriver()
		input := strings.NewReader(
			`{"jsonrpc":"2.0","id":1,"method":"driver.ping"}` + "\n" +
				`{"jsonrpc":"2.0","id":2,"method":"driver.shutdown"}` + "\n",
		)
		var output bytes.Buffer

		So(server.Run(input, &output), ShouldBeNil)

		decoder := json.NewDecoder(&output)
		responses := make(map[string]map[string]any)
		for decoder.More() {
			var decoded struct {
				ID     json.RawMessage `json:"id"`
				Result map[string]any  `json:"result"`
			}
			So(decoder.Decode(&decoded), ShouldBeNil)
			responses[string(decoded.ID)] = decoded.Result
		}
		So(responses, ShouldHaveLength, 2)
		So(responses["1"]["protocolVersion"], ShouldEqual, "1")
		So(responses["2"]["shuttingDown"], ShouldEqual, true)
	})

	Convey("Connection and session methods drive a terminal end to end", t, func() {
		server := newTestDriver()
		defer server.Close()

		opened, responseErr := server.openConnection(raw(map[string]any{
			"protocol": "memory",
			"address":  "reference",
		}))
		So(responseErr, ShouldBeNil)
		connectionID := opened.(map[string]uint64)["connectionId"]

		opened, responseErr = server.openSession(raw(map[string]any{
			"connectionId": connectionID,
			"terminal":     string(tuicast.ProfileVT220),
			"width":        20,
			"height":       3,
		}))
		So(responseErr, ShouldBeNil)
		sessionID := opened.(map[string]uint64)["sessionId"]

		result, responseErr := server.send(raw(map[string]any{
			"sessionId": sessionID,
			"base64":    "UkVBRFk=",
		}))
		So(responseErr, ShouldBeNil)
		So(result.(map[string]int)["bytesSent"], ShouldEqual, 5)

		ready := "READY"
		result, responseErr = server.wait(raw(map[string]any{
			"sessionId":           sessionID,
			"matcher":             Matcher{Contains: &ready},
			"timeoutMilliseconds": 1000,
			"stableMilliseconds":  10,
		}))
		So(responseErr, ShouldBeNil)
		So(result.(screenResult).Text, ShouldContainSubstring, "READY")

		result, responseErr = server.resize(raw(map[string]any{
			"sessionId": sessionID,
			"width":     30,
			"height":    4,
		}))
		So(responseErr, ShouldBeNil)
		So(result.(map[string]bool)["resized"], ShouldBeTrue)

		result, responseErr = server.screen(raw(map[string]uint64{"sessionId": sessionID}))
		So(responseErr, ShouldBeNil)
		So(result.(screenResult).Width, ShouldEqual, 30)
		So(result.(screenResult).Height, ShouldEqual, 4)

		_, responseErr = server.closeConnection(raw(map[string]uint64{"connectionId": connectionID}))
		So(responseErr, ShouldBeNil)
		_, responseErr = server.screen(raw(map[string]uint64{"sessionId": sessionID}))
		So(responseErr.Code, ShouldEqual, -32602)
	})

	Convey("Subscriptions emit the current screen as a notification", t, func() {
		server := newTestDriver()
		defer server.Close()
		opened, responseErr := server.openConnection(raw(map[string]any{"protocol": "memory", "address": "reference"}))
		So(responseErr, ShouldBeNil)
		connectionID := opened.(map[string]uint64)["connectionId"]
		opened, responseErr = server.openSession(raw(map[string]any{
			"connectionId": connectionID,
			"terminal":     string(tuicast.ProfileVT220),
			"width":        10,
			"height":       2,
		}))
		So(responseErr, ShouldBeNil)
		sessionID := opened.(map[string]uint64)["sessionId"]

		var output synchronizedBuffer
		writer := newRPCWriter(&output)
		server.serve(request{
			JSONRPC: "2.0",
			ID:      json.RawMessage("1"),
			Method:  "session.subscribe",
			Params:  raw(map[string]uint64{"sessionId": sessionID}),
		}, writer, make(chan struct{}, 1))

		deadline := time.Now().Add(time.Second)
		for !strings.Contains(output.String(), `"method":"session.screen"`) && time.Now().Before(deadline) {
			time.Sleep(time.Millisecond)
		}
		So(output.String(), ShouldContainSubstring, `"method":"session.screen"`)
		So(strings.Index(output.String(), `"id":1`), ShouldBeLessThan, strings.Index(output.String(), `"method":"session.screen"`))
		_, responseErr = server.unsubscribe(raw(map[string]uint64{"subscriptionId": 1}))
		So(responseErr, ShouldBeNil)
	})

	Convey("Modified keys and terminal events cross the driver protocol", t, func() {
		server := newTestDriver()
		defer server.Close()
		opened, responseErr := server.openConnection(raw(map[string]any{"protocol": "memory", "address": "reference"}))
		So(responseErr, ShouldBeNil)
		connectionID := opened.(map[string]uint64)["connectionId"]
		opened, responseErr = server.openSession(raw(map[string]any{
			"connectionId": connectionID,
			"terminal":     string(tuicast.ProfileVT220),
			"width":        20,
			"height":       2,
			"answerback":   "TUICAST",
		}))
		So(responseErr, ShouldBeNil)
		sessionID := opened.(map[string]uint64)["sessionId"]

		_, responseErr = server.press(raw(map[string]any{
			"sessionId": sessionID,
			"key":       "c",
			"modifiers": []string{"Control", "Alt"},
		}))
		So(responseErr, ShouldBeNil)
		_, responseErr = server.press(raw(map[string]any{
			"sessionId": sessionID,
			"key":       "c",
			"modifiers": []string{"Super"},
		}))
		So(responseErr.Code, ShouldEqual, -32602)

		var output synchronizedBuffer
		writer := newRPCWriter(&output)
		server.serve(request{
			JSONRPC: "2.0",
			ID:      json.RawMessage("1"),
			Method:  "session.subscribeEvents",
			Params:  raw(map[string]uint64{"sessionId": sessionID}),
		}, writer, make(chan struct{}, 1))
		_, responseErr = server.send(raw(map[string]any{
			"sessionId": sessionID,
			"base64":    "BwU=",
		}))
		So(responseErr, ShouldBeNil)

		deadline := time.Now().Add(time.Second)
		for (!strings.Contains(output.String(), `"type":"bell"`) || !strings.Contains(output.String(), `"type":"enquiry"`)) && time.Now().Before(deadline) {
			time.Sleep(time.Millisecond)
		}
		So(output.String(), ShouldContainSubstring, `"type":"bell"`)
		So(output.String(), ShouldContainSubstring, `"type":"enquiry"`)
		So(output.String(), ShouldContainSubstring, `"data":"TUICAST"`)
	})
}

func newTestDriver() *Server {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server, err := New(
		logger,
		func(ConnectionOptions) (tuicast.Connector, error) {
			return memory.NewConnector(func(_ context.Context, remote io.ReadWriteCloser) {
				_, _ = io.Copy(remote, remote)
			})
		},
		func(_ string, width, height int) (tuicast.Terminal, error) {
			return vt220.New(width, height)
		},
	)
	So(err, ShouldBeNil)
	return server
}

func raw(value any) json.RawMessage {
	encoded, err := json.Marshal(value)
	So(err, ShouldBeNil)
	return encoded
}

type synchronizedBuffer struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (b *synchronizedBuffer) Write(data []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.Write(data)
}

func (b *synchronizedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.String()
}
