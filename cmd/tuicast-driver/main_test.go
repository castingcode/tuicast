package main

import (
	"bytes"
	"io"
	"log/slog"
	"net"
	"regexp"
	"testing"

	"github.com/castingcode/tuicast"
	"github.com/castingcode/tuicast/driver"
	. "github.com/smartystreets/goconvey/convey"
)

func TestComposition(t *testing.T) {
	Convey("Concrete terminal profiles are composed by name", t, func() {
		created, err := terminal(string(tuicast.ProfileVT220), 80, 24)
		So(err, ShouldBeNil)
		So(created.Profile(), ShouldEqual, tuicast.ProfileVT220)

		created, err = terminal(string(tuicast.ProfileXTerm), 80, 24)
		So(err, ShouldBeNil)
		So(created.Profile(), ShouldEqual, tuicast.ProfileXTerm)

		created, err = terminal("unknown", 80, 24)
		So(err, ShouldNotBeNil)
		So(created, ShouldBeNil)
	})

	Convey("SSH requires one explicit host-key verification strategy", t, func() {
		callback, err := makeHostKeyCallback(driver.ConnectionOptions{})
		So(err, ShouldNotBeNil)
		So(callback, ShouldBeNil)

		callback, err = makeHostKeyCallback(driver.ConnectionOptions{InsecureSkipHostKeyCheck: true})
		So(err, ShouldBeNil)
		So(callback, ShouldNotBeNil)

		callback, err = makeHostKeyCallback(driver.ConnectionOptions{
			HostKeyFingerprint:       "SHA256:example",
			InsecureSkipHostKeyCheck: true,
		})
		So(err, ShouldNotBeNil)
		So(callback, ShouldBeNil)
	})

	Convey("The Inspector uses a resolved ephemeral loopback port and shuts down with the driver", t, func() {
		var logs bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&logs, nil))
		input := bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"driver.shutdown"}` + "\n")

		err := run([]string{"-ui-address", "127.0.0.1:0"}, input, io.Discard, logger)
		So(err, ShouldBeNil)
		matches := regexp.MustCompile(`url=http://([^ ]+)`).FindStringSubmatch(logs.String())
		So(matches, ShouldHaveLength, 2)
		connection, err := net.Dial("tcp", matches[1])
		So(err, ShouldNotBeNil)
		So(connection, ShouldBeNil)
	})

	Convey("Inspector loopback detection distinguishes exposed listeners", t, func() {
		So(isLoopbackAddress("127.0.0.1:0"), ShouldBeTrue)
		So(isLoopbackAddress("[::1]:0"), ShouldBeTrue)
		So(isLoopbackAddress("0.0.0.0:8080"), ShouldBeFalse)
	})
}
