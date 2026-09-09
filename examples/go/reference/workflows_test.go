package reference_test

import (
	"context"
	"testing"

	tuicast "github.com/castingcode/tuicast/sdk/go"
	. "github.com/smartystreets/goconvey/convey"
)

func TestFormWorkflow(t *testing.T) {
	Convey("A receiving form can be validated, completed, and submitted", t, func() {
		ctx := context.Background()
		fixture, err := launchSSHReference(ctx)
		So(err, ShouldBeNil)
		defer func() { So(fixture.close(ctx), ShouldBeNil) }()
		session, err := fixture.connection.OpenSession(ctx, tuicast.WithTerminal(tuicast.XTerm256Color))
		So(err, ShouldBeNil)
		defer func() { So(session.Close(ctx), ShouldBeNil) }()
		So(login(ctx, session), ShouldBeNil)

		screen, err := openScenario(ctx, session, 1, "RECEIVING FORM")
		So(err, ShouldBeNil)
		So(screen.Contains("Purchase order"), ShouldBeTrue)

		// Move backward from the first field to Submit and exercise validation.
		So(session.Press(ctx, tuicast.Tab, tuicast.Shift), ShouldBeNil)
		So(session.Press(ctx, tuicast.Tab, tuicast.Shift), ShouldBeNil)
		So(session.Press(ctx, tuicast.Enter), ShouldBeNil)
		_, err = session.WaitFor(ctx, tuicast.All(
			tuicast.Contains("purchase order is required"),
			tuicast.Contains("quantity must be a positive number"),
		))
		So(err, ShouldBeNil)

		for _, value := range []string{"PO-10002341", "WIDGET-42", "25", "A-01-02", "dock 3"} {
			So(session.Type(ctx, value), ShouldBeNil)
			So(session.Press(ctx, tuicast.Tab), ShouldBeNil)
		}
		So(session.Press(ctx, tuicast.ArrowRight), ShouldBeNil) // urgent priority
		So(session.Press(ctx, tuicast.Tab), ShouldBeNil)        // submit
		So(session.Press(ctx, tuicast.Enter), ShouldBeNil)

		screen, err = session.WaitForText(ctx, "RECEIPT ACCEPTED")
		So(err, ShouldBeNil)
		So(screen.Contains("PO-10002341 / WIDGET-42 / quantity 25 / A-01-02 / Urgent"), ShouldBeTrue)
	})
}

func TestTableWorkflow(t *testing.T) {
	Convey("Warehouse orders can be filtered, navigated, and inspected", t, func() {
		ctx := context.Background()
		fixture, err := launchSSHReference(ctx)
		So(err, ShouldBeNil)
		defer func() { So(fixture.close(ctx), ShouldBeNil) }()
		session, err := fixture.connection.OpenSession(ctx)
		So(err, ShouldBeNil)
		defer func() { So(session.Close(ctx), ShouldBeNil) }()
		So(login(ctx, session), ShouldBeNil)
		_, err = openScenario(ctx, session, 2, "WAREHOUSE ORDERS")
		So(err, ShouldBeNil)

		So(session.Press(ctx, tuicast.Key("/")), ShouldBeNil)
		So(session.Type(ctx, "HOLD"), ShouldBeNil)
		So(session.Press(ctx, tuicast.Enter), ShouldBeNil)
		So(session.Press(ctx, tuicast.End), ShouldBeNil)
		So(session.Press(ctx, tuicast.Enter), ShouldBeNil)

		screen, err := session.WaitFor(ctx, tuicast.All(
			tuicast.Contains("ORDER DETAILS"),
			tuicast.Contains("Status:    HOLD"),
		))
		So(err, ShouldBeNil)
		So(screen.Contains("Location:"), ShouldBeTrue)
	})
}
