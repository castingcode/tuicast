package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	tuicast "github.com/castingcode/tuicast/sdk/go"
	"github.com/cucumber/godog"
)

const referencePassword = "cast" + "ing"

type scenario struct {
	driver     *tuicast.Driver
	connection *tuicast.Connection
	session    *tuicast.Session
}

func main() {
	os.Exit(testSuite("pretty").Run())
}

func testSuite(format string) godog.TestSuite {
	state := &scenario{}
	return godog.TestSuite{
		Name:                "reference-tui",
		ScenarioInitializer: state.initialize,
		Options: &godog.Options{
			Format: format,
			Paths:  []string{featuresPath()},
			Strict: true,
		},
	}
}

func featuresPath() string {
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		return filepath.Join("..", "features")
	}
	return filepath.Join(filepath.Dir(source), "..", "..", "features")
}

func (s *scenario) initialize(ctx *godog.ScenarioContext) {
	ctx.Given(`^I am connected to the reference TUI$`, s.connect)
	ctx.Given(`^I am logged in$`, s.login)
	ctx.When(`^I enter username "([^"]*)"$`, s.enterUsername)
	ctx.When(`^I enter password "([^"]*)"$`, s.enterPassword)
	ctx.When(`^I press (.+)$`, s.press)
	ctx.When(`^I select the "([^"]*)" menu option$`, s.selectMenuOption)
	ctx.Then(`^the application is on the "([^"]*)" screen$`, s.verifyScreen)
	ctx.Then(`^the screen contains "([^"]*)"$`, s.screenContains)
	ctx.Then(`^the application reports that authentication failed$`, s.authenticationFailed)
	ctx.Then(`^"([^"]*)" begins at zero-based column (\d+) and row (\d+)$`, s.textBeginsAt)
	ctx.Then(`^"([^"]*)" is rendered with the "([^"]*)" attribute$`, s.textHasAttribute)
	ctx.Then(`^the "([^"]*)" color sample uses foreground (\d+) and background (\d+)$`, s.colorSample)
	ctx.Then(`^the "([^"]*)" sample renders "([^"]*)" at zero-based column (\d+) and row (\d+)$`, s.unicodeSample)
	ctx.Then(`^the captured key count is (\d+)$`, s.capturedKeyCount)
	ctx.Then(`^the latest key is "([^"]*)"$`, s.latestKey)
	ctx.Then(`^the latest key modifiers are "([^"]*)"$`, s.latestKeyModifiers)

	ctx.After(func(ctx context.Context, _ *godog.Scenario, _ error) (context.Context, error) {
		return ctx, s.close()
	})
}

func (s *scenario) connect(ctx context.Context) error {
	if s.driver != nil || s.connection != nil || s.session != nil {
		return fmt.Errorf("connecting to reference TUI: scenario already has an active fixture")
	}
	driver, err := tuicast.Launch(ctx, tuicast.WithDriverPath(environment("TUICAST_DRIVER", "tuicast-driver")))
	if err != nil {
		return fmt.Errorf("launching driver: %w", err)
	}
	s.driver = driver
	connection, err := driver.Connect(ctx, tuicast.SSH{
		Address:                  environment("TUICAST_REFERENCE_ADDRESS", "127.0.0.1:2222"),
		Username:                 "operator",
		Password:                 referencePassword,
		InsecureSkipHostKeyCheck: true,
	})
	if err != nil {
		return fmt.Errorf("connecting to reference TUI: %w", err)
	}
	s.connection = connection
	session, err := connection.OpenSession(ctx, tuicast.WithTerminal(tuicast.XTerm256Color))
	if err != nil {
		return fmt.Errorf("opening reference TUI session: %w", err)
	}
	s.session = session
	if _, err := session.WaitForText(ctx, "LOGIN / AUTHENTICATION", tuicast.StableFor(50*time.Millisecond)); err != nil {
		return fmt.Errorf("waiting for login screen: %w", err)
	}
	return nil
}

func (s *scenario) login(ctx context.Context) error {
	if err := s.enterUsername(ctx, "operator"); err != nil {
		return err
	}
	if err := s.enterPassword(ctx, referencePassword); err != nil {
		return err
	}
	if err := s.press(ctx, "Enter"); err != nil {
		return err
	}
	return s.verifyScreen(ctx, "main menu")
}

