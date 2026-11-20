package reference_test

import (
	"context"
	"testing"

	tuicast "github.com/castingcode/tuicast/sdk/go"
	. "github.com/smartystreets/goconvey/convey"
)

func TestPTYTransport(t *testing.T) {
	Convey("The same workflow runs on a local pseudo-terminal", t, func() {
		ctx := context.Background()
		password, err := requiredEnvironment("TUICAST_REFERENCE_APP_PASSWORD")
		So(err, ShouldBeNil)
		fixture, err := launchReference(ctx, tuicast.PTY{
			Command:     environment("TUICAST_REFERENCE_TUI", "reference-tui"),
			Environment: map[string]string{"TUICAST_REFERENCE_APP_PASSWORD": password},
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
