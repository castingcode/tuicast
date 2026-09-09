package reference_test

import (
	"context"
	"testing"
	"time"

	tuicast "github.com/castingcode/tuicast/sdk/go"
	. "github.com/smartystreets/goconvey/convey"
)

func TestScreenSubscription(t *testing.T) {
	Convey("A screen subscription observes coalesced revisions without polling", t, func() {
		ctx := context.Background()
		fixture, err := launchSSHReference(ctx)
		So(err, ShouldBeNil)
		defer func() { So(fixture.close(ctx), ShouldBeNil) }()
		session, err := fixture.connection.OpenSession(ctx)
		So(err, ShouldBeNil)
		defer func() { So(session.Close(ctx), ShouldBeNil) }()
		So(login(ctx, session), ShouldBeNil)
		subscription, err := session.Subscribe(ctx)
		So(err, ShouldBeNil)
		defer func() { So(subscription.Close(ctx), ShouldBeNil) }()

		_, err = openScenario(ctx, session, 1, "RECEIVING FORM")
		So(err, ShouldBeNil)
		var observed tuicast.Screen
		deadline := time.After(time.Second)
	observe:
		for !observed.Contains("RECEIVING FORM") {
			select {
			case observed = <-subscription.Screens:
			case <-deadline:
				break observe
			}
		}
		So(observed.Contains("RECEIVING FORM"), ShouldBeTrue)
		So(observed.Revision, ShouldBeGreaterThan, uint64(0))
	})
}

func TestTerminalEventSubscription(t *testing.T) {
	Convey("BELL and ENQ arrive in order and ENQ sends the configured answerback first", t, func() {
		ctx := context.Background()
		fixture, err := launchSSHReference(ctx)
		So(err, ShouldBeNil)
		defer func() { So(fixture.close(ctx), ShouldBeNil) }()
		session, err := fixture.connection.OpenSession(ctx, tuicast.WithAnswerback("TUICAST-ANSWER"))
		So(err, ShouldBeNil)
		defer func() { So(session.Close(ctx), ShouldBeNil) }()
		So(login(ctx, session), ShouldBeNil)
		_, err = openScenario(ctx, session, 11, "BELL / ENQ ANSWERBACK")
		So(err, ShouldBeNil)
		events, err := session.SubscribeEvents(ctx)
		So(err, ShouldBeNil)
		defer func() { So(events.Close(ctx), ShouldBeNil) }()

		So(session.Press(ctx, tuicast.Key("b")), ShouldBeNil)
		bell, received := receiveEvent(events.Events)
		So(received, ShouldBeTrue)
		So(bell, ShouldResemble, tuicast.TerminalEvent{Sequence: 1, Type: tuicast.EventBell})
		So(session.Press(ctx, tuicast.Key("e")), ShouldBeNil)
		enquiry, received := receiveEvent(events.Events)
		So(received, ShouldBeTrue)
		So(enquiry, ShouldResemble, tuicast.TerminalEvent{Sequence: 2, Type: tuicast.EventEnquiry, Data: "TUICAST-ANSWER"})

		screen, err := session.WaitForText(ctx, `Answerback: "TUICAST-ANSWER"`)
		So(err, ShouldBeNil)
		So(screen.Contains("BEL emitted: 1 / 1"), ShouldBeTrue)
		So(screen.Contains("ENQ emitted: 1 / 1"), ShouldBeTrue)
	})
}

func receiveEvent(events <-chan tuicast.TerminalEvent) (tuicast.TerminalEvent, bool) {
	select {
	case event, open := <-events:
		return event, open
	case <-time.After(time.Second):
		return tuicast.TerminalEvent{}, false
	}
}
