package mcpserver_test

import (
	"context"
	"encoding/json"
	"io"
	"testing"

	"github.com/castingcode/tuicast/mcpserver"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	. "github.com/smartystreets/goconvey/convey"
)

func TestMCPProtocol(t *testing.T) {
	Convey("MCP advertises the bounded TUICast tool surface and structured profile metadata", t, func() {
		service := newService(func(_ context.Context, remote io.ReadWriteCloser) {
			_, _ = remote.Write([]byte("MCP READY"))
			_, _ = io.Copy(remote, remote)
		})
		defer service.Close()
		server, err := mcpserver.NewMCPServer(service, "test")
		So(err, ShouldBeNil)

		serverTransport, clientTransport := mcp.NewInMemoryTransports()
		serverSession, err := server.Connect(context.Background(), serverTransport, nil)
		So(err, ShouldBeNil)
		client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "test"}, nil)
		clientSession, err := client.Connect(context.Background(), clientTransport, nil)
		So(err, ShouldBeNil)
		defer func() {
			_ = clientSession.Close()
			_ = serverSession.Wait()
		}()

		tools, err := clientSession.ListTools(context.Background(), nil)
		So(err, ShouldBeNil)
		So(tools.Tools, ShouldHaveLength, 9)
		names := make([]string, 0, len(tools.Tools))
		for _, tool := range tools.Tools {
			names = append(names, tool.Name)
		}
		So(names, ShouldContain, "tuicast_list_profiles")
		So(names, ShouldContain, "tuicast_connect")
		So(names, ShouldContain, "tuicast_wait_for_text")
		So(names, ShouldContain, "tuicast_close_connection")
		for _, tool := range tools.Tools {
			if tool.Name == "tuicast_connect" {
				schema, marshalErr := json.Marshal(tool.InputSchema)
				So(marshalErr, ShouldBeNil)
				So(string(schema), ShouldContainSubstring, "profile")
				So(string(schema), ShouldNotContainSubstring, "address")
			}
		}

		result, err := clientSession.CallTool(context.Background(), &mcp.CallToolParams{Name: "tuicast_list_profiles"})
		So(err, ShouldBeNil)
		So(result.IsError, ShouldBeFalse)
		encoded, err := json.Marshal(result.StructuredContent)
		So(err, ShouldBeNil)
		So(string(encoded), ShouldContainSubstring, `"name":"warehouse"`)
		So(string(encoded), ShouldNotContainSubstring, "address")
		So(string(encoded), ShouldNotContainSubstring, "password")

		result, err = clientSession.CallTool(context.Background(), &mcp.CallToolParams{
			Name: "tuicast_connect", Arguments: map[string]any{"profile": "warehouse"},
		})
		So(err, ShouldBeNil)
		So(result.IsError, ShouldBeFalse)
		var connection mcpserver.ConnectionInfo
		decodeStructured(result, &connection)
		So(connection.ConnectionID, ShouldNotEqual, uint64(0))

		result, err = clientSession.CallTool(context.Background(), &mcp.CallToolParams{
			Name: "tuicast_open_session", Arguments: map[string]any{"connectionId": connection.ConnectionID},
		})
		So(err, ShouldBeNil)
		So(result.IsError, ShouldBeFalse)
		var session mcpserver.SessionInfo
		decodeStructured(result, &session)
		So(session.SessionID, ShouldNotEqual, uint64(0))

		result, err = clientSession.CallTool(context.Background(), &mcp.CallToolParams{
			Name: "tuicast_wait_for_text", Arguments: map[string]any{
				"sessionId": session.SessionID, "text": "MCP READY", "timeoutMilliseconds": 1000,
			},
		})
		So(err, ShouldBeNil)
		So(result.IsError, ShouldBeFalse)
		var screen mcpserver.ScreenInfo
		decodeStructured(result, &screen)
		So(screen.Text, ShouldStartWith, "MCP READY")

		result, err = clientSession.CallTool(context.Background(), &mcp.CallToolParams{
			Name: "tuicast_close_connection", Arguments: map[string]any{"connectionId": connection.ConnectionID},
		})
		So(err, ShouldBeNil)
		So(result.IsError, ShouldBeFalse)

		result, err = clientSession.CallTool(context.Background(), &mcp.CallToolParams{
			Name: "tuicast_connect", Arguments: map[string]any{"profile": "unapproved.example:22"},
		})
		So(err, ShouldBeNil)
		So(result.IsError, ShouldBeTrue)
		So(result.Content[0].(*mcp.TextContent).Text, ShouldContainSubstring, "unknown profile")
	})
}

func decodeStructured(result *mcp.CallToolResult, target any) {
	encoded, err := json.Marshal(result.StructuredContent)
	So(err, ShouldBeNil)
	So(json.Unmarshal(encoded, target), ShouldBeNil)
}
