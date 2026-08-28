package main

import (
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
}
