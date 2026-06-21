package targets

import "github.com/okyashgajjar/costwise-mcp/internal/installer"

func init() {
	installer.RegisterTarget(&ClaudeTarget{})
	installer.RegisterTarget(&CursorTarget{})
	installer.RegisterTarget(&OpencodeTarget{})
	installer.RegisterTarget(&CodexTarget{})
	installer.RegisterTarget(&AntigravityTarget{})
}
