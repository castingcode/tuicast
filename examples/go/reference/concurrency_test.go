package reference_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	tuicast "github.com/castingcode/tuicast/sdk/go"
	. "github.com/smartystreets/goconvey/convey"
)

func TestConcurrentSessions(t *testing.T) {
	Convey("Independent sessions share one authenticated SSH connection", t, func() {
		ctx := context.Background()
		fixture, err := launchSSHReference(ctx)
		So(err, ShouldBeNil)
		defer func() { So(fixture.close(ctx), ShouldBeNil) }()

		scenarios := []struct {
			index   int
			heading string
		}{
			{1, "RECEIVING FORM"},
			{2, "WAREHOUSE ORDERS"},
			{6, "FUNCTION KEYS"},
		}
		results := make(chan error, len(scenarios))
		var sessionsMu sync.Mutex
		var sessions []*tuicast.Session
		for _, scenario := range scenarios {
			go func() {
				session, openErr := fixture.connection.OpenSession(ctx)
				if openErr != nil {
					results <- openErr
					return
				}
				sessionsMu.Lock()
				sessions = append(sessions, session)
				sessionsMu.Unlock()
				if loginErr := login(ctx, session); loginErr != nil {
					results <- loginErr
					return
				}
				screen, scenarioErr := openScenario(ctx, session, scenario.index, scenario.heading)
				if scenarioErr == nil && !screen.Contains(scenario.heading) {
					scenarioErr = fmt.Errorf("expected session screen to contain %q", scenario.heading)
				}
				results <- scenarioErr
			}()
		}
		for range scenarios {
			So(<-results, ShouldBeNil)
		}
		for _, session := range sessions {
			So(session.Close(ctx), ShouldBeNil)
		}
	})
}
