package main

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestServerFlags(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	Convey("SSH and Telnet listen flags are mutually exclusive", t, func() {
		err := run(context.Background(), []string{
			"--ssh-address", "127.0.0.1:0",
			"--telnet-address", "127.0.0.1:0",
		}, environment(nil), nil, io.Discard, logger)

		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldContainSubstring, "mutually exclusive")
	})

	Convey("SSH serving accepts an ephemeral listen address", t, func() {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		So(run(ctx, []string{"--ssh-address", "127.0.0.1:0"}, environment(map[string]string{
			sshPasswordVariable: "ssh-test-password",
			appPasswordVariable: "app-test-password",
		}), nil, io.Discard, logger), ShouldBeNil)
	})

	Convey("SSH serving requires an SSH password from the environment or flags", t, func() {
		err := run(context.Background(), []string{"--ssh-address", "127.0.0.1:0"}, environment(map[string]string{
			appPasswordVariable: "app-test-password",
		}), nil, io.Discard, logger)
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldContainSubstring, sshPasswordVariable)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		So(run(ctx, []string{"--ssh-address", "127.0.0.1:0", "--ssh-password", "ssh-test-password"}, environment(map[string]string{
			appPasswordVariable: "app-test-password",
		}), nil, io.Discard, logger), ShouldBeNil)
	})

	Convey("An application password longer than the login field is rejected at startup", t, func() {
		err := run(context.Background(), []string{"--telnet-address", "127.0.0.1:0"}, environment(map[string]string{
			appPasswordVariable: strings.Repeat("x", 25),
		}), nil, io.Discard, logger)
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldContainSubstring, "at most 24 characters")
	})

	Convey("Every mode requires an application login password", t, func() {
		for _, arguments := range [][]string{
			{"--ssh-address", "127.0.0.1:0"},
			{"--telnet-address", "127.0.0.1:0"},
			{},
		} {
			err := run(context.Background(), arguments, environment(map[string]string{
				sshPasswordVariable: "ssh-test-password",
			}), nil, io.Discard, logger)
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, appPasswordVariable)
		}
	})

	Convey("Telnet serving accepts an ephemeral listen address", t, func() {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		So(run(ctx, []string{"--telnet-address", "127.0.0.1:0", "--app-password", "app-test-password"}, environment(nil), nil, io.Discard, logger), ShouldBeNil)
	})

	Convey("SSH users can be loaded from a JSON file", t, func() {
		path := filepath.Join(t.TempDir(), "users.json")
		So(os.WriteFile(path, []byte(`{"operator":"ssh-test-password","supervisor":"warehouse"}`), 0o600), ShouldBeNil)

		users, err := sshUsers(path, "ignored", "ignored")

		So(err, ShouldBeNil)
		So(users, ShouldResemble, map[string]string{
			"operator":   "ssh-test-password",
			"supervisor": "warehouse",
		})
	})

	Convey("An empty SSH users file is rejected", t, func() {
		path := filepath.Join(t.TempDir(), "users.json")
		So(os.WriteFile(path, []byte(`{}`), 0o600), ShouldBeNil)

		_, err := sshUsers(path, "ignored", "ignored")

		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldContainSubstring, "at least one user")
	})
}

func TestInformationFlags(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	for _, flagName := range []string{"-help", "-h"} {
		Convey("The "+flagName+" flag prints usage and exits without serving", t, func() {
			var output bytes.Buffer
			So(run(context.Background(), []string{flagName}, environment(nil), nil, &output, logger), ShouldBeNil)
			So(output.String(), ShouldStartWith, "Usage: reference-tui")
			So(output.String(), ShouldContainSubstring, "-ssh-address")
			So(output.String(), ShouldContainSubstring, "-version")
			So(output.String(), ShouldContainSubstring, appPasswordVariable)
		})
	}

	for _, flagName := range []string{"-version", "-v"} {
		Convey("The "+flagName+" flag prints build metadata and exits without serving", t, func() {
			var output bytes.Buffer
			So(run(context.Background(), []string{flagName}, environment(nil), nil, &output, logger), ShouldBeNil)
			So(output.String(), ShouldEqual, "reference-tui dev (commit none, built unknown)\n")
		})
	}
}

func environment(values map[string]string) func(string) string {
	return func(name string) string { return values[name] }
}
