package luart

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	lua "github.com/yuin/gopher-lua"
	"github.com/yuin/gopher-lua/parse"
	"portifolio/core"
)

const APIVersion = 1

type HostOptions struct {
	HTTPClient  *http.Client
	URLRewrites map[string]string
	CardTimeout time.Duration
}

// Host loads, compiles, validates, and registers Lua plugins into a core.Kernel.
type Host struct {
	client      *http.Client
	urlRewrites map[string]string
	cardTimeout time.Duration
}

func NewHost(opts HostOptions) *Host {
	client := opts.HTTPClient
	if client == nil {
		client = HTTPClient(20 * time.Second)
	}
	timeout := opts.CardTimeout
	if timeout <= 0 {
		timeout = DefaultCardTimeout
	}
	rewrites := map[string]string{}
	for k, v := range opts.URLRewrites {
		rewrites[k] = v
	}
	return &Host{
		client:      client,
		urlRewrites: rewrites,
		cardTimeout: timeout,
	}
}

// LoadFS loads all *.lua scripts from an embedded or virtual fs.FS in sorted order.
func (h *Host) LoadFS(k *core.Kernel, fsys fs.FS, dir string) []string {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		msg := fmt.Sprintf("plugins: %v", err)
		k.AddPluginError(msg)
		return []string{msg}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	var problems []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".lua") {
			continue
		}
		path := entry.Name()
		if dir != "" && dir != "." {
			path = dir + "/" + entry.Name()
		}
		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			msg := entry.Name() + ": " + err.Error()
			problems = append(problems, msg)
			k.AddPluginError(msg)
			continue
		}
		for _, errMsg := range h.LoadBytes(k, entry.Name(), data) {
			problems = append(problems, errMsg)
			k.AddPluginError(errMsg)
		}
	}
	return problems
}

// LoadDir loads all *.lua files from a filesystem directory. Missing directories are ignored.
func (h *Host) LoadDir(k *core.Kernel, dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		msg := fmt.Sprintf("plugins: %v", err)
		k.AddPluginError(msg)
		return []string{msg}
	}
	var problems []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".lua") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		for _, errMsg := range h.LoadFile(k, path) {
			problems = append(problems, errMsg)
			k.AddPluginError(errMsg)
		}
	}
	return problems
}

// LoadForConfig loads <dir-of-config>/plugins/*.lua into k.
func (h *Host) LoadForConfig(k *core.Kernel, configPath string) []string {
	if strings.TrimSpace(configPath) == "" {
		return nil
	}
	return h.LoadDir(k, filepath.Join(filepath.Dir(configPath), "plugins"))
}

// LoadFile reads and registers a single Lua plugin file.
func (h *Host) LoadFile(k *core.Kernel, path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return []string{filepath.Base(path) + ": " + err.Error()}
	}
	return h.LoadBytes(k, filepath.Base(path), data)
}

