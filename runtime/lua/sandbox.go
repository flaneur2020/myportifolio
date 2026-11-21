package luart

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	lua "github.com/yuin/gopher-lua"
	"portifolio/core"
)

const (
	defaultMaxHTTPBytes = 20 << 20 // 20 MiB
)

type sandboxOptions struct {
	allowHTTP    bool
	allowedHosts map[string]bool
	client       *http.Client
	urlRewrites  map[string]string
}

func newSandboxedState(opts sandboxOptions) *lua.LState {
	L := lua.NewState(lua.Options{SkipOpenLibs: true})
	lua.OpenBase(L)
	lua.OpenTable(L)
	lua.OpenString(L)
	lua.OpenMath(L)

	// Strictly disable filesystem, process, dynamic code loading, and stdout.
	for _, name := range []string{"print", "dofile", "loadfile", "load", "loadstring", "require", "package", "io", "os", "debug"} {
		L.SetGlobal(name, lua.LNil)
	}

	host := L.NewTable()
	registerPureHostFuncs(L, host)
	if opts.allowHTTP {
		registerNetworkHostFuncs(L, host, opts)
	}
	L.SetGlobal("host", host)
	return L
}

func registerPureHostFuncs(L *lua.LState, host *lua.LTable) {
	L.SetFuncs(host, map[string]lua.LGFunction{
		"drawdown": func(L *lua.LState) int {
			points := luaPoints(L.CheckTable(1))
			L.Push(pointsToLua(L, core.DrawdownSeries(points)))
			return 1
		},
		"aggregate_drawdown": func(L *lua.LState) int {
			points := luaPoints(L.CheckTable(1))
			grain := core.Grain(L.OptString(2, string(core.GrainDay)))
			L.Push(pointsToLua(L, core.AggregateDrawdown(points, grain)))
			return 1
		},
		"returns": func(L *lua.LState) int {
			points := luaPoints(L.CheckTable(1))
			grain := core.Grain(L.OptString(2, string(core.GrainMonth)))
			L.Push(pointsToLua(L, core.PeriodReturns(points, grain)))
			return 1
		},
		"gains": func(L *lua.LState) int {
			points := luaPoints(L.CheckTable(1))
			grain := core.Grain(L.OptString(2, string(core.GrainMonth)))
			capital := float64(L.CheckNumber(3))
			L.Push(pointsToLua(L, core.PeriodGains(points, grain, capital)))
			return 1
		},
		"recent_change": func(L *lua.LState) int {
			points := luaPoints(L.CheckTable(1))
			capital := float64(L.CheckNumber(2))
			pct, amt, ok := core.RecentChange(points, capital)
			L.Push(lua.LNumber(pct))
			L.Push(lua.LNumber(amt))
			L.Push(lua.LBool(ok))
			return 3
		},
		"portfolio_candles": func(L *lua.LState) int {
			points := luaPoints(L.CheckTable(1))
			L.Push(barsToLua(L, core.PortfolioCandles(points)))
			return 1
		},
		"aggregate_candles": func(L *lua.LState) int {
			bars := luaBars(L.CheckTable(1))
			grain := core.Grain(L.OptString(2, string(core.GrainDay)))
			L.Push(barsToLua(L, core.AggregateCandles(bars, grain)))
			return 1
		},
		"candle_window": func(L *lua.LState) int {
			bars := luaBars(L.CheckTable(1))
			capacity := L.CheckInt(2)
			offset := L.CheckInt(3)
			L.Push(barsToLua(L, core.CandleWindow(bars, capacity, offset)))
			return 1
		},
		"point_window": func(L *lua.LState) int {
			points := luaPoints(L.CheckTable(1))
			capacity := L.CheckInt(2)
			offset := L.CheckInt(3)
			L.Push(pointsToLua(L, core.PointWindow(points, capacity, offset)))
			return 1
		},
		"period_start": func(L *lua.LState) int {
			d, err := time.Parse("2006-01-02", L.CheckString(1))
			if err != nil {
				L.Push(lua.LString(L.CheckString(1)))
				return 1
			}
			grain := core.Grain(L.OptString(2, string(core.GrainDay)))
			L.Push(lua.LString(core.PeriodStart(d, grain).Format("2006-01-02")))
			return 1
		},
		"max_drift": func(L *lua.LState) int {
			snaps := luaSnapshots(L.CheckTable(1))
			targets := luaFloatArray(L.CheckTable(2))
			grain := core.Grain(L.OptString(3, string(core.GrainDay)))
			L.Push(pointsToLua(L, core.MaxDriftSeries(snaps, targets, grain)))
			return 1
		},
		"contributions": func(L *lua.LState) int {
			rebalance := L.CheckString(1)
			snaps := luaSnapshots(L.CheckTable(2))
			targets := luaFloatArray(L.CheckTable(3))
			grain := core.Grain(L.OptString(4, string(core.GrainDay)))
			periods := core.ContributionHistory(core.Portfolio{Rebalance: rebalance}, snaps, targets, grain)
			tbl := L.NewTable()
			for i, p := range periods {
				row := L.NewTable()
				row.RawSetString("date", lua.LString(p.Date.Format("2006-01-02")))
				row.RawSetString("total", lua.LNumber(p.Total))
				assets := L.NewTable()
				for j, v := range p.Assets {
					assets.RawSetInt(j+1, lua.LNumber(v))
				}
				row.RawSetString("assets", assets)
				tbl.RawSetInt(i+1, row)
			}
			L.Push(tbl)
			return 1
		},
	})

	// Pure URL/time utilities useful for both cards and providers.
	urlTbl := L.NewTable()
	L.SetFuncs(urlTbl, map[string]lua.LGFunction{
		"query_escape": func(L *lua.LState) int {
			L.Push(lua.LString(url.QueryEscape(L.CheckString(1))))
			return 1
		},
		"path_escape": func(L *lua.LState) int {
			L.Push(lua.LString(url.PathEscape(L.CheckString(1))))
			return 1
		},
	})
	host.RawSetString("url", urlTbl)

	timeTbl := L.NewTable()
	L.SetFuncs(timeTbl, map[string]lua.LGFunction{
		"unix": func(L *lua.LState) int {
			d, err := time.Parse("2006-01-02", L.CheckString(1))
			if err != nil {
				L.Push(lua.LNil)
				L.Push(lua.LString(err.Error()))
				return 2
			}
			L.Push(lua.LNumber(d.UTC().Unix()))
			return 1
		},
		"format_unix": func(L *lua.LState) int {
			ts := int64(L.CheckNumber(1))
			L.Push(lua.LString(time.Unix(ts, 0).UTC().Format("2006-01-02")))
			return 1
		},
		"add_days": func(L *lua.LState) int {
			d, err := time.Parse("2006-01-02", L.CheckString(1))
			if err != nil {
				L.Push(lua.LNil)
				L.Push(lua.LString(err.Error()))
				return 2
			}
			days := L.CheckInt(2)
			L.Push(lua.LString(d.AddDate(0, 0, days).Format("2006-01-02")))
			return 1
		},
		"compact": func(L *lua.LState) int {
			d, err := time.Parse("2006-01-02", L.CheckString(1))
			if err != nil {
				L.Push(lua.LString(strings.ReplaceAll(L.CheckString(1), "-", "")))
				return 1
			}
			L.Push(lua.LString(d.Format("20060102")))
			return 1
		},
	})
	host.RawSetString("time", timeTbl)
}

