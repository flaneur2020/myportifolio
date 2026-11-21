# Analysis plugins

Analysis cards can be extended with Lua files. Plugins are loaded at startup from:

```text
<directory containing config.toml>/plugins/*.lua
```

Plugin cards participate in the Analysis card order. New cards are appended unless an order is configured. Restart the app after editing a plugin. There is no hot reload yet.

Card order and hidden cards are stored in TOML:

```toml
[analysis]
order = ["information", "history", "returns", "drawdown-summary"]
hidden = ["allocation", "drift"]
```

You normally do not need to edit this section manually. Press `a` on Analysis to open the hidden-card list and add a selected card. Use `x` to remove the selected card (with confirmation), and `J` / `K` or Shift+↓ / Shift+↑ to move it.

## Minimal plugin

```lua
return {
  api_version = 1,
  cards = {
    {
      id = "drawdown-summary",
      title = "Drawdown Summary",
      supports_interval = true,
      supports_pan = true,

      render = function(ctx)
        local dd = host.drawdown(ctx.curve)
        local worst = 0
        for i = 1, #dd do
          if dd[i].value < worst then worst = dd[i].value end
        end
        return {
          { type = "metrics", rows = {
            { "Latest index", string.format("%.2f", ctx.curve[#ctx.curve].value) },
            { "Worst drawdown", string.format("%.2f%%", worst) },
          }},
          { type = "line", title = "Drawdown (%)", points = dd, height = 6 },
        }
      end,
    },
  },
}
```

Copy it to:

```text
~/.config/portifolio/plugins/drawdown_summary.lua
```

## Card fields

| Field | Description |
|---|---|
| `id` | Stable unique ID, letters/numbers/`-`/`_` |
| `title` | Card heading |
| `supports_interval` | Enables Up/Down interval changes while active |
| `supports_pan` | Enables Left/Right paging; `ctx.offset` is supplied |
| `render` | Returns one block or an array of blocks |

Cards are selected with Up/Down, activated with Enter, and deactivated with Esc. Active cards can use Tab to move to the next active card.

## Context

`ctx` contains:

- `portfolio`: name, market, base_currency, start_date, rebalance, initial_capital
- `positions`: array of symbol, market, kind, allocation, currency
- `curve`: normalized portfolio points `{ date = "YYYY-MM-DD", value = number }`
- `bars`: map from symbol to arrays of `{ date, open, high, low, close, adjusted_close, currency, provider }`
- `grain`: `day`, `week`, `month`, `quarter`, or `year`
- `offset`: selected card's paging offset
- `width`: terminal content width

Host helpers:

- `host.drawdown(points)` returns drawdown points in percent.
- `host.returns(points, grain)` returns period returns in percent.

## Blocks

### Text

```lua
{ type = "text", text = "Any explanatory text" }
```

### Metrics

```lua
{ type = "metrics", rows = {
  { "Total return", "+12.4%" },
  { "Volatility", "18.2%" },
}}
```

### Table

```lua
{ type = "table",
  headers = { "Symbol", "Weight" },
  rows = {
    { "VTI", "25.0%" },
    { "QQQ", "25.0%" },
  },
}
```

### Line

```lua
{ type = "line", title = "Series", points = ctx.curve, height = 6 }
```

Points must contain `date` and `value`. The host draws a compact terminal chart.

### Bars

```lua
{ type = "bars", rows = {
  { label = "VTI", value = 5.2 },
  { label = "QQQ", value = -3.1 },
}}
```

Values are percentages by default. Add `suffix = ""` to omit `%`.

## Sandbox and limits

Plugins run in a restricted Lua state:

- no `os`, `io`, `package`, `debug`, `require`, `load`, `loadfile`, or `dofile`
- no `print`, to prevent terminal corruption
- 200 ms execution timeout per render
- ANSI/control characters are stripped
- panics and Lua errors are contained to the individual card

Plugins are local code and should still be treated as trusted. There is currently no memory quota or hot reload.

## Errors

Invalid syntax, unsupported API versions, duplicate IDs, and top-level sandbox violations prevent that card from loading. Load errors are shown at the end of the Analysis page.