// LoadBytes compiles and registers a Lua plugin module into k.
func (h *Host) LoadBytes(k *core.Kernel, name string, src []byte) []string {
	chunk, err := parse.Parse(bytes.NewReader(src), name)
	if err != nil {
		return []string{name + ": " + err.Error()}
	}
	proto, err := lua.Compile(chunk, name)
	if err != nil {
		return []string{name + ": " + err.Error()}
	}

	// Evaluate top-level manifest in a pure sandbox (no network at load time).
	L := newSandboxedState(sandboxOptions{allowHTTP: false})
	defer L.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	L.SetContext(ctx)
	defer L.RemoveContext()

	root, err := execProto(L, proto)
	if err != nil {
		return []string{name + ": " + err.Error()}
	}
	if int(lua.LVAsNumber(root.RawGetString("api_version"))) != APIVersion {
		return []string{fmt.Sprintf("%s: unsupported api_version (expected %d)", name, APIVersion)}
	}

	allowHTTP := false
	for _, capName := range luaStringArray(root.RawGetString("capabilities")) {
		if strings.EqualFold(capName, "http") {
			allowHTTP = true
		}
	}
	allowedHosts := map[string]bool{}
	for _, hostName := range luaStringArray(root.RawGetString("allowed_hosts")) {
		hostName = strings.ToLower(strings.TrimSpace(hostName))
		if hostName != "" {
			allowedHosts[hostName] = true
		}
	}

	var problems []string
	registered := 0

	// 1. Markets
	if marketsTbl, ok := root.RawGetString("markets").(*lua.LTable); ok {
		for i := 1; i <= marketsTbl.Len(); i++ {
			tbl, ok := marketsTbl.RawGetInt(i).(*lua.LTable)
			if !ok {
				continue
			}
			spec := core.MarketSpec{
				ID:                strings.TrimSpace(lua.LVAsString(tbl.RawGetString("id"))),
				Name:              core.SanitizeLine(lua.LVAsString(tbl.RawGetString("name"))),
				Description:       core.SanitizeLine(lua.LVAsString(tbl.RawGetString("description"))),
				Currency:          strings.TrimSpace(lua.LVAsString(tbl.RawGetString("currency"))),
				AssetKinds:        luaStringArray(tbl.RawGetString("asset_kinds")),
				PriceProviders:    luaStringArray(tbl.RawGetString("price_providers")),
				DirectoryProvider: strings.TrimSpace(lua.LVAsString(tbl.RawGetString("directory_provider"))),
				CommonSymbols:     luaStringArray(tbl.RawGetString("common_symbols")),
			}
			override := lua.LVAsBool(tbl.RawGetString("override"))
			if err := k.RegisterMarket(spec, override); err != nil {
				problems = append(problems, name+": "+err.Error())
				continue
			}
			registered++
		}
	}

	// 2. Price providers
	if providersTbl, ok := root.RawGetString("providers").(*lua.LTable); ok {
		for i := 1; i <= providersTbl.Len(); i++ {
			tbl, ok := providersTbl.RawGetInt(i).(*lua.LTable)
			if !ok {
				continue
			}
			pName := strings.TrimSpace(lua.LVAsString(tbl.RawGetString("name")))
			if _, ok := tbl.RawGetString("history").(*lua.LFunction); !ok || pName == "" {
				problems = append(problems, fmt.Sprintf("%s: invalid price provider at index %d", name, i))
				continue
			}
			p := &LuaPriceProvider{
				name:         pName,
				markets:      luaStringArray(tbl.RawGetString("markets")),
				proto:        proto,
				idx:          i,
				allowHTTP:    allowHTTP,
				allowedHosts: allowedHosts,
				client:       h.client,
				urlRewrites:  h.urlRewrites,
			}
			override := lua.LVAsBool(tbl.RawGetString("override"))
			if err := k.RegisterPriceProvider(p, override); err != nil {
				problems = append(problems, name+": "+err.Error())
				continue
			}
			registered++
		}
	}

	// 3. Directory providers
	if dirsTbl, ok := root.RawGetString("directories").(*lua.LTable); ok {
		for i := 1; i <= dirsTbl.Len(); i++ {
			tbl, ok := dirsTbl.RawGetInt(i).(*lua.LTable)
			if !ok {
				continue
			}
			dName := strings.TrimSpace(lua.LVAsString(tbl.RawGetString("name")))
			if _, ok := tbl.RawGetString("assets").(*lua.LFunction); !ok || dName == "" {
				problems = append(problems, fmt.Sprintf("%s: invalid directory provider at index %d", name, i))
				continue
			}
			d := &LuaDirectoryProvider{
				name:         dName,
				markets:      luaStringArray(tbl.RawGetString("markets")),
				proto:        proto,
				idx:          i,
				allowHTTP:    allowHTTP,
				allowedHosts: allowedHosts,
				client:       h.client,
				urlRewrites:  h.urlRewrites,
			}
			override := lua.LVAsBool(tbl.RawGetString("override"))
			if err := k.RegisterDirectoryProvider(d, override); err != nil {
				problems = append(problems, name+": "+err.Error())
				continue
			}
			registered++
		}
	}

	// 4. Analysis cards
	if cardsTbl, ok := root.RawGetString("cards").(*lua.LTable); ok {
		for i := 1; i <= cardsTbl.Len(); i++ {
			tbl, ok := cardsTbl.RawGetInt(i).(*lua.LTable)
			if !ok {
				continue
			}
			id := strings.TrimSpace(lua.LVAsString(tbl.RawGetString("id")))
			title := core.SanitizeLine(lua.LVAsString(tbl.RawGetString("title")))
			if _, ok := tbl.RawGetString("render").(*lua.LFunction); !ok || id == "" || title == "" {
				continue
			}
			card := &LuaCard{
				meta: core.CardMeta{
					ID:               id,
					Title:            title,
					SupportsInterval: lua.LVAsBool(tbl.RawGetString("supports_interval")),
					SupportsPan:      lua.LVAsBool(tbl.RawGetString("supports_pan")),
				},
				proto:   proto,
				cardIdx: i,
				timeout: h.cardTimeout,
			}
			override := lua.LVAsBool(tbl.RawGetString("override"))
			if err := k.RegisterCard(card, override); err != nil {
				problems = append(problems, name+": "+err.Error())
				continue
			}
			registered++
		}
	}

	// 5. Themes
	if themesTbl, ok := root.RawGetString("themes").(*lua.LTable); ok {
		for i := 1; i <= themesTbl.Len(); i++ {
			tbl, ok := themesTbl.RawGetInt(i).(*lua.LTable)
			if !ok {
				continue
			}
			spec := core.ThemeSpec{
				ID:           strings.TrimSpace(lua.LVAsString(tbl.RawGetString("id"))),
				Name:         core.SanitizeLine(lua.LVAsString(tbl.RawGetString("name"))),
				Accent:       strings.TrimSpace(lua.LVAsString(tbl.RawGetString("accent"))),
				Muted:        strings.TrimSpace(lua.LVAsString(tbl.RawGetString("muted"))),
				Highlight:    strings.TrimSpace(lua.LVAsString(tbl.RawGetString("highlight"))),
				Gain:         strings.TrimSpace(lua.LVAsString(tbl.RawGetString("gain"))),
				GainFill:     strings.TrimSpace(lua.LVAsString(tbl.RawGetString("gain_fill"))),
				Loss:         strings.TrimSpace(lua.LVAsString(tbl.RawGetString("loss"))),
				LossFill:     strings.TrimSpace(lua.LVAsString(tbl.RawGetString("loss_fill"))),
				Neutral:      strings.TrimSpace(lua.LVAsString(tbl.RawGetString("neutral"))),
				BorderNormal: strings.TrimSpace(lua.LVAsString(tbl.RawGetString("border_normal"))),
				DriftBar:     strings.TrimSpace(lua.LVAsString(tbl.RawGetString("drift_bar"))),
				Series:       luaStringArray(tbl.RawGetString("series")),
			}
			override := lua.LVAsBool(tbl.RawGetString("override"))
			if err := k.RegisterTheme(spec, override); err != nil {
				problems = append(problems, name+": "+err.Error())
				continue
			}
			registered++
		}
	}

	if registered == 0 && len(problems) == 0 {
		problems = append(problems, name+": plugin registered no markets, providers, directories, cards, or themes")
	}
	return problems
}

func execProto(L *lua.LState, proto *lua.FunctionProto) (*lua.LTable, error) {
	fn := L.NewFunctionFromProto(proto)
	L.Push(fn)
	if err := L.PCall(0, 1, nil); err != nil {
		return nil, err
	}
	root, ok := L.Get(-1).(*lua.LTable)
	L.Pop(1)
	if !ok {
		return nil, fmt.Errorf("must return a plugin table")
	}
	return root, nil
}
