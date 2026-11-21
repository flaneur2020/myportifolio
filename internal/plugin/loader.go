package plugin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	lua "github.com/yuin/gopher-lua"
	"portifolio/internal/config"
	"portifolio/internal/domain"
)

const (
	APIVersion     = 1
	DefaultTimeout = 200 * time.Millisecond
)

type Input struct {
	Portfolio config.Portfolio
	Curve     []domain.Point
	Snapshots []domain.PortfolioSnapshot
	Bars      map[string][]domain.Bar
	Grain     domain.Grain
	Offset    int
	Width     int
}

type Card struct {
	ID               string
	Title            string
	SupportsInterval bool
	SupportsPan      bool
	state            *lua.LState
	render           *lua.LFunction
}

// CloseAll closes every unique Lua state owned by cards.
func CloseAll(cards []*Card) {
	closed := map[*lua.LState]bool{}
	for _, c := range cards {
		if c != nil && c.state != nil && !closed[c.state] {
			c.state.Close()
			closed[c.state] = true
		}
	}
}

func LoadForConfig(configPath string, reservedIDs []string) ([]*Card, []string) {
	if strings.TrimSpace(configPath) == "" {
		return nil, nil
	}
	return LoadDir(filepath.Join(filepath.Dir(configPath), "plugins"), reservedIDs)
}

func LoadDir(dir string, reservedIDs []string) ([]*Card, []string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, []string{fmt.Sprintf("plugins: %v", err)}
	}
	var cards []*Card
	var problems []string
	seen := make(map[string]bool, len(reservedIDs))
	for _, id := range reservedIDs {
		seen[id] = true
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".lua") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		loaded, err := LoadFile(path)
		if err != nil {
			problems = append(problems, entry.Name()+": "+err.Error())
			continue
		}
		var kept []*Card
		for _, card := range loaded {
			if seen[card.ID] {
				problems = append(problems, entry.Name()+": duplicate card id "+card.ID)
				continue
			}
			seen[card.ID] = true
			kept = append(kept, card)
		}
		if len(kept) == 0 {
			CloseAll(loaded)
			continue
		}
		cards = append(cards, kept...)
	}
	return cards, problems
}

func LoadFile(path string) ([]*Card, error) {
	L := sandboxLua()
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	L.SetContext(ctx)
	fn, err := L.LoadFile(path)
	if err != nil {
		L.RemoveContext()
		L.Close()
		return nil, err
	}
	L.Push(fn)
	if err := L.PCall(0, 1, nil); err != nil {
		L.RemoveContext()
		L.Close()
		return nil, err
	}
	L.RemoveContext()
	root, ok := L.Get(-1).(*lua.LTable)
	L.Pop(1)
	if !ok {
		L.Close()
		return nil, fmt.Errorf("must return a plugin table")
	}
	if int(lua.LVAsNumber(root.RawGetString("api_version"))) != APIVersion {
		L.Close()
		return nil, fmt.Errorf("unsupported api_version (expected %d)", APIVersion)
	}
	cardTable, ok := root.RawGetString("cards").(*lua.LTable)
	if !ok || cardTable.Len() == 0 {
		L.Close()
		return nil, fmt.Errorf("no cards defined")
	}
	var cards []*Card
	for i := 1; i <= cardTable.Len(); i++ {
		tbl, ok := cardTable.RawGetInt(i).(*lua.LTable)
		if !ok {
			continue
		}
		id := strings.TrimSpace(lua.LVAsString(tbl.RawGetString("id")))
		title := strings.TrimSpace(lua.LVAsString(tbl.RawGetString("title")))
		render, ok := tbl.RawGetString("render").(*lua.LFunction)
		if id == "" || title == "" || !ok {
			continue
		}
		if !validID(id) {
			L.Close()
			return nil, fmt.Errorf("invalid card id %q", id)
		}
		cards = append(cards, &Card{
			ID:               id,
			Title:            SanitizeLine(title),
			SupportsInterval: lua.LVAsBool(tbl.RawGetString("supports_interval")),
			SupportsPan:      lua.LVAsBool(tbl.RawGetString("supports_pan")),
			state:            L,
			render:           render,
		})
	}
	if len(cards) == 0 {
		L.Close()
		return nil, fmt.Errorf("no valid cards")
	}
	return cards, nil
}

// Evaluate executes the Lua card's render function and returns sanitized UI-agnostic blocks.
func (c *Card) Evaluate(in Input) (blocks []Block, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			blocks = nil
			err = fmt.Errorf("%v", recovered)
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()
	c.state.SetContext(ctx)
	defer c.state.RemoveContext()

	c.state.Push(c.render)
	c.state.Push(c.contextTable(in))
	if callErr := c.state.PCall(1, 1, nil); callErr != nil {
		c.state.SetTop(0)
		return nil, fmt.Errorf("%s", SanitizeText(callErr.Error()))
	}
	result := c.state.Get(-1)
	c.state.Pop(1)
	return parseBlocks(result)
}

