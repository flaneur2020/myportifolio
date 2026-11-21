package luart

import (
	"context"
	"fmt"
	"strings"
	"time"

	lua "github.com/yuin/gopher-lua"
	"portifolio/core"
)

const DefaultCardTimeout = 200 * time.Millisecond

// LuaCard implements core.Card by executing a precompiled Lua chunk inside a
// fresh pure-compute sandbox on each evaluation (race-free and memory-isolated).
type LuaCard struct {
	meta    core.CardMeta
	proto   *lua.FunctionProto
	cardIdx int
	timeout time.Duration
}

func (c *LuaCard) Meta() core.CardMeta { return c.meta }

func (c *LuaCard) Evaluate(in core.CardInput) (res core.CardResult, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			res = core.CardResult{}
			err = fmt.Errorf("%v", recovered)
		}
	}()
	timeout := c.timeout
	if timeout <= 0 {
		timeout = DefaultCardTimeout
	}
	L := newSandboxedState(sandboxOptions{allowHTTP: false})
	defer L.Close()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	L.SetContext(ctx)
	defer L.RemoveContext()

	root, err := execProto(L, c.proto)
	if err != nil {
		return core.CardResult{}, fmt.Errorf("%s", core.SanitizeText(err.Error()))
	}
	cardsTbl, ok := root.RawGetString("cards").(*lua.LTable)
	if !ok {
		return core.CardResult{}, fmt.Errorf("plugin missing cards table")
	}
	cardTbl, ok := cardsTbl.RawGetInt(c.cardIdx).(*lua.LTable)
	if !ok {
		return core.CardResult{}, fmt.Errorf("card %q not found", c.meta.ID)
	}
	renderFn, ok := cardTbl.RawGetString("render").(*lua.LFunction)
	if !ok {
		return core.CardResult{}, fmt.Errorf("card %q missing render function", c.meta.ID)
	}

	L.Push(renderFn)
	L.Push(buildCardContext(L, in))
	if callErr := L.PCall(1, 1, nil); callErr != nil {
		return core.CardResult{}, fmt.Errorf("%s", core.SanitizeText(callErr.Error()))
	}
	resultVal := L.Get(-1)
	L.Pop(1)
	return parseCardResult(resultVal)
}

func buildCardContext(L *lua.LState, in core.CardInput) *lua.LTable {
	d := in.Analysis
	tbl := L.NewTable()

	portfolio := L.NewTable()
	portfolio.RawSetString("id", lua.LString(d.Portfolio.ID))
	portfolio.RawSetString("name", lua.LString(d.Portfolio.Name))
	portfolio.RawSetString("market", lua.LString(d.Portfolio.Market))
	portfolio.RawSetString("base_currency", lua.LString(d.Portfolio.BaseCurrency))
	portfolio.RawSetString("price_currency", lua.LString(d.Portfolio.PriceCurrency))
	portfolio.RawSetString("start_date", lua.LString(d.Portfolio.StartDate))
	portfolio.RawSetString("rebalance", lua.LString(d.Portfolio.Rebalance))
	portfolio.RawSetString("initial_capital", lua.LNumber(d.Portfolio.InitialCapital))
	tbl.RawSetString("portfolio", portfolio)

	positions := L.NewTable()
	for i, pos := range d.Portfolio.Positions {
		pt := L.NewTable()
		pt.RawSetString("symbol", lua.LString(pos.Symbol))
		pt.RawSetString("market", lua.LString(pos.Market))
		pt.RawSetString("kind", lua.LString(pos.Kind))
		pt.RawSetString("allocation", lua.LNumber(pos.Allocation))
		pt.RawSetString("currency", lua.LString(pos.Currency))
		positions.RawSetInt(i+1, pt)
	}
	tbl.RawSetString("positions", positions)

	targets := L.NewTable()
	for i, w := range d.Targets {
		targets.RawSetInt(i+1, lua.LNumber(w))
	}
	tbl.RawSetString("targets", targets)
	tbl.RawSetString("curve", pointsToLua(L, d.Curve))
	tbl.RawSetString("configured_weight", lua.LNumber(d.ConfiguredWeight))
	tbl.RawSetString("effective_start_note", lua.LString(d.EffectiveStartNote))
	tbl.RawSetString("max_drawdown", lua.LNumber(d.MaxDrawdown))
	tbl.RawSetString("capital", lua.LNumber(d.Capital))
	tbl.RawSetString("grain", lua.LString(in.Grain))
	tbl.RawSetString("offset", lua.LNumber(in.Offset))
	tbl.RawSetString("width", lua.LNumber(in.Width))

	// Lazy bars proxy: converts only the requested symbol's bars on demand.
	barsProxy := L.NewTable()
	barsMT := L.NewTable()
	barsMT.RawSetString("__index", L.NewFunction(func(L *lua.LState) int {
		self := L.CheckTable(1)
		symbol := L.CheckString(2)
		if existing := self.RawGetString(symbol); existing != lua.LNil {
			L.Push(existing)
			return 1
		}
		assetBars, ok := in.Bars[symbol]
		if !ok {
			L.Push(lua.LNil)
			return 1
		}
		rows := barsToLua(L, assetBars)
		self.RawSetString(symbol, rows)
		L.Push(rows)
		return 1
	}))
	L.SetMetatable(barsProxy, barsMT)
	tbl.RawSetString("bars", barsProxy)

	// Lazy snapshots proxy on ctx so cards that don't inspect snapshots pay zero cost.
	ctxMT := L.NewTable()
	ctxMT.RawSetString("__index", L.NewFunction(func(L *lua.LState) int {
		self := L.CheckTable(1)
		key := L.CheckString(2)
		if key == "snapshots" {
			snaps := snapshotsToLua(L, d.Snapshots)
			self.RawSetString("snapshots", snaps)
			L.Push(snaps)
			return 1
		}
		L.Push(lua.LNil)
		return 1
	}))
	L.SetMetatable(tbl, ctxMT)
	return tbl
}