func (s *scenario) enterUsername(ctx context.Context, username string) error {
	if err := s.requireSession(); err != nil {
		return err
	}
	if err := s.session.Type(ctx, username); err != nil {
		return fmt.Errorf("entering username: %w", err)
	}
	return nil
}

func (s *scenario) enterPassword(ctx context.Context, password string) error {
	if err := s.requireSession(); err != nil {
		return err
	}
	if err := s.session.Press(ctx, tuicast.Tab); err != nil {
		return fmt.Errorf("focusing password field: %w", err)
	}
	if err := s.session.Type(ctx, password); err != nil {
		return fmt.Errorf("entering password: %w", err)
	}
	return nil
}

func (s *scenario) press(ctx context.Context, name string) error {
	if err := s.requireSession(); err != nil {
		return err
	}
	key, modifiers, err := namedKey(name)
	if err != nil {
		return err
	}
	if err := s.session.Press(ctx, key, modifiers...); err != nil {
		return fmt.Errorf("pressing %s: %w", name, err)
	}
	return nil
}

func namedKey(name string) (tuicast.Key, []tuicast.Modifier, error) {
	switch name {
	case "Enter":
		return tuicast.Enter, nil, nil
	case "F5":
		return tuicast.F5, nil, nil
	case "Control+C":
		return tuicast.Key("c"), []tuicast.Modifier{tuicast.Control}, nil
	default:
		return "", nil, fmt.Errorf("pressing key: unsupported key %q", name)
	}
}

func (s *scenario) selectMenuOption(ctx context.Context, option string) error {
	menuIndexes := map[string]int{
		"Colors and Attributes": 4,
		"Function Keys":         6,
		"Unicode":               10,
	}
	index, ok := menuIndexes[option]
	if !ok {
		return fmt.Errorf("selecting menu option: unsupported option %q", option)
	}
	for range index {
		if err := s.session.Press(ctx, tuicast.ArrowDown); err != nil {
			return fmt.Errorf("selecting %q menu option: %w", option, err)
		}
	}
	if err := s.session.Press(ctx, tuicast.Enter); err != nil {
		return fmt.Errorf("opening %q menu option: %w", option, err)
	}
	return nil
}

func (s *scenario) verifyScreen(ctx context.Context, name string) error {
	headings := map[string]string{
		"login":                 "LOGIN / AUTHENTICATION",
		"main menu":             "TERMINAL TEST SYSTEM",
		"Colors and Attributes": "ANSI COLORS AND ATTRIBUTES",
		"Function Keys":         "FUNCTION KEYS",
		"Unicode":               "UNICODE ALIGNMENT",
	}
	heading, ok := headings[name]
	if !ok {
		return fmt.Errorf("verifying application screen: unsupported screen %q", name)
	}
	if _, err := s.session.WaitForText(ctx, heading, tuicast.StableFor(50*time.Millisecond)); err != nil {
		return fmt.Errorf("waiting for %q screen: %w", name, err)
	}
	return nil
}

func (s *scenario) screenContains(ctx context.Context, text string) error {
	if _, err := s.session.WaitForText(ctx, text); err != nil {
		return fmt.Errorf("waiting for screen text %q: %w", text, err)
	}
	return nil
}

func (s *scenario) authenticationFailed(ctx context.Context) error {
	return s.screenContains(ctx, "Invalid user ID or password")
}

func (s *scenario) textBeginsAt(ctx context.Context, text string, column, row int) error {
	screen, err := s.session.Screen(ctx)
	if err != nil {
		return fmt.Errorf("reading screen: %w", err)
	}
	position, found := screen.Find(text)
	if !found {
		return fmt.Errorf("verifying text position: %q was not found", text)
	}
	if position.Column != column || position.Row != row {
		return fmt.Errorf("verifying text position: %q began at column %d row %d, expected column %d row %d", text, position.Column, position.Row, column, row)
	}
	return nil
}

