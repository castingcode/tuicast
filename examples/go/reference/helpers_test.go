package reference_test

import (
	"context"
	"fmt"
	"os"
	"time"

	tuicast "github.com/castingcode/tuicast/sdk/go"
)

type referenceFixture struct {
	driver     *tuicast.Driver
	connection *tuicast.Connection
}

func launchReference(ctx context.Context, config tuicast.ConnectionConfig) (*referenceFixture, error) {
	driver, err := tuicast.Launch(ctx, tuicast.WithDriverPath(environment("TUICAST_DRIVER", "tuicast-driver")))
	if err != nil {
		return nil, fmt.Errorf("launching driver: %w", err)
	}
	connection, err := driver.Connect(ctx, config)
	if err != nil {
		_ = driver.Close()
		return nil, fmt.Errorf("connecting to reference TUI: %w", err)
	}
	return &referenceFixture{driver: driver, connection: connection}, nil
}

func launchSSHReference(ctx context.Context) (*referenceFixture, error) {
	password, err := requiredEnvironment("TUICAST_REFERENCE_PASSWORD")
	if err != nil {
		return nil, err
	}
	return launchReference(ctx, tuicast.SSH{
		Address:                  environment("TUICAST_REFERENCE_ADDRESS", "127.0.0.1:2222"),
		Username:                 "operator",
		Password:                 password,
		InsecureSkipHostKeyCheck: true,
	})
}

func (fixture *referenceFixture) close(ctx context.Context) error {
	if err := fixture.connection.Close(ctx); err != nil {
		_ = fixture.driver.Close()
		return err
	}
	return fixture.driver.Close()
}

func login(ctx context.Context, session *tuicast.Session) error {
	password, err := requiredEnvironment("TUICAST_REFERENCE_APP_PASSWORD")
	if err != nil {
		return err
	}
	if _, err := session.WaitForText(ctx, "LOGIN / AUTHENTICATION", tuicast.StableFor(50*time.Millisecond)); err != nil {
		return err
	}
	if err := session.Type(ctx, "operator"); err != nil {
		return err
	}
	if err := session.Press(ctx, tuicast.Tab); err != nil {
		return err
	}
	if err := session.Type(ctx, password); err != nil {
		return err
	}
	if err := session.Press(ctx, tuicast.Enter); err != nil {
		return err
	}
	_, err = session.WaitForText(ctx, "TERMINAL TEST SYSTEM")
	return err
}

func openScenario(ctx context.Context, session *tuicast.Session, menuIndex int, heading string) (tuicast.Screen, error) {
	for range menuIndex {
		if err := session.Press(ctx, tuicast.ArrowDown); err != nil {
			return tuicast.Screen{}, err
		}
	}
	if err := session.Press(ctx, tuicast.Enter); err != nil {
		return tuicast.Screen{}, err
	}
	return session.WaitForText(ctx, heading, tuicast.StableFor(50*time.Millisecond))
}

func environment(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

// requiredEnvironment reads a password from the environment so that it is
// never committed. See .env.example at the repository root.
func requiredEnvironment(name string) (string, error) {
	value := os.Getenv(name)
	if value == "" {
		return "", fmt.Errorf("reading %s: the environment variable is required; see .env.example", name)
	}
	return value, nil
}