func registerNetworkHostFuncs(L *lua.LState, host *lua.LTable, opts sandboxOptions) {
	httpTbl := L.NewTable()
	L.SetFuncs(httpTbl, map[string]lua.LGFunction{
		"get": func(L *lua.LState) int {
			rawURL := L.CheckString(1)
			parsed, err := url.Parse(rawURL)
			if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
				L.Push(lua.LNil)
				L.Push(lua.LString("invalid http url"))
				return 2
			}
			hostName := strings.ToLower(parsed.Hostname())
			if !opts.allowedHosts[hostName] {
				L.Push(lua.LNil)
				L.Push(lua.LString(fmt.Sprintf("host %q is not in plugin allowed_hosts", hostName)))
				return 2
			}
			actualURL := rewriteURL(rawURL, opts.urlRewrites)
			ctx := L.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, actualURL, nil)
			if err != nil {
				L.Push(lua.LNil)
				L.Push(lua.LString(err.Error()))
				return 2
			}
			maxBytes := int64(defaultMaxHTTPBytes)
			if L.GetTop() >= 2 {
				if optTbl, ok := L.Get(2).(*lua.LTable); ok {
					if headers, ok := optTbl.RawGetString("headers").(*lua.LTable); ok {
						headers.ForEach(func(k, v lua.LValue) {
							req.Header.Set(lua.LVAsString(k), lua.LVAsString(v))
						})
					}
					if limit := int64(lua.LVAsNumber(optTbl.RawGetString("max_bytes"))); limit > 0 && limit <= defaultMaxHTTPBytes {
						maxBytes = limit
					}
				}
			}
			client := opts.client
			if client == nil {
				client = HTTPClient(20 * time.Second)
			}
			resp, err := client.Do(req)
			if err != nil {
				L.Push(lua.LNil)
				L.Push(lua.LString(err.Error()))
				return 2
			}
			defer resp.Body.Close()
			body, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes))
			if err != nil {
				L.Push(lua.LNil)
				L.Push(lua.LString(err.Error()))
				return 2
			}
			out := L.NewTable()
			out.RawSetString("status", lua.LNumber(resp.StatusCode))
			out.RawSetString("status_text", lua.LString(resp.Status))
			out.RawSetString("body", lua.LString(string(body)))
			L.Push(out)
			return 1
		},
	})
	host.RawSetString("http", httpTbl)

	jsonTbl := L.NewTable()
	L.SetFuncs(jsonTbl, map[string]lua.LGFunction{
		"decode": func(L *lua.LState) int {
			raw := L.CheckString(1)
			var val any
			if err := json.Unmarshal([]byte(raw), &val); err != nil {
				L.Push(lua.LNil)
				L.Push(lua.LString(err.Error()))
				return 2
			}
			L.Push(goToLua(L, val))
			return 1
		},
	})
	host.RawSetString("json", jsonTbl)

	csvTbl := L.NewTable()
	L.SetFuncs(csvTbl, map[string]lua.LGFunction{
		"parse": func(L *lua.LState) int {
			raw := L.CheckString(1)
			delim := ','
			useHeader := false
			var wantCols []string
			if L.GetTop() >= 2 {
				if optTbl, ok := L.Get(2).(*lua.LTable); ok {
					if d := lua.LVAsString(optTbl.RawGetString("delimiter")); len(d) > 0 {
						delim = rune(d[0])
					}
					useHeader = lua.LVAsBool(optTbl.RawGetString("header"))
					if cols, ok := optTbl.RawGetString("columns").(*lua.LTable); ok {
						for i := 1; i <= cols.Len(); i++ {
							wantCols = append(wantCols, lua.LVAsString(cols.RawGetInt(i)))
						}
					}
				}
			}
			r := csv.NewReader(strings.NewReader(raw))
			r.Comma = delim
			r.FieldsPerRecord = -1
			if !useHeader {
				rows, err := r.ReadAll()
				if err != nil {
					L.Push(lua.LNil)
					L.Push(lua.LString(err.Error()))
					return 2
				}
				tbl := L.NewTable()
				for i, row := range rows {
					rt := L.NewTable()
					for j, cell := range row {
						rt.RawSetInt(j+1, lua.LString(cell))
					}
					tbl.RawSetInt(i+1, rt)
				}
				L.Push(tbl)
				return 1
			}
			header, err := r.Read()
			if err != nil {
				L.Push(lua.LNil)
				L.Push(lua.LString(err.Error()))
				return 2
			}
			colIndex := map[string]int{}
			headersTbl := L.NewTable()
			for i, h := range header {
				colIndex[h] = i
				headersTbl.RawSetInt(i+1, lua.LString(h))
			}
			type colSpec struct {
				name string
				idx  int
			}
			var selected []colSpec
			if len(wantCols) > 0 {
				for _, name := range wantCols {
					if idx, ok := colIndex[name]; ok {
						selected = append(selected, colSpec{name: name, idx: idx})
					}
				}
			} else {
				for i, name := range header {
					selected = append(selected, colSpec{name: name, idx: i})
				}
			}
			rowsTbl := L.NewTable()
			rowNum := 0
			for {
				record, err := r.Read()
				if err == io.EOF {
					break
				}
				if err != nil {
					L.Push(lua.LNil)
					L.Push(lua.LString(err.Error()))
					return 2
				}
				rowNum++
				rt := L.NewTable()
				if len(record) > 0 {
					rt.RawSetString("_first", lua.LString(record[0]))
				}
				for _, c := range selected {
					if c.idx < len(record) {
						rt.RawSetString(c.name, lua.LString(record[c.idx]))
					} else {
						rt.RawSetString(c.name, lua.LString(""))
					}
				}
				rowsTbl.RawSetInt(rowNum, rt)
			}
			L.Push(rowsTbl)
			L.Push(headersTbl)
			return 2
		},
	})
	host.RawSetString("csv", csvTbl)
}

