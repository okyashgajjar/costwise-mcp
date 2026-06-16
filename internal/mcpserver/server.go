package mcpserver

import (
	"github.com/mark3labs/mcp-go/server"

	"github.com/okyashgajjar/costaffective-mcp/internal/skill"
)

func NewServer() *server.MCPServer {
	s := server.NewMCPServer(
		"CostAffective Code Intelligence",
		"1.0.0",
		server.WithToolCapabilities(true),
		// Cross-IDE, zero-install session-awareness guidance: every MCP client
		// auto-loads this on connect, steering the model toward the
		// cache-reducing tools. See internal/skill.
		server.WithInstructions(skill.Instructions()),
	)

	RegisterTools(s)
	return s
}