func parseCardResult(value lua.LValue) (core.CardResult, error) {
	tbl, ok := value.(*lua.LTable)
	if !ok {
		return core.CardResult{}, fmt.Errorf("render must return a block table")
	}
	// Extended form: { title_suffix = " · DAY", blocks = { ... } }
	if blocksVal := tbl.RawGetString("blocks"); blocksVal != lua.LNil {
		blocks, err := parseBlocks(blocksVal)
		if err != nil {
			return core.CardResult{}, err
		}
		return core.CardResult{
			TitleSuffix: core.SanitizeInline(lua.LVAsString(tbl.RawGetString("title_suffix"))),
			Blocks:      blocks,
		}, nil
	}
	blocks, err := parseBlocks(tbl)
	if err != nil {
		return core.CardResult{}, err
	}
	return core.CardResult{Blocks: blocks}, nil
}

func parseBlocks(value lua.LValue) ([]core.Block, error) {
	tbl, ok := value.(*lua.LTable)
	if !ok {
		return nil, fmt.Errorf("render must return a block table")
	}
	var raw []*lua.LTable
	if lua.LVAsString(tbl.RawGetString("type")) != "" {
		raw = append(raw, tbl)
	} else {
		for i := 1; i <= tbl.Len() && i <= 64; i++ {
			if block, ok := tbl.RawGetInt(i).(*lua.LTable); ok {
				raw = append(raw, block)
			}
		}
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("render returned no blocks")
	}
	out := make([]core.Block, 0, len(raw))
	for _, block := range raw {
		out = append(out, parseBlock(block))
	}
	return out, nil
}

func parseTone(value lua.LValue) core.Tone {
	switch strings.ToLower(strings.TrimSpace(lua.LVAsString(value))) {
	case "muted":
		return core.ToneMuted
	case "accent":
		return core.ToneAccent
	case "highlight":
		return core.ToneHighlight
	case "gain":
		return core.ToneGain
	case "loss":
		return core.ToneLoss
	case "neutral":
		return core.ToneNeutral
	default:
		return core.ToneDefault
	}
}

func parseSpans(value lua.LValue) []core.TextSpan {
	tbl, ok := value.(*lua.LTable)
	if !ok {
		return nil
	}
	out := make([]core.TextSpan, 0, tbl.Len())
	for i := 1; i <= tbl.Len() && i <= 32; i++ {
		item := tbl.RawGetInt(i)
		switch v := item.(type) {
		case lua.LString:
			out = append(out, core.TextSpan{Text: core.SanitizeInline(string(v))})
		case *lua.LTable:
			text := core.SanitizeInline(lua.LVAsString(v.RawGetString("text")))
			if text == "" && v.RawGetInt(1) != lua.LNil {
				text = core.SanitizeInline(lua.LVAsString(v.RawGetInt(1)))
			}
			tone := parseTone(v.RawGetString("tone"))
			if tone == core.ToneDefault && v.RawGetInt(2) != lua.LNil {
				tone = parseTone(v.RawGetInt(2))
			}
			out = append(out, core.TextSpan{Text: text, Tone: tone})
		}
	}
	return out
}