func (s *scenario) textHasAttribute(ctx context.Context, text, attribute string) error {
	attributes := map[string]tuicast.Attributes{
		"bold":      tuicast.Bold,
		"underline": tuicast.Underline,
	}
	expected, ok := attributes[attribute]
	if !ok {
		return fmt.Errorf("verifying text attribute: unsupported attribute %q", attribute)
	}
	screen, err := s.session.Screen(ctx)
	if err != nil {
		return fmt.Errorf("reading screen: %w", err)
	}
	position, found := screen.Find(text)
	if !found {
		return fmt.Errorf("verifying text attribute: %q was not found", text)
	}
	cell, found := screen.CellAt(position.Column, position.Row)
	if !found || !cell.Attributes.Has(expected) {
		return fmt.Errorf("verifying text attribute: %q does not have attribute %q", text, attribute)
	}
	return nil
}

func (s *scenario) colorSample(ctx context.Context, text string, foreground, background int) error {
	screen, err := s.session.Screen(ctx)
	if err != nil {
		return fmt.Errorf("reading screen: %w", err)
	}
	position, found := screen.Find(text)
	if !found {
		return fmt.Errorf("verifying color sample: %q was not found", text)
	}
	cell, found := screen.CellAt(position.Column, position.Row)
	if !found {
		return fmt.Errorf("verifying color sample: cell for %q was not found", text)
	}
	if int(cell.Foreground) != foreground || int(cell.Background) != background {
		return fmt.Errorf("verifying color sample %q: got foreground %d and background %d, expected foreground %d and background %d", text, cell.Foreground, cell.Background, foreground, background)
	}
	return nil
}

func (s *scenario) unicodeSample(ctx context.Context, label, text string, column, row int) error {
	screen, err := s.session.Screen(ctx)
	if err != nil {
		return fmt.Errorf("reading screen: %w", err)
	}
	labelPosition, found := screen.Find(label)
	if !found || labelPosition.Row != row {
		return fmt.Errorf("verifying %q Unicode sample: label was not found on row %d", label, row)
	}
	position, found := screen.Find(text)
	if !found || position.Column != column || position.Row != row {
		return fmt.Errorf("verifying %q Unicode sample: %q was not found at column %d row %d", label, text, column, row)
	}
	return nil
}

func (s *scenario) capturedKeyCount(ctx context.Context, count int) error {
	return s.screenContains(ctx, "Captured count: "+strconv.Itoa(count))
}

func (s *scenario) latestKey(ctx context.Context, key string) error {
	return s.screenContains(ctx, "Last: "+key)
}

func (s *scenario) latestKeyModifiers(ctx context.Context, modifiers string) error {
	return s.screenContains(ctx, "Modifiers: "+modifiers)
}

func (s *scenario) close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var cleanupErrors []error
	if s.session != nil {
		if err := s.logout(ctx); err != nil {
			cleanupErrors = append(cleanupErrors, err)
		}
		if err := s.session.Close(ctx); err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("closing session: %w", err))
		}
	}
	if s.connection != nil {
		if err := s.connection.Close(ctx); err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("closing connection: %w", err))
		}
	}
	if s.driver != nil {
		if err := s.driver.Close(); err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("closing driver: %w", err))
		}
	}
	s.driver, s.connection, s.session = nil, nil, nil
	return errors.Join(cleanupErrors...)
}

func (s *scenario) logout(ctx context.Context) error {
	screen, err := s.session.Screen(ctx)
	if err != nil {
		return fmt.Errorf("reading screen before logout: %w", err)
	}
	if screen.Contains("LOGIN / AUTHENTICATION") {
		return nil
	}
	if !screen.Contains("TERMINAL TEST SYSTEM") {
		if err := s.session.Press(ctx, tuicast.Escape); err != nil {
			return fmt.Errorf("returning to main menu for logout: %w", err)
		}
		if _, err := s.session.WaitForText(ctx, "TERMINAL TEST SYSTEM"); err != nil {
			return fmt.Errorf("waiting for main menu before logout: %w", err)
		}
	}
	if err := s.session.Press(ctx, tuicast.F2); err != nil {
		return fmt.Errorf("logging out: %w", err)
	}
	if _, err := s.session.WaitForText(ctx, "Signed out"); err != nil {
		return fmt.Errorf("waiting for logout: %w", err)
	}
	return nil
}

func (s *scenario) requireSession() error {
	if s.session == nil {
		return fmt.Errorf("using reference TUI: no session is connected")
	}
	return nil
}

func environment(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
