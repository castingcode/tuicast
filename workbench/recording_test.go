package workbench

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/castingcode/tuicast"
	. "github.com/smartystreets/goconvey/convey"
)

func TestRecording(t *testing.T) {
	Convey("Recording captures assertions, input, named keys, and secret parameters", t, func() {
		controls := &controlFactory{screen: textScreen("User ID: Password:")}
		manager, err := New(controls.acquire)
		So(err, ShouldBeNil)

		recording, err := manager.Start(Session{ID: 7, Terminal: "vt220", Width: 80, Height: 24})
		So(err, ShouldBeNil)
		So(recording.Version, ShouldEqual, "1")

		_, err = manager.AssertText(7, "User ID:", 3*time.Second)
		So(err, ShouldBeNil)
		_, err = manager.Type(7, "operator", "")
		So(err, ShouldBeNil)
		_, err = manager.Press(7, tuicast.KeyEnter)
		So(err, ShouldBeNil)
		_, err = manager.Type(7, "correct horse battery staple", "password")
		So(err, ShouldBeNil)
		recording, err = manager.Stop(7)
		So(err, ShouldBeNil)

		So(recording.Steps, ShouldResemble, []Step{
			{Action: "waitForText", Text: "User ID:", TimeoutMilliseconds: 3000},
			{Action: "type", Text: "operator"},
			{Action: "press", Key: "Enter"},
			{Action: "type", Parameter: "password"},
		})
		encoded, err := json.Marshal(recording)
		So(err, ShouldBeNil)
		So(string(encoded), ShouldNotContainSubstring, "correct horse")
		So(controls.created[0].released, ShouldBeTrue)
	})

	Convey("Replay executes the retained trace in order using supplied parameters", t, func() {
		controls := &controlFactory{screen: textScreen("READY")}
		manager, err := New(controls.acquire)
		So(err, ShouldBeNil)
		_, err = manager.Start(Session{ID: 2, Terminal: "xterm-256color", Width: 80, Height: 24})
		So(err, ShouldBeNil)
		_, err = manager.AssertText(2, "READY", time.Second)
		So(err, ShouldBeNil)
		_, err = manager.Type(2, "private", "value")
		So(err, ShouldBeNil)
		_, err = manager.Press(2, tuicast.KeyF2, tuicast.ModifierControl)
		So(err, ShouldBeNil)
		_, err = manager.Stop(2)
		So(err, ShouldBeNil)

		err = manager.Replay(context.Background(), 2, map[string]string{"value": "replayed"})
		So(err, ShouldBeNil)
		So(controls.created, ShouldHaveLength, 2)
		So(controls.created[1].actions, ShouldResemble, []string{"wait:READY", "send:replayed", "press:Control+F2"})
		So(controls.created[1].released, ShouldBeTrue)
	})

	Convey("Assertions reject text not present on the current screen", t, func() {
		controls := &controlFactory{screen: textScreen("READY")}
		manager, err := New(controls.acquire)
		So(err, ShouldBeNil)
		_, err = manager.Start(Session{ID: 1, Terminal: "vt220", Width: 80, Height: 24})
		So(err, ShouldBeNil)

		_, err = manager.AssertText(1, "MISSING", time.Second)
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldContainSubstring, `current screen does not contain "MISSING"`)
	})

	Convey("Closing Workbench releases active controls", t, func() {
		controls := &controlFactory{screen: textScreen("READY")}
		manager, err := New(controls.acquire)
		So(err, ShouldBeNil)
		_, err = manager.Start(Session{ID: 4, Terminal: "vt220", Width: 80, Height: 24})
		So(err, ShouldBeNil)

		manager.Close()
		_, active, err := manager.Recording(4)
		So(err, ShouldBeNil)
		So(active, ShouldBeFalse)
		So(controls.created[0].released, ShouldBeTrue)
	})
}

type controlFactory struct {
	screen  tuicast.Screen
	created []*fakeControl
}

func (f *controlFactory) acquire(uint64) (SessionControl, error) {
	control := &fakeControl{screen: f.screen}
	f.created = append(f.created, control)
	return control, nil
}

type fakeControl struct {
	screen   tuicast.Screen
	actions  []string
	released bool
}

func (c *fakeControl) Send(data []byte) error {
	c.actions = append(c.actions, "send:"+string(data))
	return nil
}

func (c *fakeControl) Press(key tuicast.Key, modifiers ...tuicast.KeyModifier) error {
	var names []string
	for _, modifier := range modifierNames(modifiers) {
		names = append(names, modifier+"+")
	}
	c.actions = append(c.actions, "press:"+strings.Join(names, "")+string(key))
	return nil
}

func (c *fakeControl) WaitFor(_ context.Context, matcher tuicast.ScreenMatcher) (tuicast.Screen, error) {
	c.actions = append(c.actions, "wait:READY")
	return c.screen, nil
}

func (c *fakeControl) Screen() tuicast.Screen {
	return c.screen
}

func (c *fakeControl) Release() {
	c.released = true
}

func textScreen(text string) tuicast.Screen {
	cells := make([]tuicast.Cell, len([]rune(text)))
	for index, character := range []rune(text) {
		cells[index] = tuicast.Cell{Text: string(character), Width: 1}
	}
	return tuicast.Screen{Width: len(cells), Height: 1, Cells: cells}
}
