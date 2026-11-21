package luart

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	lua "github.com/yuin/gopher-lua"
	"portifolio/core"
)

// LuaPriceProvider implements core.PriceProvider with per-call sandboxed LStates
// created from a precompiled FunctionProto (concurrent-safe).
type LuaPriceProvider struct {
	name         string
	markets      []string
	proto        *lua.FunctionProto
	idx          int
	allowHTTP    bool
	allowedHosts map[string]bool
	client       *http.Client
	urlRewrites  map[string]string
}

func (p *LuaPriceProvider) Name() string      { return p.name }
func (p *LuaPriceProvider) Markets() []string { return append([]string(nil), p.markets...) }

func (p *LuaPriceProvider) History(ctx context.Context, pos core.Position, start, end time.Time) (bars []core.Bar, err error) {
	defer func() {
		if r := recover(); r != nil {
			bars = nil
			err = fmt.Errorf("provider %s panic: %v", p.name, r)
		}
	}()
	L := newSandboxedState(sandboxOptions{
		allowHTTP:    p.allowHTTP,
		allowedHosts: p.allowedHosts,
		client:       p.client,
		urlRewrites:  p.urlRewrites,
	})
	defer L.Close()
	if ctx != nil {
		L.SetContext(ctx)
		defer L.RemoveContext()
	}

	root, err := execProto(L, p.proto)
	if err != nil {
		return nil, err
	}
	providersTbl, ok := root.RawGetString("providers").(*lua.LTable)
	if !ok {
		return nil, fmt.Errorf("plugin missing providers table")
	}
	entry, ok := providersTbl.RawGetInt(p.idx).(*lua.LTable)
	if !ok {
		return nil, fmt.Errorf("provider %q not found", p.name)
	}
	fn, ok := entry.RawGetString("history").(*lua.LFunction)
	if !ok {
		return nil, fmt.Errorf("provider %q missing history function", p.name)
	}

	req := L.NewTable()
	req.RawSetString("symbol", lua.LString(pos.Symbol))
	req.RawSetString("market", lua.LString(pos.Market))
	req.RawSetString("kind", lua.LString(pos.Kind))
	req.RawSetString("currency", lua.LString(pos.Currency))
	req.RawSetString("start", lua.LString(start.Format("2006-01-02")))
	req.RawSetString("end", lua.LString(end.Format("2006-01-02")))
	req.RawSetString("start_unix", lua.LNumber(start.Unix()))
	req.RawSetString("end_unix", lua.LNumber(end.Unix()))

	L.Push(fn)
	L.Push(req)
	if callErr := L.PCall(1, 2, nil); callErr != nil {
		return nil, fmt.Errorf("%s", core.SanitizeText(callErr.Error()))
	}
	errVal := L.Get(-1)
	resVal := L.Get(-2)
	L.Pop(2)
	if errVal != lua.LNil {
		return nil, fmt.Errorf("%s", core.SanitizeText(lua.LVAsString(errVal)))
	}
	tbl, ok := resVal.(*lua.LTable)
	if !ok {
		return nil, fmt.Errorf("provider %s returned no prices", p.name)
	}
	out := luaBars(tbl)
	for i := range out {
		if out[i].Source == "" {
			out[i].Source = p.name
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("provider %s returned no valid prices", p.name)
	}
	return out, nil
}

// LuaDirectoryProvider implements core.DirectoryProvider using a precompiled Lua chunk.
type LuaDirectoryProvider struct {
	name         string
	markets      []string
	proto        *lua.FunctionProto
	idx          int
	allowHTTP    bool
	allowedHosts map[string]bool
	client       *http.Client
	urlRewrites  map[string]string
}

func (d *LuaDirectoryProvider) Name() string      { return d.name }
func (d *LuaDirectoryProvider) Markets() []string { return append([]string(nil), d.markets...) }

func (d *LuaDirectoryProvider) Assets(ctx context.Context, market string) (assets []core.Asset, err error) {
	defer func() {
		if r := recover(); r != nil {
			assets = nil
			err = fmt.Errorf("directory %s panic: %v", d.name, r)
		}
	}()
	L := newSandboxedState(sandboxOptions{
		allowHTTP:    d.allowHTTP,
		allowedHosts: d.allowedHosts,
		client:       d.client,
		urlRewrites:  d.urlRewrites,
	})
	defer L.Close()
	if ctx != nil {
		L.SetContext(ctx)
		defer L.RemoveContext()
	}

	root, err := execProto(L, d.proto)
	if err != nil {
		return nil, err
	}
	dirsTbl, ok := root.RawGetString("directories").(*lua.LTable)
	if !ok {
		return nil, fmt.Errorf("plugin missing directories table")
	}
	entry, ok := dirsTbl.RawGetInt(d.idx).(*lua.LTable)
	if !ok {
		return nil, fmt.Errorf("directory %q not found", d.name)
	}
	fn, ok := entry.RawGetString("assets").(*lua.LFunction)
	if !ok {
		return nil, fmt.Errorf("directory %q missing assets function", d.name)
	}

	req := L.NewTable()
	req.RawSetString("market", lua.LString(market))

	L.Push(fn)
	L.Push(req)
	if callErr := L.PCall(1, 2, nil); callErr != nil {
		return nil, fmt.Errorf("%s", core.SanitizeText(callErr.Error()))
	}
	errVal := L.Get(-1)
	resVal := L.Get(-2)
	L.Pop(2)
	if errVal != lua.LNil {
		return nil, fmt.Errorf("%s", core.SanitizeText(lua.LVAsString(errVal)))
	}
	tbl, ok := resVal.(*lua.LTable)
	if !ok {
		return nil, fmt.Errorf("directory %s returned no assets", d.name)
	}
	out := make([]core.Asset, 0, tbl.Len())
	for i := 1; i <= tbl.Len(); i++ {
		row, ok := tbl.RawGetInt(i).(*lua.LTable)
		if !ok {
			continue
		}
		symbol := strings.TrimSpace(lua.LVAsString(row.RawGetString("symbol")))
		name := strings.TrimSpace(lua.LVAsString(row.RawGetString("name")))
		kind := strings.TrimSpace(lua.LVAsString(row.RawGetString("kind")))
		m := strings.TrimSpace(lua.LVAsString(row.RawGetString("market")))
		if m == "" {
			m = market
		}
		if symbol == "" {
			continue
		}
		out = append(out, core.Asset{Symbol: symbol, Name: name, Market: m, Kind: kind})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("directory %s returned empty list", d.name)
	}
	return out, nil
}
