# portifolio

A Linux/macOS portfolio TUI built with Go, Bubble Tea and Lip Gloss. Each configuration file represents one market and one portfolio. The interface is in English.

## Run

```sh
go run ./cmd/portifolio
# Independent portfolio/session:
go run ./cmd/portifolio --config ./my-portfolio.toml --db ./cache.sqlite
```

On first launch, select a built-in market and press Enter. Currency and free data providers are bound to that market:

- United States: USD, Yahoo Finance with Stooq fallback.
- China A-shares: CNY, Eastmoney.

The default page is My Portfolio. Press `a` (Add) to open the searchable asset directory. A scrolling loading bar runs while the directory is loading; search and Esc remain available. Its asset directory is fetched from Nasdaq Trader (US stocks/ETFs) or Eastmoney (China stocks/ETFs, paginated) and persisted to SQLite. Startup displays cached assets first; directories older than 24 hours refresh in the background. In Add, press Ctrl+R to refresh the directory manually. Failed or incomplete requests preserve the existing cached directory. Price columns show the latest available daily close and change, not streaming quotes. Missing or failed prices are shown explicitly. SQLite caches downloaded history.

## Workflow

1. Start in My Portfolio and press `a` to Add a position.
2. Type a symbol or name to fuzzy-search the directory, use Up/Down to select, and Enter to add. The app returns to My Portfolio; new positions start with zero weight. Esc cancels.
3. Select a position and press `e` to set its allocation percentage. Enter saves; Esc cancels. Use `x` to remove a position.
4. Press `2` to analyze the portfolio. Incomplete weights are normalized for analysis. Analysis includes candlestick history, drawdown with axes, target allocation, period returns with percentages and currency gain/loss, weight drift from targets, and per-asset contribution to model returns.

Empty portfolios and partially assigned allocations can be saved. Analysis normalizes incomplete weights and requires sufficient price history. Returns are model results, not actual account profit; fees, taxes and FX conversion are excluded.

## Keys

- `1` My Portfolio, `2` Analysis. Tab switches pages when no card is active; while an Analysis card is active, Tab moves to and activates the next card.
- In Analysis: ↑/↓ selects a card, Enter activates it, and Esc returns to card selection. On active chart cards, ↑/↓ changes the interval and ←/→ pans that chart. `z` cycles day/week/month/quarter/year; PgUp/PgDn scrolls.
- In Analysis, `a` opens the hidden-card list and adds a selected card. `x` removes the selected card after confirmation. `J` / `K` or Shift+↓ / Shift+↑ move it. Card order and visibility persist in the `[analysis]` TOML section.
- On My Portfolio, `e` edits the selected asset target weight. On Analysis, `e` opens portfolio settings (rebalance, initial capital, start date).
- In asset details, `z`/Ctrl+↑↓ changes interval; ←/→ pans one candle.
- Up/Down or j/k: select an asset.
- `a` / `n`: Add (type to search); Enter in Add selects a position; `x`: remove; `e`: edit target percentage.
- `/`: fuzzy search; Enter/Esc finishes search.
- `s`: change analysis start date; `b`: cycle never/monthly/quarterly/yearly rebalance.
- `r`: refresh asset directory and load prices; d/w/m/c: day/week/month/quarter chart aggregation.
- Left/Right: pan analysis by 30 days; Home: return to latest.
- `q` or Ctrl+C: quit.

## Analysis plugins

Analysis cards can be extended with Lua. Files placed in `<config-dir>/plugins/*.lua` load at startup and participate in the same add/remove/reorder workflow as built-in cards. Plugins run in a restricted Lua sandbox and return declarative `text`, `metrics`, `table`, `line`, and `bars` blocks. See [docs/plugins.md](docs/plugins.md) and [examples/plugins/drawdown_summary.lua](examples/plugins/drawdown_summary.lua).

Restart after adding or editing a plugin; hot reload is not implemented yet.

## Proxy

All HTTP calls honor `HTTP_PROXY`, `HTTPS_PROXY`, `ALL_PROXY`, and `NO_PROXY`, including lowercase forms. Scheme-specific proxies take precedence, then `ALL_PROXY`. Proxy selection is made per request; restart after changing environment variables.

## Configuration

The TUI creates and updates one TOML file in the platform config directory under `portifolio/config.toml`. The default SQLite path is `portifolio/cache.sqlite` in that directory; `--db` overrides it independently of `--config`.

```toml
market = "us"

[portfolio]
id = "my-portfolio"
name = "My Portfolio"
market = "us"
base_currency = "USD"
price_currency = "USD"
start_date = "2025-01-01"
rebalance = "quarterly"
initial_capital = 10000.0

[[portfolio.positions]]
symbol = "VTI"
market = "us"
kind = "etf"
allocation = 1.0
currency = "USD"
```

The old multiple-portfolio schema is no longer accepted. Use a fresh `--config` path to initialize; existing files are not automatically migrated or overwritten. Provider implementations remain behind a Go interface, but users no longer configure provider chains during initialization.

## Check

```sh
go test ./...
go vet ./...
go build ./cmd/portifolio
```