func rewriteURL(raw string, rewrites map[string]string) string {
	if len(rewrites) == 0 {
		return raw
	}
	// Prefer longest matching prefix for deterministic rewrites.
	prefixes := make([]string, 0, len(rewrites))
	for prefix := range rewrites {
		prefixes = append(prefixes, prefix)
	}
	sort.Slice(prefixes, func(i, j int) bool { return len(prefixes[i]) > len(prefixes[j]) })
	for _, prefix := range prefixes {
		if strings.HasPrefix(raw, prefix) {
			return rewrites[prefix] + strings.TrimPrefix(raw, prefix)
		}
	}
	return raw
}

func goToLua(L *lua.LState, v any) lua.LValue {
	switch x := v.(type) {
	case nil:
		return lua.LNil
	case bool:
		return lua.LBool(x)
	case float64:
		return lua.LNumber(x)
	case string:
		return lua.LString(x)
	case []any:
		tbl := L.NewTable()
		for i, item := range x {
			tbl.RawSetInt(i+1, goToLua(L, item))
		}
		return tbl
	case map[string]any:
		tbl := L.NewTable()
		for k, item := range x {
			tbl.RawSetString(k, goToLua(L, item))
		}
		return tbl
	default:
		return lua.LString(fmt.Sprint(x))
	}
}

