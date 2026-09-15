package main

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestFeatures(t *testing.T) {
	Convey("The shared reference TUI features pass", t, func() {
		So(testSuite("progress").Run(), ShouldEqual, 0)
	})
}