func parseBlock(block *lua.LTable) core.Block {
	kind := strings.ToLower(lua.LVAsString(block.RawGetString("type")))
	switch core.BlockKind(kind) {
	case core.BlockText:
		return core.Block{
			Kind: core.BlockText,
			Text: core.SanitizeText(lua.LVAsString(block.RawGetString("text"))),
			Tone: parseTone(block.RawGetString("tone")),
		}
	case core.BlockSpans:
		return core.Block{
			Kind:  core.BlockSpans,
			Spans: parseSpans(block.RawGetString("spans")),
		}
	case core.BlockSpacer:
		return core.Block{Kind: core.BlockSpacer}
	case core.BlockMetrics:
		return core.Block{
			Kind:       core.BlockMetrics,
			LabelWidth: int(lua.LVAsNumber(block.RawGetString("label_width"))),
			MetricRows: parseMetricRows(block.RawGetString("rows")),
		}
	case core.BlockTable:
		return core.Block{
			Kind:      core.BlockTable,
			Headers:   luaStringArray(block.RawGetString("headers")),
			TableRows: parseTableRows(block.RawGetString("rows")),
		}
	case core.BlockLine:
		var points []core.Point
		if tbl, ok := block.RawGetString("points").(*lua.LTable); ok {
			points = luaPoints(tbl)
		}
		height := int(lua.LVAsNumber(block.RawGetString("height")))
		if height <= 0 {
			height = 6
		}
		return core.Block{
			Kind:   core.BlockLine,
			Title:  core.SanitizeLine(lua.LVAsString(block.RawGetString("title"))),
			Points: points,
			Height: height,
		}
	case core.BlockCandles:
		var bars []core.Bar
		if tbl, ok := block.RawGetString("bars").(*lua.LTable); ok {
			bars = luaBars(tbl)
		}
		height := int(lua.LVAsNumber(block.RawGetString("height")))
		if height <= 0 {
			height = 12
		}
		grain := core.Grain(lua.LVAsString(block.RawGetString("grain")))
		if grain == "" {
			grain = core.GrainDay
		}
		return core.Block{
			Kind:   core.BlockCandles,
			Title:  core.SanitizeLine(lua.LVAsString(block.RawGetString("title"))),
			Bars:   bars,
			Grain:  grain,
			Height: height,
		}
	case core.BlockDrawdown:
		var points []core.Point
		if tbl, ok := block.RawGetString("points").(*lua.LTable); ok {
			points = luaPoints(tbl)
		}
		height := int(lua.LVAsNumber(block.RawGetString("height")))
		if height <= 0 {
			height = 7
		}
		return core.Block{
			Kind:   core.BlockDrawdown,
			Title:  core.SanitizeLine(lua.LVAsString(block.RawGetString("title"))),
			Points: points,
			Height: height,
		}
	case core.BlockReturnBars:
		var points []core.Point
		if tbl, ok := block.RawGetString("points").(*lua.LTable); ok {
			points = luaPoints(tbl)
		}
		height := int(lua.LVAsNumber(block.RawGetString("height")))
		if height <= 0 {
			height = 7
		}
		return core.Block{
			Kind:     core.BlockReturnBars,
			Points:   points,
			Height:   height,
			Capital:  float64(lua.LVAsNumber(block.RawGetString("capital"))),
			Currency: core.SanitizeLine(lua.LVAsString(block.RawGetString("currency"))),
		}
	case core.BlockStackedBar:
		return core.Block{
			Kind:        core.BlockStackedBar,
			StackedRows: parseStackedRows(block.RawGetString("rows")),
		}
	case core.BlockDriftHistory:
		var points []core.Point
		if tbl, ok := block.RawGetString("points").(*lua.LTable); ok {
			points = luaPoints(tbl)
		}
		height := int(lua.LVAsNumber(block.RawGetString("height")))
		if height <= 0 {
			height = 4
		}
		return core.Block{
			Kind:   core.BlockDriftHistory,
			Title:  core.SanitizeLine(lua.LVAsString(block.RawGetString("title"))),
			Points: points,
			Height: height,
		}
	case core.BlockBars:
		return core.Block{
			Kind:          core.BlockBars,
			BarRows:       parseBarRows(block.RawGetString("rows")),
			BarScale:      float64(lua.LVAsNumber(block.RawGetString("scale"))),
			BarMinOne:     lua.LVAsBool(block.RawGetString("min_one")),
			BarWidthCap:   int(lua.LVAsNumber(block.RawGetString("bar_width_cap"))),
			BarLabelWidth: int(lua.LVAsNumber(block.RawGetString("label_width"))),
		}
	default:
		return core.Block{
			Kind:    core.BlockUnsupported,
			RawKind: core.SanitizeLine(kind),
		}
	}
}

