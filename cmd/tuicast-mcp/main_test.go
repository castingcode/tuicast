package main

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

type writeCloser struct{ bytes.Buffer }

func (*writeCloser) Close() error { return nil }

func TestInformationFlags(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	for _, flagName := range []string{"-help", "-h"} {
		Convey("The "+flagName+" flag prints usage without requiring -config", t, func() {
			var output writeCloser
			So(run(context.Background(), []string{flagName}, io.NopCloser(&bytes.Buffer{}), &output, logger), ShouldBeNil)
			So(output.String(), ShouldStartWith, "Usage: tuicast-mcp")
			So(output.String(), ShouldContainSubstring, "-config")
			So(output.String(), ShouldContainSubstring, "-version")
		})
	}

	for _, flagName := range []string{"-version", "-v"} {
		Convey("The "+flagName+" flag prints build metadata without requiring -config", t, func() {
			var output writeCloser
			So(run(context.Background(), []string{flagName}, io.NopCloser(&bytes.Buffer{}), &output, logger), ShouldBeNil)
			So(output.String(), ShouldEqual, "tuicast-mcp dev (commit none, built unknown)\n")
		})
	}
}
