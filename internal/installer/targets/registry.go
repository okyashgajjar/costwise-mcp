package targets

import "github.com/okyashgajjar/costaffective-mcp/internal/installer"

func init() {
	installer.RegisterTarget(&ClaudeTarget{})
	installer.RegisterTarget(&CursorTarget{})
	installer.RegisterTarget(&OpencodeTarget{})
	installer.RegisterTarget(&CodexTarget{})
	installer.RegisterTarget(&AntigravityTarget{})
}
