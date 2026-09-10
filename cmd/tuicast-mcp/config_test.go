package main

import (
	"bytes"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestProfileConfiguration(t *testing.T) {
	Convey("A strict Telnet profile loads without credentials", t, func() {
		path := writeConfig(t, `{
  "profiles": {
    "warehouse": {
      "description": "integration environment",
      "protocol": "telnet",
      "address": "127.0.0.1:2323",
      "terminal": "vt220",
      "width": 80,
      "height": 24
    }
  }
}`)
		profiles, err := loadProfiles(path, discardLogger())
		So(err, ShouldBeNil)
		So(profiles, ShouldHaveLength, 1)
		So(profiles[0].Name, ShouldEqual, "warehouse")
		So(profiles[0].Protocol, ShouldEqual, "telnet")
		So(profiles[0].Connector, ShouldNotBeNil)
	})

	Convey("Unknown configuration fields are rejected", t, func() {
		path := writeConfig(t, `{"profiles":{"warehouse":{"protocol":"telnet","address":"host:23","terminal":"vt220","width":80,"height":24,"host":"unexpected"}}}`)
		profiles, err := loadProfiles(path, discardLogger())
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldContainSubstring, "unknown field")
		So(profiles, ShouldBeNil)
	})

	Convey("SSH credentials come from named environment variables and are never included in errors", t, func() {
		secret := "do-not-leak-this-password"
		So(os.Setenv("TUICAST_TEST_PASSWORD", secret), ShouldBeNil)
		Reset(func() { _ = os.Unsetenv("TUICAST_TEST_PASSWORD") })
		path := writeConfig(t, `{"profiles":{"warehouse":{"protocol":"ssh","address":"host:22","terminal":"xterm-256color","width":100,"height":30,"ssh":{"username":"automation","passwordEnv":"TUICAST_TEST_PASSWORD","hostKeyFingerprint":"SHA256:expected"}}}}`)
		profiles, err := loadProfiles(path, discardLogger())
		So(err, ShouldBeNil)
		So(profiles, ShouldHaveLength, 1)
		So(profiles[0].Connector, ShouldNotBeNil)

		So(os.Unsetenv("TUICAST_TEST_PASSWORD"), ShouldBeNil)
		profiles, err = loadProfiles(path, discardLogger())
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldNotContainSubstring, secret)
		So(profiles, ShouldBeNil)
	})

	Convey("SSH requires one explicit host-key policy", t, func() {
		So(os.Setenv("TUICAST_TEST_PASSWORD", "secret"), ShouldBeNil)
		Reset(func() { _ = os.Unsetenv("TUICAST_TEST_PASSWORD") })
		path := writeConfig(t, `{"profiles":{"warehouse":{"protocol":"ssh","address":"host:22","terminal":"vt220","width":80,"height":24,"ssh":{"username":"automation","passwordEnv":"TUICAST_TEST_PASSWORD"}}}}`)
		profiles, err := loadProfiles(path, discardLogger())
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldContainSubstring, "exactly one SSH host-key")
		So(profiles, ShouldBeNil)
	})

	Convey("The command requires a profile configuration", t, func() {
		err := run(t.Context(), nil, io.NopCloser(bytes.NewReader(nil)), nopWriteCloser{io.Discard}, discardLogger())
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldContainSubstring, "-config is required")
	})
}

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "mcp.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type nopWriteCloser struct {
	io.Writer
}

func (nopWriteCloser) Close() error { return nil }
