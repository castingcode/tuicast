package tuicast

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

func TestDriverHelperProcess(t *testing.T) {
	if os.Getenv("TUICAST_SDK_HELPER") != "1" {
		return
	}
	decoder := json.NewDecoder(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)
	var writerMu sync.Mutex
	write := func(response map[string]any) error {
		writerMu.Lock()
		defer writerMu.Unlock()
		return encoder.Encode(response)
	}
	for {
		var request struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      uint64          `json:"id"`
			Method  string          `json:"method"`
			Params  json.RawMessage `json:"params"`
		}
		if err := decoder.Decode(&request); err != nil {
			os.Exit(0)
		}
		response := map[string]any{"jsonrpc": "2.0", "id": request.ID}
		switch request.Method {
		case "driver.ping":
			response["result"] = map[string]any{"protocolVersion": "1"}
		case "driver.shutdown":
			response["result"] = map[string]any{"shuttingDown": true}
			_ = write(response)
			os.Exit(0)
		case "connection.open":
			response["result"] = map[string]any{"connectionId": 11}
		case "connection.close", "session.close":
			response["result"] = map[string]any{"closed": true}
		case "session.open":
			var params struct {
				Terminal string `json:"terminal"`
				Width    int    `json:"width"`
				Height   int    `json:"height"`
			}
			_ = json.Unmarshal(request.Params, &params)
			if params.Terminal != "vt220" || params.Width != 80 || params.Height != 24 {
				response["error"] = map[string]any{"code": -32602, "message": "unexpected session defaults"}
			} else {
				response["result"] = map[string]any{"sessionId": 22}
			}
		case "session.send":
			response["result"] = map[string]any{"bytesSent": 1}
		case "session.press":
			response["result"] = map[string]any{"sent": true}
		case "session.resize":
			response["result"] = map[string]any{"resized": true}
		case "session.screen":
			response["result"] = helperScreen()
			go func(id uint64, outgoing map[string]any) {
				if id%2 == 0 {
					time.Sleep(5 * time.Millisecond)
				}
				if err := write(outgoing); err != nil {
					fmt.Fprintln(os.Stderr, err)
				}
			}(request.ID, response)
			continue
		case "session.wait", "session.waitForIdle":
			if request.Method == "session.wait" && string(request.Params) != "" {
				var params struct {
					Matcher matcherSpec `json:"matcher"`
				}
				_ = json.Unmarshal(request.Params, &params)
				if params.Matcher.Contains != nil && *params.Matcher.Contains == "MISSING" {
					response["error"] = map[string]any{
						"code": -32000, "message": "wait timed out",
						"data": map[string]any{"kind": "timeout", "expected": `screen containing "MISSING"`, "screen": helperScreen()},
					}
					break
				}
			}
			response["result"] = helperScreen()
		default:
			response["error"] = map[string]any{"code": -32601, "message": "method not found"}
		}
		if err := write(response); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}

func TestClientLifecycle(t *testing.T) {
	Convey("The client launches a driver and provides ergonomic session operations", t, func() {
		client, err := Launch(
			context.Background(),
			WithDriverPath(os.Args[0]),
			WithDriverArgs("-test.run=TestDriverHelperProcess"),
			WithDriverEnvironment("TUICAST_SDK_HELPER=1"),
			WithDefaultTimeout(5*time.Second),
		)
		So(err, ShouldBeNil)

		connection, err := client.Connect(context.Background(), Telnet{Address: "example.test:23"})
		So(err, ShouldBeNil)
		So(connection.ID(), ShouldEqual, uint64(11))
		session, err := connection.OpenSession(context.Background())
		So(err, ShouldBeNil)
		So(session.ID(), ShouldEqual, uint64(22))

		So(session.Type(context.Background(), "operator"), ShouldBeNil)
		So(session.Send(context.Background(), []byte{0, 1}), ShouldBeNil)
		So(session.Press(context.Background(), Enter), ShouldBeNil)
		So(session.Resize(context.Background(), 132, 24), ShouldBeNil)

		screen, err := session.Screen(context.Background())
		So(err, ShouldBeNil)
		So(screen.Contains("READY"), ShouldBeTrue)
		var waitGroup sync.WaitGroup
		concurrentErrors := make(chan error, 10)
		for range 10 {
			waitGroup.Add(1)
			go func() {
				defer waitGroup.Done()
				current, screenErr := session.Screen(context.Background())
				if screenErr != nil {
					concurrentErrors <- screenErr
					return
				}
				if current.Text() != "READY" {
					concurrentErrors <- fmt.Errorf("unexpected screen %q", current.Text())
				}
			}()
		}
		waitGroup.Wait()
		close(concurrentErrors)
		So(concurrentErrors, ShouldHaveLength, 0)
		screen, err = session.WaitForText(context.Background(), "READY", StableFor(10*time.Millisecond))
		So(err, ShouldBeNil)
		So(screen.Text(), ShouldEqual, "READY")
		_, err = session.WaitForTextGone(context.Background(), "LOADING")
		So(err, ShouldBeNil)
		_, err = session.WaitForIdle(context.Background(), IdleFor(10*time.Millisecond))
		So(err, ShouldBeNil)

		_, err = session.WaitForText(context.Background(), "MISSING")
		So(err, ShouldNotBeNil)
		So(IsTimeout(err), ShouldBeTrue)
		var waitErr *WaitError
		So(errors.As(err, &waitErr), ShouldBeTrue)
		So(waitErr.LastScreen.Text(), ShouldEqual, "READY")

		So(session.Close(context.Background()), ShouldBeNil)
		So(session.Close(context.Background()), ShouldBeNil)
		So(connection.Close(context.Background()), ShouldBeNil)
		So(client.Close(), ShouldBeNil)
		So(client.Close(), ShouldBeNil)
	})

	Convey("Typed SSH configuration rejects ambiguous authentication", t, func() {
		_, _, err := (SSH{Address: "example.test:22", Username: "operator", Password: "secret"}).driverParams(time.Second)
		So(err, ShouldNotBeNil)
		_, _, err = (SSH{
			Address:                  "example.test:22",
			Username:                 "operator",
			Password:                 "secret",
			KnownHostsFile:           "known_hosts",
			InsecureSkipHostKeyCheck: true,
		}).driverParams(time.Second)
		So(err, ShouldNotBeNil)
	})
}

func helperScreen() map[string]any {
	cells := make([]map[string]any, 5)
	for index, character := range "READY" {
		cells[index] = map[string]any{
			"text": string(character), "width": 1,
			"foreground": -1, "background": -1, "attributes": 0,
		}
	}
	return map[string]any{
		"width": 5, "height": 1, "revision": 3, "text": "READY", "cells": cells,
		"cursor": map[string]any{"column": 0, "row": 0, "visible": true},
	}
}