func pointsToLua(L *lua.LState, points []core.Point) *lua.LTable {
	tbl := L.NewTable()
	for i, point := range points {
		row := L.NewTable()
		row.RawSetString("date", lua.LString(point.Date.Format("2006-01-02")))
		row.RawSetString("value", lua.LNumber(point.Value))
		tbl.RawSetInt(i+1, row)
	}
	return tbl
}

func luaPoints(tbl *lua.LTable) []core.Point {
	var points []core.Point
	for i := 1; i <= tbl.Len(); i++ {
		row, ok := tbl.RawGetInt(i).(*lua.LTable)
		if !ok {
			continue
		}
		date, err := time.Parse("2006-01-02", lua.LVAsString(row.RawGetString("date")))
		if err != nil {
			continue
		}
		points = append(points, core.Point{Date: date, Value: float64(lua.LVAsNumber(row.RawGetString("value")))})
	}
	sort.Slice(points, func(i, j int) bool { return points[i].Date.Before(points[j].Date) })
	return points
}

func barsToLua(L *lua.LState, bars []core.Bar) *lua.LTable {
	tbl := L.NewTable()
	for i, bar := range bars {
		row := L.NewTable()
		row.RawSetString("date", lua.LString(bar.Date.Format("2006-01-02")))
		row.RawSetString("open", lua.LNumber(bar.Open))
		row.RawSetString("high", lua.LNumber(bar.High))
		row.RawSetString("low", lua.LNumber(bar.Low))
		row.RawSetString("close", lua.LNumber(bar.Close))
		row.RawSetString("adjusted_close", lua.LNumber(bar.AdjustedClose))
		row.RawSetString("currency", lua.LString(bar.Currency))
		row.RawSetString("provider", lua.LString(bar.Source))
		tbl.RawSetInt(i+1, row)
	}
	return tbl
}

