package reference_test

import (
	"context"
	"os"
	"testing"
	"time"

	tuicast "github.com/castingcode/tuicast/sdk/go"
	. "github.com/smartystreets/goconvey/convey"
)

const referencePassword = "cast" + "ing"

func TestReferenceWorkflow(t *testing.T) {
	Convey("Given a TUICast driver and the reference SSH application", t, func() {
		ctx := context.Background()
		driver, err := tuicast.Launch(ctx, tuicast.WithDriverPath(environment("TUICAST_DRIVER", "tuicast-driver")))
		So(err, ShouldBeNil)
		defer func() { So(driver.Close(), ShouldBeNil) }()

		connection, err := driver.Connect(ctx, tuicast.SSH{
			Address:                  environment("TUICAST_REFERENCE_ADDRESS", "127.0.0.1:2222"),
			Username:                 "operator",
			Password:                 referencePassword,
			InsecureSkipHostKeyCheck: true,
		})
		So(err, ShouldBeNil)
		defer func() { So(connection.Close(ctx), ShouldBeNil) }()

		session, err := connection.OpenSession(ctx, tuicast.WithTerminal(tuicast.XTerm256Color))
		So(err, ShouldBeNil)
		defer func() { So(session.Close(ctx), ShouldBeNil) }()

		Convey("When the user logs in", func() {
			screen, err := session.WaitForText(ctx, "LOGIN / AUTHENTICATION", tuicast.StableFor(50*time.Millisecond))
			So(err, ShouldBeNil)
			So(screen.Contains("User ID"), ShouldBeTrue)

			So(session.Type(ctx, "operator"), ShouldBeNil)
			So(session.Press(ctx, tuicast.Tab), ShouldBeNil)
			So(session.Type(ctx, referencePassword), ShouldBeNil)
			So(session.Press(ctx, tuicast.Enter), ShouldBeNil)

			Convey("Then the menu and receiving form can be inspected", func() {
				screen, err = session.WaitForText(ctx, "TERMINAL TEST SYSTEM")
				So(err, ShouldBeNil)
				So(screen.Contains("Forms and Input Fields"), ShouldBeTrue)
				_, found := screen.Find("Forms and Input Fields")
				So(found, ShouldBeTrue)

				So(session.Press(ctx, tuicast.ArrowDown), ShouldBeNil)
				So(session.Press(ctx, tuicast.Enter), ShouldBeNil)
				screen, err = session.WaitForText(ctx, "RECEIVING FORM")
				So(err, ShouldBeNil)
				So(screen.Contains("Purchase order"), ShouldBeTrue)
			})
		})
	})
}

func environment(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