func parseMetricRows(value lua.LValue) []core.MetricRow {
	tbl, ok := value.(*lua.LTable)
	if !ok {
		return nil
	}
	rows := make([]core.MetricRow, 0, tbl.Len())
	for i := 1; i <= tbl.Len() && i <= 100; i++ {
		row, ok := tbl.RawGetInt(i).(*lua.LTable)
		if !ok {
			continue
		}
		if lua.LVAsBool(row.RawGetString("separator")) || (row.Len() == 0 && row.RawGetString("label") == lua.LNil) {
			rows = append(rows, core.MetricRow{Separator: true})
			continue
		}
		label := core.SanitizeLine(lua.LVAsString(row.RawGetString("label")))
		if label == "" {
			label = core.SanitizeLine(lua.LVAsString(row.RawGetInt(1)))
		}
		val := core.SanitizeLine(lua.LVAsString(row.RawGetString("value")))
		if val == "" && row.RawGetInt(2) != lua.LNil {
			val = core.SanitizeLine(lua.LVAsString(row.RawGetInt(2)))
		}
		tone := parseTone(row.RawGetString("tone"))
		if tone == core.ToneDefault && row.RawGetInt(3) != lua.LNil {
			tone = parseTone(row.RawGetInt(3))
		}
		spans := parseSpans(row.RawGetString("spans"))
		rows = append(rows, core.MetricRow{Label: label, Value: val, Tone: tone, Spans: spans})
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

func parseStackedRows(value lua.LValue) []core.StackedRow {
	tbl, ok := value.(*lua.LTable)
	if !ok {
		return nil
	}
	rows := make([]core.StackedRow, 0, tbl.Len())
	for i := 1; i <= tbl.Len() && i <= 16; i++ {
		row, ok := tbl.RawGetInt(i).(*lua.LTable)
		if !ok {
			continue
		}
		var weights []float64
		if wt, ok := row.RawGetString("weights").(*lua.LTable); ok {
			weights = luaFloatArray(wt)
		}
		rows = append(rows, core.StackedRow{
			Label:   core.SanitizeLine(lua.LVAsString(row.RawGetString("label"))),
			Weights: weights,
		})
	}
	return rows
}

func parseBarRows(value lua.LValue) []core.BarRow {
	tbl, ok := value.(*lua.LTable)
	if !ok {
		return nil
	}
	rows := make([]core.BarRow, 0, tbl.Len())
	for i := 1; i <= tbl.Len() && i <= 100; i++ {
		row, ok := tbl.RawGetInt(i).(*lua.LTable)
		if !ok {
			continue
		}
		label := core.SanitizeLine(lua.LVAsString(row.RawGetString("label")))
		val := float64(lua.LVAsNumber(row.RawGetString("value")))
		suffixVal := row.RawGetString("suffix")
		suffix := "%"
		if suffixVal != lua.LNil {
			suffix = core.SanitizeLine(lua.LVAsString(suffixVal))
		}
		precVal := row.RawGetString("precision")
		prec := 2
		precSet := false
		if precVal != lua.LNil {
			prec = int(lua.LVAsNumber(precVal))
			precSet = true
		}
		swatch := -1
		if sw := row.RawGetString("swatch"); sw != lua.LNil {
			swatch = int(lua.LVAsNumber(sw))
		}
		rows = append(rows, core.BarRow{
			Label:        label,
			Value:        val,
			Suffix:       suffix,
			Precision:    prec,
			PrecisionSet: precSet,
			HideSign:     lua.LVAsBool(row.RawGetString("hide_sign")),
			SwatchIndex:  swatch,
			Prefix:       core.SanitizeInline(lua.LVAsString(row.RawGetString("prefix"))),
			ExtraSpans:   parseSpans(row.RawGetString("extra")),
		})
	}
	return rows
}
