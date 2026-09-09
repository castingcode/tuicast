package reference_test

import (
	"context"
	"testing"

	tuicast "github.com/castingcode/tuicast/sdk/go"
	. "github.com/smartystreets/goconvey/convey"
)

func TestTerminalResize(t *testing.T) {
	Convey("A remote resize changes layout while preserving application state", t, func() {
		ctx := context.Background()
		fixture, err := launchSSHReference(ctx)
		So(err, ShouldBeNil)
		defer func() { So(fixture.close(ctx), ShouldBeNil) }()
		session, err := fixture.connection.OpenSession(ctx, tuicast.WithSize(80, 24))
		So(err, ShouldBeNil)
		defer func() { So(session.Close(ctx), ShouldBeNil) }()
		So(login(ctx, session), ShouldBeNil)
		_, err = openScenario(ctx, session, 9, "TERMINAL RESIZE")
		So(err, ShouldBeNil)
		So(session.Type(ctx, "value survives"), ShouldBeNil)
		So(session.Press(ctx, tuicast.ArrowDown), ShouldBeNil)
		So(session.Press(ctx, tuicast.Key("T")), ShouldBeNil)
		So(session.Resize(ctx, 132, 24), ShouldBeNil)

		screen, err := session.WaitFor(ctx, tuicast.All(
			tuicast.Contains("Actual dimensions: 132x24"),
			tuicast.Contains("Status: MATCH"),
			tuicast.Contains("value survives"),
			tuicast.Contains("> ORD-10002342"),
		))
		So(err, ShouldBeNil)
		So(screen.Width, ShouldEqual, 132)
	})
}

func TestStructuredScreenInspection(t *testing.T) {
	Convey("Colors, attributes, and wide Unicode can be inspected as cells", t, func() {
		ctx := context.Background()
		fixture, err := launchSSHReference(ctx)
		So(err, ShouldBeNil)
		defer func() { So(fixture.close(ctx), ShouldBeNil) }()

		colors, err := fixture.connection.OpenSession(ctx, tuicast.WithTerminal(tuicast.XTerm256Color))
		So(err, ShouldBeNil)
		defer func() { So(colors.Close(ctx), ShouldBeNil) }()
		So(login(ctx, colors), ShouldBeNil)
		screen, err := openScenario(ctx, colors, 4, "ANSI COLORS AND ATTRIBUTES")
		So(err, ShouldBeNil)
		position, found := screen.Find("BOLD")
		So(found, ShouldBeTrue)
		cell, found := screen.CellAt(position.Column, position.Row)
		So(found, ShouldBeTrue)
		So(cell.Attributes.Has(tuicast.Bold), ShouldBeTrue)

		unicode, err := fixture.connection.OpenSession(ctx, tuicast.WithTerminal(tuicast.XTerm256Color))
		So(err, ShouldBeNil)
		defer func() { So(unicode.Close(ctx), ShouldBeNil) }()
		So(login(ctx, unicode), ShouldBeNil)
		screen, err = openScenario(ctx, unicode, 10, "UNICODE ALIGNMENT")
		So(err, ShouldBeNil)
		position, found = screen.Find("漢字")
		So(found, ShouldBeTrue)
		wide, _ := screen.CellAt(position.Column, position.Row)
		continuation, _ := screen.CellAt(position.Column+1, position.Row)
		So(wide.Width, ShouldEqual, 2)
		So(continuation.Width, ShouldEqual, 0)
	})
}

func TestKeysAndModifiers(t *testing.T) {
	Convey("Named keys and modifiers are encoded for the selected terminal", t, func() {
		ctx := context.Background()
		fixture, err := launchSSHReference(ctx)
		So(err, ShouldBeNil)
		defer func() { So(fixture.close(ctx), ShouldBeNil) }()
		session, err := fixture.connection.OpenSession(ctx, tuicast.WithTerminal(tuicast.XTerm256Color))
		So(err, ShouldBeNil)
		defer func() { So(session.Close(ctx), ShouldBeNil) }()
		So(login(ctx, session), ShouldBeNil)
		_, err = openScenario(ctx, session, 6, "FUNCTION KEYS")
		So(err, ShouldBeNil)

		So(session.Press(ctx, tuicast.F5), ShouldBeNil)
		_, err = session.WaitForText(ctx, "Last: f5")
		So(err, ShouldBeNil)
		So(session.Press(ctx, tuicast.Key("c"), tuicast.Control), ShouldBeNil)
		screen, err := session.WaitForText(ctx, "Last: ctrl+c")
		So(err, ShouldBeNil)
		So(screen.Contains("Modifiers: control"), ShouldBeTrue)
	})
}