func sandboxLua() *lua.LState {
	L := lua.NewState(lua.Options{SkipOpenLibs: true})
	lua.OpenBase(L)
	lua.OpenTable(L)
	lua.OpenString(L)
	lua.OpenMath(L)
	// No filesystem, process, network, package loading, or console output.
	for _, name := range []string{"print", "dofile", "loadfile", "load", "loadstring", "require", "package", "io", "os", "debug"} {
		L.SetGlobal(name, lua.LNil)
	}
	host := L.NewTable()
	L.SetFuncs(host, map[string]lua.LGFunction{
		"drawdown": func(L *lua.LState) int {
			points := luaPoints(L.CheckTable(1))
			L.Push(pointsToLua(L, domain.DrawdownSeries(points)))
			return 1
		},
		"returns": func(L *lua.LState) int {
			points := luaPoints(L.CheckTable(1))
			grain := domain.Grain(L.OptString(2, string(domain.GrainMonth)))
			L.Push(pointsToLua(L, domain.PeriodReturns(points, grain)))
			return 1
		},
	})
	L.SetGlobal("host", host)
	return L
}

func validID(id string) bool {
	for _, r := range id {
		if !(r == '-' || r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			return false
		}
	}
	return id != ""
}

func (c *Card) contextTable(in Input) *lua.LTable {
	L := c.state
	tbl := L.NewTable()
	portfolio := L.NewTable()
	portfolio.RawSetString("name", lua.LString(in.Portfolio.Name))
	portfolio.RawSetString("market", lua.LString(in.Portfolio.Market))
	portfolio.RawSetString("base_currency", lua.LString(in.Portfolio.BaseCurrency))
	portfolio.RawSetString("start_date", lua.LString(in.Portfolio.StartDate))
	portfolio.RawSetString("rebalance", lua.LString(in.Portfolio.Rebalance))
	portfolio.RawSetString("initial_capital", lua.LNumber(in.Portfolio.InitialCapital))
	tbl.RawSetString("portfolio", portfolio)

	positions := L.NewTable()
	for i, pos := range in.Portfolio.Positions {
		pt := L.NewTable()
		pt.RawSetString("symbol", lua.LString(pos.Symbol))
		pt.RawSetString("market", lua.LString(pos.Market))
		pt.RawSetString("kind", lua.LString(pos.Kind))
		pt.RawSetString("allocation", lua.LNumber(pos.Allocation))
		pt.RawSetString("currency", lua.LString(pos.Currency))
		positions.RawSetInt(i+1, pt)
	}
	tbl.RawSetString("positions", positions)
	tbl.RawSetString("curve", pointsToLua(L, in.Curve))

	barsTable := L.NewTable()
	symbols := make([]string, 0, len(in.Bars))
	for symbol := range in.Bars {
		symbols = append(symbols, symbol)
	}
	sort.Strings(symbols)
	for _, symbol := range symbols {
		rows := L.NewTable()
		for i, bar := range in.Bars[symbol] {
			row := L.NewTable()
			row.RawSetString("date", lua.LString(bar.Date.Format("2006-01-02")))
			row.RawSetString("open", lua.LNumber(bar.Open))
			row.RawSetString("high", lua.LNumber(bar.High))
			row.RawSetString("low", lua.LNumber(bar.Low))
			row.RawSetString("close", lua.LNumber(bar.Close))
			row.RawSetString("adjusted_close", lua.LNumber(bar.AdjustedClose))
			row.RawSetString("currency", lua.LString(bar.Currency))
			row.RawSetString("provider", lua.LString(bar.Source))
			rows.RawSetInt(i+1, row)
		}
		barsTable.RawSetString(symbol, rows)
	}
	tbl.RawSetString("bars", barsTable)
	tbl.RawSetString("grain", lua.LString(in.Grain))
	tbl.RawSetString("offset", lua.LNumber(in.Offset))
	tbl.RawSetString("width", lua.LNumber(in.Width))
	return tbl
}

func pointsToLua(L *lua.LState, points []domain.Point) *lua.LTable {
	tbl := L.NewTable()
	for i, point := range points {
		row := L.NewTable()
		row.RawSetString("date", lua.LString(point.Date.Format("2006-01-02")))
		row.RawSetString("value", lua.LNumber(point.Value))
		tbl.RawSetInt(i+1, row)
	}
	return tbl
}

