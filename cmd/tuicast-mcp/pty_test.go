//go:build linux || darwin

package main

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/castingcode/tuicast"
	"github.com/castingcode/tuicast/mcpserver"
	. "github.com/smartystreets/goconvey/convey"
)

// writeProgram writes an executable shell script into the configuration's
// directory and returns its file name.
func writeProgram(directory, script string) string {
	So(os.WriteFile(filepath.Join(directory, "rf-client"), []byte("#!/bin/sh\n"+script), 0o755), ShouldBeNil)
	return "rf-client"
}

func TestPTYProfiles(t *testing.T) {
	Convey("A PTY profile runs its program with the configured environment and a secret resolved at connection time", t, func() {
		So(os.Setenv("TUICAST_TEST_SECRET_HELPER", "print"), ShouldBeNil)
		Reset(func() { _ = os.Unsetenv("TUICAST_TEST_SECRET_HELPER") })
		path := writeConfig(t, `{"profiles":{"local":{
  "description": "local RF client",
  "protocol": "pty",
  "terminal": "vt220",
  "width": 80,
  "height": 24,
  "pty": {
    "command": "./rf-client",
    "arguments": ["DC1"],
    "workingDirectory": "site",
    "environment": {"WMS_ENV": "test"},
    "environmentFromSecrets": {"WMS_TOKEN": "TUICAST_TEST_PTY_TOKEN"}
  }
}}}`)
		directory := filepath.Dir(path)
		So(os.Mkdir(filepath.Join(directory, "site"), 0o755), ShouldBeNil)
		writeProgram(directory, `printf 'pid=%s site=%s env=%s term=%s\ntoken=[%s]\ndir=%s\n' "$$" "$1" "$WMS_ENV" "$TERM" "$WMS_TOKEN" "$(basename "$(pwd)")"
read line
echo "GOT $line"
sleep 60
`)
		profiles, err := loadProfiles(path, os.Args[0], true, discardLogger())
		So(err, ShouldBeNil)
		So(profiles, ShouldHaveLength, 1)
		So(profiles[0].Protocol, ShouldEqual, "pty")

		service, err := mcpserver.New(discardLogger(), profiles, terminal)
		So(err, ShouldBeNil)
		defer service.Close()
		So(service.Profiles(), ShouldResemble, []mcpserver.ProfileInfo{{
			Name: "local", Description: "local RF client", Protocol: "pty", Terminal: "vt220", Width: 80, Height: 24,
		}})

		ctx := context.Background()
		connection, err := service.Connect(ctx, "local")
		So(err, ShouldBeNil)
		session, err := service.OpenSession(ctx, connection.ConnectionID)
		So(err, ShouldBeNil)
		screen, err := service.WaitForText(ctx, session.SessionID, "dir=site", 5*time.Second)
		So(err, ShouldBeNil)
		So(screen.Text, ShouldContainSubstring, "site=DC1 env=test term=vt220")
		So(screen.Text, ShouldContainSubstring, "token=[ TUICAST_TEST_PTY_TOKEN secret]")
		pid := regexp.MustCompile(`pid=(\d+)`).FindStringSubmatch(screen.Text)
		So(pid, ShouldHaveLength, 2)

		So(service.Type(session.SessionID, "lpn 7", ""), ShouldBeNil)
		So(service.Press(session.SessionID, tuicast.KeyEnter), ShouldBeNil)
		_, err = service.WaitForText(ctx, session.SessionID, "GOT lpn 7", 5*time.Second)
		So(err, ShouldBeNil)

		_, err = service.CloseConnection(connection.ConnectionID)
		So(err, ShouldBeNil)
		id, err := strconv.Atoi(pid[1])
		So(err, ShouldBeNil)
		So(syscall.Kill(id, 0), ShouldNotBeNil)
	})

	Convey("PTY profile settings are validated at startup", t, func() {
		cases := []struct {
			name    string
			profile string
			message string
		}{
			{"address", `"address":"host:23","pty":{"command":"./rf-client"}`, "address is not valid for pty"},
			{"missing settings", ``, "pty settings are required"},
			{"ssh settings", `"pty":{"command":"./rf-client"},"ssh":{"username":"u"}`, "ssh settings are not valid for pty"},
			{"missing program", `"pty":{"command":"./missing"}`, "pty command"},
			{"TERM", `"pty":{"command":"./rf-client","environment":{"TERM":"xterm"}}`, "TERM is set from the profile's terminal"},
			{"duplicate variable", `"pty":{"command":"./rf-client","environment":{"WMS_TOKEN":"x"},"environmentFromSecrets":{"WMS_TOKEN":"Y"}}`, "set by both"},
		}
		for _, testCase := range cases {
			Convey("Rejects "+testCase.name, func() {
				prefix := `{"profiles":{"local":{"protocol":"pty","terminal":"vt220","width":80,"height":24`
				if testCase.profile != "" {
					prefix += ","
				}
				path := writeConfig(t, prefix+testCase.profile+`}}}`)
				writeProgram(filepath.Dir(path), "exit 0\n")
				profiles, err := loadProfiles(path, "", true, discardLogger())
				So(err, ShouldNotBeNil)
				So(err.Error(), ShouldContainSubstring, testCase.message)
				So(profiles, ShouldBeNil)
			})
		}

		Convey("Rejects pty settings on a network profile", func() {
			path := writeConfig(t, `{"profiles":{"remote":{"protocol":"telnet","address":"host:23","terminal":"vt220","width":80,"height":24,"pty":{"command":"rf-client"}}}}`)
			_, err := loadProfiles(path, "", true, discardLogger())
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "valid only for protocol pty")
		})
	})
}
