package builtin

import (
	"embed"

	"portifolio/core"
	luart "portifolio/runtime/lua"
)

//go:embed lua/*.lua
var luaFS embed.FS

// Register loads all built-in embedded Lua plugins (markets, providers,
// directories, analysis cards, and default theme) into k.
func Register(k *core.Kernel, host *luart.Host) []string {
	if host == nil {
		host = luart.NewHost(luart.HostOptions{})
	}
	return host.LoadFS(k, luaFS, "lua")
}
