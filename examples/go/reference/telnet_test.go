package reference_test

import (
	"context"
	"testing"

	tuicast "github.com/castingcode/tuicast/sdk/go"
	. "github.com/smartystreets/goconvey/convey"
)

func TestTelnetTransport(t *testing.T) {
	Convey("The same workflow runs over a Telnet connection", t, func() {
		ctx := context.Background()
		fixture, err := launchReference(ctx, tuicast.Telnet{
			Address: environment("TUICAST_REFERENCE_TELNET_ADDRESS", "127.0.0.1:2323"),
		})
		So(err, ShouldBeNil)
		defer func() { So(fixture.close(ctx), ShouldBeNil) }()
		session, err := fixture.connection.OpenSession(ctx)
		So(err, ShouldBeNil)
		defer func() { So(session.Close(ctx), ShouldBeNil) }()

		So(login(ctx, session), ShouldBeNil)
		screen, err := openScenario(ctx, session, 1, "RECEIVING FORM")
		So(err, ShouldBeNil)
		So(screen.Contains("Purchase order"), ShouldBeTrue)
	})
}
