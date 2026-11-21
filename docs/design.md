# Portfolio TUI design

## Scope

One session opens one TOML configuration, representing one built-in market and one portfolio. The UI is English and targets Linux/macOS. Initialization asks only for a market. Currency and provider defaults follow the selected market.

- US: USD, Yahoo Finance with Stooq fallback.
- China A-shares: CNY, Eastmoney.

Provider implementations use the existing Go interface. SQLite stores historical price cache; TOML stores favorites, weights and portfolio settings.

## Screens and interaction

The default screen is My Portfolio. Add opens a searchable asset directory: an asset list with favorite indicator, symbol, name, latest available daily close, daily change and target weight. Asset directories come from Nasdaq Trader for US securities and Eastmoney paginated stock/ETF lists for China. They are cached in SQLite by market and symbol with a directory update timestamp. Quotes are daily history, not streaming prices.

Up/Down selects rows. Add directly accepts fuzzy symbol/name queries. Enter in Add persists the selected position with zero weight; `x` on My Portfolio removes it. Highlighted assets show cached historical prices below the list. Missing price data is displayed explicitly.

`1` opens My Portfolio; `2` opens Analysis. Tab switches between these pages. The standalone Market page is removed. `a` opens Add: type a fuzzy query, Up/Down selects, Enter adds and returns home, Esc cancels. A scrolling indeterminate loading bar runs while directory I/O is pending and stops on completion or failure. Search remains responsive while loading. `e` edits a favorite's percentage, Enter saves, Esc cancels. Favorites and incomplete weights can be saved. Total allocation must not exceed 100%; analysis requires 100% and sufficient price history. Each market binds its currency, with no FX configuration.

Analysis shows normalized model portfolio value, total return and maximum drawdown. `d/w/m/c` selects daily/weekly/monthly/quarterly aggregation. Left/Right pans by 30 calendar days; Home returns to the latest window. `s` edits the model start date; `b` cycles never/monthly/quarterly/yearly rebalance. Model returns exclude fees, taxes and FX conversion.

## Configuration and persistence

The on-disk schema has a top-level `market` and one `[portfolio]` table with `[[portfolio.positions]]`. The TUI generates the file atomically, validates single-market positions, duplicates, dates and allocation bounds, and preserves configuration if saving fails. Provider selection is built in; no provider-selection wizard is required.

Default configuration and cache are in the platform configuration directory under `portifolio/config.toml` and `portifolio/cache.sqlite`. `--config` and `--db` override paths independently. Separate sessions may use separate configuration files.

The older multiple-portfolio schema is rejected without being overwritten. Automatic migration is not implemented; create a new configuration path to initialize the simplified workflow.

## Data loading

Bubble Tea commands perform I/O outside the update loop. Cached asset directories display first; after 24 hours a background refresh fetches a complete replacement snapshot. Failed or partial downloads preserve the previous snapshot. `r` refreshes manually. Historical prices load only for the selected asset and portfolio members, avoiding a market-wide request fanout. Cache history is reused; missing histories are requested through the market's provider chain and written to SQLite. Rows display unavailable prices until results arrive. Failed providers do not produce synthetic data. Free endpoint availability and historical adjustment semantics remain provider-specific limitations.

## Verification

Tests exercise market-only initialization, empty configuration round trips, favorite persistence, weight editing/correction, fuzzy search and allocation validation. Domain calculation, provider fallback and SQLite round-trip tests remain in place.