func luaPoints(tbl *lua.LTable) []domain.Point {
	var points []domain.Point
	for i := 1; i <= tbl.Len(); i++ {
		row, ok := tbl.RawGetInt(i).(*lua.LTable)
		if !ok {
			continue
		}
		date, err := time.Parse("2006-01-02", lua.LVAsString(row.RawGetString("date")))
		if err != nil {
			continue
		}
		points = append(points, domain.Point{Date: date, Value: float64(lua.LVAsNumber(row.RawGetString("value")))})
	}
	sort.Slice(points, func(i, j int) bool { return points[i].Date.Before(points[j].Date) })
	return points
}

func parseBlocks(value lua.LValue) ([]Block, error) {
	tbl, ok := value.(*lua.LTable)
	if !ok {
		return nil, fmt.Errorf("render must return a block table")
	}
	var raw []*lua.LTable
	if lua.LVAsString(tbl.RawGetString("type")) != "" {
		raw = append(raw, tbl)
	} else {
		for i := 1; i <= tbl.Len() && i <= 32; i++ {
			if block, ok := tbl.RawGetInt(i).(*lua.LTable); ok {
				raw = append(raw, block)
			}
		}
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("render returned no blocks")
	}
	out := make([]Block, 0, len(raw))
	for _, block := range raw {
		out = append(out, parseBlock(block))
	}
	return out, nil
}

func parseBlock(block *lua.LTable) Block {
	kind := strings.ToLower(lua.LVAsString(block.RawGetString("type")))
	switch BlockKind(kind) {
	case BlockText:
		return Block{
			Kind: BlockText,
			Text: SanitizeText(lua.LVAsString(block.RawGetString("text"))),
		}
	case BlockMetrics:
		return Block{
			Kind:       BlockMetrics,
			MetricRows: parseMetricRows(block.RawGetString("rows")),
		}
	case BlockTable:
		headers := luaStringArray(block.RawGetString("headers"))
		return Block{
			Kind:      BlockTable,
			Headers:   headers,
			TableRows: parseTableRows(block.RawGetString("rows")),
		}
	case BlockLine:
		var points []domain.Point
		if tbl, ok := block.RawGetString("points").(*lua.LTable); ok {
			points = luaPoints(tbl)
		}
		height := int(lua.LVAsNumber(block.RawGetString("height")))
		if height <= 0 {
			height = 6
		}
		return Block{
			Kind:   BlockLine,
			Title:  SanitizeLine(lua.LVAsString(block.RawGetString("title"))),
			Points: points,
			Height: height,
		}
	case BlockBars:
		return Block{
			Kind:    BlockBars,
			BarRows: parseBarRows(block.RawGetString("rows")),
		}
	default:
		return Block{
			Kind:    BlockUnsupported,
			RawKind: SanitizeLine(kind),
		}
	}
}

func parseMetricRows(value lua.LValue) [][2]string {
	tbl, ok := value.(*lua.LTable)
	if !ok {
		return nil
	}
	rows := make([][2]string, 0, tbl.Len())
	for i := 1; i <= tbl.Len() && i <= 100; i++ {
		row, ok := tbl.RawGetInt(i).(*lua.LTable)
		if !ok {
			continue
		}
		label := SanitizeLine(lua.LVAsString(row.RawGetInt(1)))
		val := SanitizeLine(lua.LVAsString(row.RawGetInt(2)))
		rows = append(rows, [2]string{label, val})
	}
	return rows
}

func parseTableRows(value lua.LValue) [][]string {
	tbl, ok := value.(*lua.LTable)
	if !ok {
		return nil
	}
	rows := make([][]string, 0, tbl.Len())
	for i := 1; i <= tbl.Len() && i <= 100; i++ {
		row, ok := tbl.RawGetInt(i).(*lua.LTable)
		if !ok {
			continue
		}
		rows = append(rows, luaStringArray(row))
	}
	return rows
}

func parseBarRows(value lua.LValue) []BarRow {
	tbl, ok := value.(*lua.LTable)
	if !ok {
		return nil
	}
	rows := make([]BarRow, 0, tbl.Len())
	for i := 1; i <= tbl.Len() && i <= 100; i++ {
		row, ok := tbl.RawGetInt(i).(*lua.LTable)
		if !ok {
			continue
		}
		label := SanitizeLine(lua.LVAsString(row.RawGetString("label")))
		val := float64(lua.LVAsNumber(row.RawGetString("value")))
		suffix := SanitizeLine(lua.LVAsString(row.RawGetString("suffix")))
		if suffix == "" {
			suffix = "%"
		}
		rows = append(rows, BarRow{Label: label, Value: val, Suffix: suffix})
	}
	return rows
}

func luaStringArray(value lua.LValue) []string {
	tbl, ok := value.(*lua.LTable)
	if !ok {
		return nil
	}
	out := make([]string, 0, tbl.Len())
	for i := 1; i <= tbl.Len(); i++ {
		out = append(out, SanitizeLine(lua.LVAsString(tbl.RawGetInt(i))))
	}
	return out
}
