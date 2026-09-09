package reference_test

import (
	"context"
	"errors"
	"testing"
	"time"

	tuicast "github.com/castingcode/tuicast/sdk/go"
	. "github.com/smartystreets/goconvey/convey"
)

func TestPartialScreenSynchronization(t *testing.T) {
	Convey("Automation waits for the complete stable screen rather than premature READY text", t, func() {
		ctx := context.Background()
		fixture, err := launchSSHReference(ctx)
		So(err, ShouldBeNil)
		defer func() { So(fixture.close(ctx), ShouldBeNil) }()
		session, err := fixture.connection.OpenSession(ctx)
		So(err, ShouldBeNil)
		defer func() { So(session.Close(ctx), ShouldBeNil) }()
		So(login(ctx, session), ShouldBeNil)
		_, err = openScenario(ctx, session, 7, "PARTIAL SCREEN UPDATES")
		So(err, ShouldBeNil)
		So(session.Press(ctx, tuicast.Key("1")), ShouldBeNil)

		// READY appears before all regions are rendered and before input is enabled.
		_, err = session.WaitForText(ctx, "READY")
		So(err, ShouldBeNil)
		screen, err := session.WaitFor(ctx, tuicast.All(
			tuicast.Contains("READY"),
			tuicast.Contains("SCREEN COMPLETE / INPUT ENABLED"),
		), tuicast.StableFor(100*time.Millisecond))
		So(err, ShouldBeNil)
		So(screen.Contains("Inventory: VERIFIED"), ShouldBeTrue)
		_, err = session.WaitForIdle(ctx, tuicast.IdleFor(100*time.Millisecond))
		So(err, ShouldBeNil)

		So(session.Press(ctx, tuicast.Key("x")), ShouldBeNil)
		screen, err = session.WaitForText(ctx, "Accepted input: x")
		So(err, ShouldBeNil)
	})
}

func TestLongRunningOperation(t *testing.T) {
	Convey("A finite operation can be observed until semantic completion", t, func() {
		ctx := context.Background()
		fixture, err := launchSSHReference(ctx)
		So(err, ShouldBeNil)
		defer func() { So(fixture.close(ctx), ShouldBeNil) }()
		session, err := fixture.connection.OpenSession(ctx)
		So(err, ShouldBeNil)
		defer func() { So(session.Close(ctx), ShouldBeNil) }()
		So(login(ctx, session), ShouldBeNil)
		_, err = openScenario(ctx, session, 8, "LONG-RUNNING OPERATION")
		So(err, ShouldBeNil)
		So(session.Press(ctx, tuicast.Key("1")), ShouldBeNil)

		screen, err := session.WaitFor(ctx, tuicast.All(
			tuicast.Contains("Processed: 120"),
			tuicast.Contains("Status: OPERATION COMPLETE"),
			tuicast.Not(tuicast.Contains("STREAMING")),
		), tuicast.WaitTimeout(12*time.Second))
		So(err, ShouldBeNil)
		So(screen.Contains("Remaining: 0"), ShouldBeTrue)
	})
}

func TestWaitDiagnostics(t *testing.T) {
	Convey("A timed-out wait retains its expected condition and last screen", t, func() {
		ctx := context.Background()
		fixture, err := launchSSHReference(ctx)
		So(err, ShouldBeNil)
		defer func() { So(fixture.close(ctx), ShouldBeNil) }()
		session, err := fixture.connection.OpenSession(ctx)
		So(err, ShouldBeNil)
		defer func() { So(session.Close(ctx), ShouldBeNil) }()

		_, err = session.WaitForText(ctx, "TEXT THAT WILL NOT APPEAR", tuicast.WaitTimeout(100*time.Millisecond))
		So(tuicast.IsTimeout(err), ShouldBeTrue)
		var waitError *tuicast.WaitError
		So(errors.As(err, &waitError), ShouldBeTrue)
		So(waitError.Expected, ShouldContainSubstring, "TEXT THAT WILL NOT APPEAR")
		So(waitError.LastScreen.Contains("LOGIN / AUTHENTICATION"), ShouldBeTrue)
	})
}
