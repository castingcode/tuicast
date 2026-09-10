package mcpserver

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/castingcode/tuicast"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// NewMCPServer creates a protocol server backed by service.
func NewMCPServer(service *Service, version string) (*mcp.Server, error) {
	if service == nil {
		return nil, fmt.Errorf("creating MCP protocol server: service is required")
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "tuicast", Version: version}, nil)

	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: boolPointer(false)}
	changesTerminal := &mcp.ToolAnnotations{DestructiveHint: boolPointer(false), OpenWorldHint: boolPointer(true)}
	cleanup := &mcp.ToolAnnotations{DestructiveHint: boolPointer(false), IdempotentHint: true, OpenWorldHint: boolPointer(true)}

	mcp.AddTool(server, &mcp.Tool{
		Name: "tuicast_list_profiles", Description: "List operator-approved terminal connection profiles. Credentials and endpoint addresses are never returned.", Annotations: readOnly,
	}, func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, profilesOutput, error) {
		return nil, profilesOutput{Profiles: service.Profiles()}, nil
	})
	mcp.AddTool(server, &mcp.Tool{
		Name: "tuicast_connect", Description: "Connect to an operator-approved terminal profile by name. Arbitrary host addresses are not accepted.", Annotations: changesTerminal,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input connectInput) (*mcp.CallToolResult, ConnectionInfo, error) {
		result, err := service.Connect(ctx, input.Profile)
		return nil, result, err
	})
	mcp.AddTool(server, &mcp.Tool{
		Name: "tuicast_open_session", Description: "Open an interactive terminal session on an existing TUICast connection using its profile's terminal type and dimensions.", Annotations: changesTerminal,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input connectionInput) (*mcp.CallToolResult, SessionInfo, error) {
		result, err := service.OpenSession(ctx, input.ConnectionID)
		return nil, result, err
	})
	mcp.AddTool(server, &mcp.Tool{
		Name: "tuicast_screen", Description: "Read the current text, dimensions, cursor, and revision of a terminal session screen.", Annotations: readOnly,
	}, func(_ context.Context, _ *mcp.CallToolRequest, input sessionInput) (*mcp.CallToolResult, ScreenInfo, error) {
		result, err := service.Screen(input.SessionID)
		return nil, result, err
	})
	mcp.AddTool(server, &mcp.Tool{
		Name: "tuicast_wait_for_text", Description: "Wait until exact case-sensitive text appears anywhere on a terminal screen.", Annotations: readOnly,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input waitForTextInput) (*mcp.CallToolResult, ScreenInfo, error) {
		timeout, err := waitTimeout(input.TimeoutMilliseconds)
		if err != nil {
			return nil, ScreenInfo{}, err
		}
		result, err := service.WaitForText(ctx, input.SessionID, input.Text, timeout)
		return nil, result, err
	})
	mcp.AddTool(server, &mcp.Tool{
		Name: "tuicast_type", Description: "Type literal text into an interactive terminal session. The text is sent exactly and is not logged by TUICast.", Annotations: changesTerminal,
	}, func(_ context.Context, _ *mcp.CallToolRequest, input typeInput) (*mcp.CallToolResult, actionOutput, error) {
		err := service.Type(input.SessionID, input.Text)
		return nil, actionOutput{Sent: err == nil}, err
	})
	mcp.AddTool(server, &mcp.Tool{
		Name: "tuicast_press", Description: "Press a named terminal key or one printable character, optionally with Shift, Alt, or Control modifiers.", Annotations: changesTerminal,
	}, func(_ context.Context, _ *mcp.CallToolRequest, input pressInput) (*mcp.CallToolResult, actionOutput, error) {
		modifiers, err := parseModifiers(input.Modifiers)
		if err == nil {
			err = service.Press(input.SessionID, tuicast.Key(input.Key), modifiers...)
		}
		return nil, actionOutput{Sent: err == nil}, err
	})
	mcp.AddTool(server, &mcp.Tool{
		Name: "tuicast_close_session", Description: "Close a TUICast terminal session. Safe to repeat during cleanup.", Annotations: cleanup,
	}, func(_ context.Context, _ *mcp.CallToolRequest, input sessionInput) (*mcp.CallToolResult, CloseInfo, error) {
		result, err := service.CloseSession(input.SessionID)
		return nil, result, err
	})
	mcp.AddTool(server, &mcp.Tool{
		Name: "tuicast_close_connection", Description: "Close a TUICast connection and all of its sessions. Safe to repeat during cleanup.", Annotations: cleanup,
	}, func(_ context.Context, _ *mcp.CallToolRequest, input connectionInput) (*mcp.CallToolResult, CloseInfo, error) {
		result, err := service.CloseConnection(input.ConnectionID)
		return nil, result, err
	})
	return server, nil
}

type profilesOutput struct {
	Profiles []ProfileInfo `json:"profiles"`
}

type connectInput struct {
	Profile string `json:"profile" jsonschema:"name of an operator-approved connection profile"`
}

type connectionInput struct {
	ConnectionID uint64 `json:"connectionId" jsonschema:"TUICast connection identifier returned by tuicast_connect"`
}

type sessionInput struct {
	SessionID uint64 `json:"sessionId" jsonschema:"TUICast session identifier returned by tuicast_open_session"`
}

type waitForTextInput struct {
	SessionID           uint64 `json:"sessionId" jsonschema:"TUICast session identifier"`
	Text                string `json:"text" jsonschema:"exact case-sensitive text to wait for"`
	TimeoutMilliseconds int    `json:"timeoutMilliseconds,omitempty" jsonschema:"timeout in milliseconds; defaults to 10000 and cannot exceed 120000"`
}

type typeInput struct {
	SessionID uint64 `json:"sessionId" jsonschema:"TUICast session identifier"`
	Text      string `json:"text" jsonschema:"literal text to send exactly as provided"`
}

type pressInput struct {
	SessionID uint64   `json:"sessionId" jsonschema:"TUICast session identifier"`
	Key       string   `json:"key" jsonschema:"named key such as Enter, Tab, ArrowUp, or F1; or one printable character"`
	Modifiers []string `json:"modifiers,omitempty" jsonschema:"optional modifiers: Shift, Alt, or Control"`
}

type actionOutput struct {
	Sent bool `json:"sent"`
}

func parseModifiers(names []string) ([]tuicast.KeyModifier, error) {
	modifiers := make([]tuicast.KeyModifier, 0, len(names))
	for _, name := range names {
		switch strings.ToLower(name) {
		case "shift":
			modifiers = append(modifiers, tuicast.ModifierShift)
		case "alt", "meta":
			modifiers = append(modifiers, tuicast.ModifierAlt)
		case "control", "ctrl":
			modifiers = append(modifiers, tuicast.ModifierControl)
		default:
			return nil, fmt.Errorf("parsing MCP key modifiers: unknown modifier %q", name)
		}
	}
	return modifiers, nil
}

func waitTimeout(milliseconds int) (time.Duration, error) {
	if milliseconds < 0 || milliseconds > int(maximumWaitTimeout/time.Millisecond) {
		return 0, fmt.Errorf("parsing MCP wait timeout: timeoutMilliseconds must be between 0 and %d", maximumWaitTimeout/time.Millisecond)
	}
	return time.Duration(milliseconds) * time.Millisecond, nil
}

func boolPointer(value bool) *bool {
	return &value
}
