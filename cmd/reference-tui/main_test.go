package main

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestServerFlags(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	Convey("SSH and Telnet listen flags are mutually exclusive", t, func() {
		err := run(context.Background(), []string{
			"--ssh-address", "127.0.0.1:0",
			"--telnet-address", "127.0.0.1:0",
		}, nil, io.Discard, logger)

		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldContainSubstring, "mutually exclusive")
	})

	Convey("SSH serving accepts an ephemeral listen address", t, func() {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		So(run(ctx, []string{"--ssh-address", "127.0.0.1:0"}, nil, io.Discard, logger), ShouldBeNil)
	})

	Convey("Telnet serving accepts an ephemeral listen address", t, func() {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		So(run(ctx, []string{"--telnet-address", "127.0.0.1:0"}, nil, io.Discard, logger), ShouldBeNil)
	})

	Convey("SSH users can be loaded from a JSON file", t, func() {
		path := filepath.Join(t.TempDir(), "users.json")
		So(os.WriteFile(path, []byte(`{"operator":"casting","supervisor":"warehouse"}`), 0o600), ShouldBeNil)

		users, err := sshUsers(path, "ignored", "ignored")

		So(err, ShouldBeNil)
		So(users, ShouldResemble, map[string]string{
			"operator":   "casting",
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
