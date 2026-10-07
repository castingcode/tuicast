package mcpserver

import (
	"context"
	"encoding/json"
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
	recordingControl := &mcp.ToolAnnotations{DestructiveHint: boolPointer(false), OpenWorldHint: boolPointer(false)}

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
		timeout, err := boundedMilliseconds("timeoutMilliseconds", input.TimeoutMilliseconds)
		if err != nil {
			return nil, ScreenInfo{}, err
		}
		result, err := service.WaitForText(ctx, input.SessionID, input.Text, timeout)
		return nil, result, err
	})
	mcp.AddTool(server, &mcp.Tool{
		Name: "tuicast_wait",
		Description: "Wait until a terminal screen matches a matcher expression. These are the same expressions accepted by the TUICast SDKs and driver: " +
			`{"contains": "text"} for exact case-sensitive text anywhere; {"line": {"row": 0, "text": "..."}} for an exact zero-based row including trailing spaces; ` +
			`{"cursor": {"column": 0, "row": 0}} for a zero-based cursor position; {"all": [...]}, {"any": [...]}, and {"not": {...}} to combine them. ` +
			"A positive stableMilliseconds additionally requires that no host output arrives for that period after the screen matches.",
		Annotations: readOnly,
		InputSchema: json.RawMessage(waitInputSchema),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input waitInput) (*mcp.CallToolResult, ScreenInfo, error) {
		timeout, err := boundedMilliseconds("timeoutMilliseconds", input.TimeoutMilliseconds)
		if err != nil {
			return nil, ScreenInfo{}, err
		}
		stable, err := boundedMilliseconds("stableMilliseconds", input.StableMilliseconds)
		if err != nil {
			return nil, ScreenInfo{}, err
		}
		result, err := service.Wait(ctx, input.SessionID, input.Matcher, timeout, stable)
		return nil, result, err
	})
	mcp.AddTool(server, &mcp.Tool{
		Name: "tuicast_wait_for_idle", Description: "Wait until the host has sent no output for quietMilliseconds, then return the screen. Output that does not visibly change the screen still resets the quiet period.", Annotations: readOnly,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input waitForIdleInput) (*mcp.CallToolResult, ScreenInfo, error) {
		quiet, err := boundedMilliseconds("quietMilliseconds", input.QuietMilliseconds)
		if err != nil {
			return nil, ScreenInfo{}, err
		}
		timeout, err := boundedMilliseconds("timeoutMilliseconds", input.TimeoutMilliseconds)
		if err != nil {
			return nil, ScreenInfo{}, err
		}
		result, err := service.WaitForIdle(ctx, input.SessionID, quiet, timeout)
		return nil, result, err
	})
	mcp.AddTool(server, &mcp.Tool{
		Name: "tuicast_type", Description: "Type literal text into an interactive terminal session. The text is sent exactly and is not logged by TUICast. While recording, set parameter for passwords and other secrets so the recording stores the parameter name instead of the text.", Annotations: changesTerminal,
	}, func(_ context.Context, _ *mcp.CallToolRequest, input typeInput) (*mcp.CallToolResult, actionOutput, error) {
		err := service.Type(input.SessionID, input.Text, input.Parameter)
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
	// Recording outputs contain recursive matcher expressions, which the SDK
	// cannot infer a schema for; schema/workbench-recording.schema.json
	// documents their format.
	mcp.AddTool(server, &mcp.Tool{
		Name: "tuicast_start_recording", Description: "Start recording a session's successful type, press, and wait operations as a replayable TUICast workflow. Screen reads and failed operations are not recorded. Use this once you know the path you want to capture, then repeat it from a known screen.", Annotations: recordingControl,
	}, func(_ context.Context, _ *mcp.CallToolRequest, input sessionInput) (*mcp.CallToolResult, any, error) {
		result, err := service.StartRecording(input.SessionID)
		return nil, result, err
	})
	mcp.AddTool(server, &mcp.Tool{
		Name: "tuicast_stop_recording", Description: "Stop recording a session and return the recorded workflow: versioned JSON with session metadata and ordered steps that map directly onto TUICast SDK calls. Stop before closing the session; closing discards an active recording.", Annotations: recordingControl,
	}, func(_ context.Context, _ *mcp.CallToolRequest, input sessionInput) (*mcp.CallToolResult, any, error) {
		result, err := service.StopRecording(input.SessionID)
		return nil, result, err
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

type waitInput struct {
	SessionID           uint64              `json:"sessionId"`
	Matcher             tuicast.MatcherSpec `json:"matcher"`
	TimeoutMilliseconds int                 `json:"timeoutMilliseconds,omitempty"`
	StableMilliseconds  int                 `json:"stableMilliseconds,omitempty"`
}

// waitInputSchema is written by hand because matcher expressions are
// recursive. It mirrors the Matcher schema in schema/openrpc.json.
const waitInputSchema = `{
  "type": "object",
  "additionalProperties": false,
  "required": ["sessionId", "matcher"],
  "properties": {
    "sessionId": {"type": "integer", "minimum": 1, "description": "TUICast session identifier"},
    "matcher": {"$ref": "#/$defs/matcher"},
    "timeoutMilliseconds": {"type": "integer", "minimum": 0, "maximum": 120000, "description": "timeout in milliseconds; defaults to 10000 and cannot exceed 120000"},
    "stableMilliseconds": {"type": "integer", "minimum": 0, "maximum": 120000, "description": "optional period without host output required after the screen matches"}
  },
  "$defs": {
    "matcher": {
      "description": "Exactly one matcher expression. Rows and columns are zero-based.",
      "oneOf": [
        {"type": "object", "additionalProperties": false, "required": ["contains"], "properties": {"contains": {"type": "string"}}},
        {"type": "object", "additionalProperties": false, "required": ["line"], "properties": {"line": {
          "type": "object", "additionalProperties": false, "required": ["row", "text"],
          "properties": {"row": {"type": "integer", "minimum": 0}, "text": {"type": "string"}}
        }}},
        {"type": "object", "additionalProperties": false, "required": ["cursor"], "properties": {"cursor": {
          "type": "object", "additionalProperties": false, "required": ["column", "row"],
          "properties": {"column": {"type": "integer", "minimum": 0}, "row": {"type": "integer", "minimum": 0}}
        }}},
        {"type": "object", "additionalProperties": false, "required": ["all"], "properties": {"all": {"type": "array", "minItems": 1, "items": {"$ref": "#/$defs/matcher"}}}},
        {"type": "object", "additionalProperties": false, "required": ["any"], "properties": {"any": {"type": "array", "minItems": 1, "items": {"$ref": "#/$defs/matcher"}}}},
        {"type": "object", "additionalProperties": false, "required": ["not"], "properties": {"not": {"$ref": "#/$defs/matcher"}}}
      ]
    }
  }
}`

type waitForIdleInput struct {
	SessionID           uint64 `json:"sessionId" jsonschema:"TUICast session identifier"`
	QuietMilliseconds   int    `json:"quietMilliseconds" jsonschema:"required period without host output, in milliseconds; cannot exceed 120000"`
	TimeoutMilliseconds int    `json:"timeoutMilliseconds,omitempty" jsonschema:"timeout in milliseconds; defaults to 10000 and cannot exceed 120000"`
}

type typeInput struct {
	SessionID uint64 `json:"sessionId" jsonschema:"TUICast session identifier"`
	Text      string `json:"text" jsonschema:"literal text to send exactly as provided"`
	Parameter string `json:"parameter,omitempty" jsonschema:"optional name recorded instead of the text while recording, such as password; letters, digits, underscores, and hyphens only"`
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

func boundedMilliseconds(name string, milliseconds int) (time.Duration, error) {
	if milliseconds < 0 || milliseconds > int(maximumWaitTimeout/time.Millisecond) {
		return 0, fmt.Errorf("parsing MCP wait duration: %s must be between 0 and %d", name, maximumWaitTimeout/time.Millisecond)
	}
	return time.Duration(milliseconds) * time.Millisecond, nil
}

func boolPointer(value bool) *bool {
	return &value
}