func luaBars(tbl *lua.LTable) []core.Bar {
	var out []core.Bar
	for i := 1; i <= tbl.Len(); i++ {
		row, ok := tbl.RawGetInt(i).(*lua.LTable)
		if !ok {
			continue
		}
		date, err := time.Parse("2006-01-02", lua.LVAsString(row.RawGetString("date")))
		if err != nil {
			continue
		}
		closeVal := float64(lua.LVAsNumber(row.RawGetString("close")))
		adj := closeVal
		if v := row.RawGetString("adjusted_close"); v != lua.LNil {
			adj = float64(lua.LVAsNumber(v))
		}
		out = append(out, core.Bar{
			Date:          date,
			Open:          float64(lua.LVAsNumber(row.RawGetString("open"))),
			High:          float64(lua.LVAsNumber(row.RawGetString("high"))),
			Low:           float64(lua.LVAsNumber(row.RawGetString("low"))),
			Close:         closeVal,
			AdjustedClose: adj,
			Currency:      lua.LVAsString(row.RawGetString("currency")),
			Source:        lua.LVAsString(row.RawGetString("provider")),
		})
	}
	return out
}

func snapshotsToLua(L *lua.LState, snaps []core.PortfolioSnapshot) *lua.LTable {
	tbl := L.NewTable()
	for i, s := range snaps {
		row := L.NewTable()
		row.RawSetString("date", lua.LString(s.Date.Format("2006-01-02")))
		row.RawSetString("value", lua.LNumber(s.Value))
		prices := L.NewTable()
		for j, p := range s.Prices {
			prices.RawSetInt(j+1, lua.LNumber(p))
		}
		weights := L.NewTable()
		for j, w := range s.Weights {
			weights.RawSetInt(j+1, lua.LNumber(w))
		}
		row.RawSetString("prices", prices)
		row.RawSetString("weights", weights)
		tbl.RawSetInt(i+1, row)
	}
	return tbl
}

func luaSnapshots(tbl *lua.LTable) []core.PortfolioSnapshot {
	out := make([]core.PortfolioSnapshot, 0, tbl.Len())
	for i := 1; i <= tbl.Len(); i++ {
		row, ok := tbl.RawGetInt(i).(*lua.LTable)
		if !ok {
			continue
		}
		date, err := time.Parse("2006-01-02", lua.LVAsString(row.RawGetString("date")))
		if err != nil {
			continue
		}
		var prices, weights []float64
		if pt, ok := row.RawGetString("prices").(*lua.LTable); ok {
			prices = luaFloatArray(pt)
		}
		if wt, ok := row.RawGetString("weights").(*lua.LTable); ok {
			weights = luaFloatArray(wt)
		}
		out = append(out, core.PortfolioSnapshot{
			Date:    date,
			Value:   float64(lua.LVAsNumber(row.RawGetString("value"))),
			Prices:  prices,
			Weights: weights,
		})
	}
	return out
}

func luaFloatArray(tbl *lua.LTable) []float64 {
	out := make([]float64, 0, tbl.Len())
	for i := 1; i <= tbl.Len(); i++ {
		out = append(out, float64(lua.LVAsNumber(tbl.RawGetInt(i))))
	}
	return out
}

func luaStringArray(value lua.LValue) []string {
	tbl, ok := value.(*lua.LTable)
	if !ok {
		return nil
	}
	out := make([]string, 0, tbl.Len())
	for i := 1; i <= tbl.Len(); i++ {
		out = append(out, core.SanitizeLine(lua.LVAsString(tbl.RawGetInt(i))))
	}
	return out
}
