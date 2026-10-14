package main

import (
	"bytes"
	"fmt"
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
		profiles, err := loadProfiles(path, "", discardLogger())
		So(err, ShouldBeNil)
		So(profiles, ShouldHaveLength, 1)
		So(profiles[0].Name, ShouldEqual, "warehouse")
		So(profiles[0].Protocol, ShouldEqual, "telnet")
		So(profiles[0].Connector, ShouldNotBeNil)
	})

	Convey("Unknown configuration fields are rejected", t, func() {
		path := writeConfig(t, `{"profiles":{"warehouse":{"protocol":"telnet","address":"host:23","terminal":"vt220","width":80,"height":24,"host":"unexpected"}}}`)
		profiles, err := loadProfiles(path, "", discardLogger())
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldContainSubstring, "unknown field")
		So(profiles, ShouldBeNil)
	})

	Convey("SSH credentials come from named environment variables and are never included in errors", t, func() {
		secret := "do-not-leak-this-password"
		So(os.Setenv("TUICAST_TEST_PASSWORD", secret), ShouldBeNil)
		Reset(func() { _ = os.Unsetenv("TUICAST_TEST_PASSWORD") })
		path := writeConfig(t, `{"profiles":{"warehouse":{"protocol":"ssh","address":"host:22","terminal":"xterm-256color","width":100,"height":30,"ssh":{"username":"automation","passwordEnv":"TUICAST_TEST_PASSWORD","hostKeyFingerprint":"SHA256:expected"}}}}`)
		profiles, err := loadProfiles(path, "", discardLogger())
		So(err, ShouldBeNil)
		So(profiles, ShouldHaveLength, 1)
		So(profiles[0].Connector, ShouldNotBeNil)

		So(os.Unsetenv("TUICAST_TEST_PASSWORD"), ShouldBeNil)
		profiles, err = loadProfiles(path, "", discardLogger())
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldNotContainSubstring, secret)
		So(profiles, ShouldBeNil)
	})

	Convey("SSH requires one explicit host-key policy", t, func() {
		So(os.Setenv("TUICAST_TEST_PASSWORD", "secret"), ShouldBeNil)
		Reset(func() { _ = os.Unsetenv("TUICAST_TEST_PASSWORD") })
		path := writeConfig(t, `{"profiles":{"warehouse":{"protocol":"ssh","address":"host:22","terminal":"vt220","width":80,"height":24,"ssh":{"username":"automation","passwordEnv":"TUICAST_TEST_PASSWORD"}}}}`)
		profiles, err := loadProfiles(path, "", discardLogger())
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

// TestMain lets the test binary act as a -secret-command helper when
// TUICAST_TEST_SECRET_HELPER selects a behavior.
func TestMain(m *testing.M) {
	name := os.Args[len(os.Args)-1]
	switch os.Getenv("TUICAST_TEST_SECRET_HELPER") {
	case "print":
		stdin, _ := io.ReadAll(os.Stdin)
		fmt.Print(" " + name + " secret" + string(stdin) + "\r\n")
	case "fail":
		fmt.Print("leaked-secret")
		fmt.Fprint(os.Stderr, "not signed in")
		os.Exit(3)
	case "empty":
	default:
		os.Exit(m.Run())
	}
}

func TestSecretSources(t *testing.T) {
	Convey("SSH secrets resolve from NAME, then NAME_FILE, then the secret command", t, func() {
		helper := os.Args[0]
		Reset(func() {
			_ = os.Unsetenv("TUICAST_TEST_SECRET")
			_ = os.Unsetenv("TUICAST_TEST_SECRET_FILE")
			_ = os.Unsetenv("TUICAST_TEST_SECRET_HELPER")
		})
		So(os.Setenv("TUICAST_TEST_SECRET_HELPER", "print"), ShouldBeNil)

		Convey("NAME is used before the secret command", func() {
			So(os.Setenv("TUICAST_TEST_SECRET", "from-environment"), ShouldBeNil)
			resolved, err := resolveSecret("warehouse", "SSH password", "TUICAST_TEST_SECRET", helper, discardLogger())
			So(err, ShouldBeNil)
			So(resolved.deferred, ShouldBeFalse)
			value, err := resolved.get()
			So(err, ShouldBeNil)
			So(value, ShouldEqual, "from-environment")
		})

		Convey("NAME_FILE is read when NAME is unset, removing only one trailing line ending", func() {
			path := filepath.Join(t.TempDir(), "password")
			So(os.WriteFile(path, []byte(" pass word \r\n"), 0o600), ShouldBeNil)
			So(os.Setenv("TUICAST_TEST_SECRET", ""), ShouldBeNil)
			So(os.Setenv("TUICAST_TEST_SECRET_FILE", path), ShouldBeNil)
			resolved, err := resolveSecret("warehouse", "SSH password", "TUICAST_TEST_SECRET", helper, discardLogger())
			So(err, ShouldBeNil)
			So(resolved.deferred, ShouldBeFalse)
			value, err := resolved.get()
			So(err, ShouldBeNil)
			So(value, ShouldEqual, " pass word ")
		})

		Convey("Setting both NAME and NAME_FILE is rejected", func() {
			So(os.Setenv("TUICAST_TEST_SECRET", "from-environment"), ShouldBeNil)
			So(os.Setenv("TUICAST_TEST_SECRET_FILE", "/unused"), ShouldBeNil)
			_, err := resolveSecret("warehouse", "SSH password", "TUICAST_TEST_SECRET", helper, discardLogger())
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "both set")
			So(err.Error(), ShouldNotContainSubstring, "from-environment")
		})

		Convey("The secret command receives the name at connection time and never reads MCP input", func() {
			reader, writer, err := os.Pipe()
			So(err, ShouldBeNil)
			_, err = writer.WriteString(" protocol")
			So(err, ShouldBeNil)
			So(writer.Close(), ShouldBeNil)
			stdin := os.Stdin
			os.Stdin = reader
			Reset(func() {
				os.Stdin = stdin
				_ = reader.Close()
			})

			resolved, err := resolveSecret("warehouse", "SSH password", "TUICAST_TEST_SECRET", helper, discardLogger())
			So(err, ShouldBeNil)
			So(resolved.deferred, ShouldBeTrue)
			value, err := resolved.get()
			So(err, ShouldBeNil)
			So(value, ShouldEqual, " TUICAST_TEST_SECRET secret")
		})

		Convey("A failing secret command does not block startup and reports stderr but not stdout", func() {
			So(os.Setenv("TUICAST_TEST_SECRET_HELPER", "fail"), ShouldBeNil)
			path := writeConfig(t, `{"profiles":{"warehouse":{"protocol":"ssh","address":"host:22","terminal":"vt220","width":80,"height":24,"ssh":{"username":"automation","passwordEnv":"TUICAST_TEST_SECRET","insecureSkipHostKeyCheck":true}}}}`)
			profiles, err := loadProfiles(path, helper, discardLogger())
			So(err, ShouldBeNil)
			So(profiles, ShouldHaveLength, 1)

			resolved, err := resolveSecret("warehouse", "SSH password", "TUICAST_TEST_SECRET", helper, discardLogger())
			So(err, ShouldBeNil)
			_, err = resolved.get()
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "exit status 3")
			So(err.Error(), ShouldContainSubstring, "not signed in")
			So(err.Error(), ShouldNotContainSubstring, "leaked-secret")
		})

		Convey("Empty secret command output is rejected", func() {
			So(os.Setenv("TUICAST_TEST_SECRET_HELPER", "empty"), ShouldBeNil)
			resolved, err := resolveSecret("warehouse", "SSH password", "TUICAST_TEST_SECRET", helper, discardLogger())
			So(err, ShouldBeNil)
			_, err = resolved.get()
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "empty")
		})

		Convey("Without any source, the error names every option", func() {
			_, err := resolveSecret("warehouse", "SSH password", "TUICAST_TEST_SECRET", "", discardLogger())
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, `"TUICAST_TEST_SECRET"`)
			So(err.Error(), ShouldContainSubstring, `"TUICAST_TEST_SECRET_FILE"`)
			So(err.Error(), ShouldContainSubstring, "-secret-command")
		})
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
