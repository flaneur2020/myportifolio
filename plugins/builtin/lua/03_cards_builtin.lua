-- Built-in Analysis cards implemented purely as Lua plugins returning
-- declarative core.Block trees.
local function sign_tone(v)
  if v > 0 then return "gain" end
  if v < 0 then return "loss" end
  return "neutral"
end

local function fmt_pct(v)
  return string.format("%+.2f%%", v)
end

local function fmt_pp(v)
  return string.format("%+.1fpp", v)
end

local function fmt_amt(v, currency)
  return string.format("%+.2f %s", v, currency)
end

local function positive_positions(positions)
  local out = {}
  for i = 1, #positions do
    if (positions[i].allocation or 0) > 0 then
      out[#out + 1] = positions[i]
    end
  end
  return out
end

return {
  api_version = 1,
  cards = {
    {
      id = "information",
      title = "PORTFOLIO INFORMATION",
      render = function(ctx)
        local p = ctx.portfolio
        local curve = ctx.curve
        local latest = curve[#curve]
        local capital = ctx.capital
        if not capital or capital <= 0 then capital = 10000 end
        local pct, amt, ok = host.recent_change(curve, capital)
        local recent_row
        if ok then
          recent_row = {
            label = "Recent day",
            spans = {
              { text = latest.date .. "  " },
              { text = fmt_pct(pct), tone = sign_tone(pct) },
              { text = "  " },
              { text = fmt_amt(amt, p.base_currency), tone = sign_tone(amt) },
            },
          }
        else
          recent_row = { label = "Recent day", value = "—" }
        end

        local total_ret = latest.value - 100
        local max_dd = (ctx.max_drawdown or 0) * 100
        local rows = {
          { label = "Model index", value = string.format("%.2f", latest.value) },
          { label = "Total return", value = fmt_pct(total_ret), tone = sign_tone(total_ret) },
          { label = "Max drawdown", value = fmt_pct(max_dd), tone = sign_tone(max_dd) },
          recent_row,
          { separator = true },
          { label = "Requested start", value = p.start_date },
          { label = "Effective start", value = curve[1].date },
          { label = "Latest data", value = latest.date },
          { label = "Rebalance", value = p.rebalance },
          { label = "Initial capital", value = string.format("%.2f %s", capital, p.base_currency) },
          { label = "Model value", value = string.format("%.2f %s", capital * latest.value / 100, p.base_currency) },
        }

        local blocks = {
          { type = "text", text = p.name },
          { type = "metrics", label_width = 17, rows = rows },
          {
            type = "text",
            tone = "muted",
            text = string.format("Configured weight total %.1f%% · normalized to 100%% for analysis", (ctx.configured_weight or 0) * 100),
          },
        }
        if ctx.effective_start_note and ctx.effective_start_note ~= "" then
          blocks[#blocks + 1] = { type = "text", tone = "muted", text = ctx.effective_start_note }
        end
        return blocks
      end,
    },

    {
      id = "history",
      title = "PORTFOLIO HISTORY",
      supports_interval = true,
      supports_pan = true,
      render = function(ctx)
        local curve = ctx.curve
        local all = host.aggregate_candles(host.portfolio_candles(curve), ctx.grain)
        local chart_w = math.max(20, math.min(68, ctx.width - 16))
        local capacity = math.max(1, math.floor((chart_w - 12) / 2))
        local visible = host.candle_window(all, capacity, ctx.offset)
        if #visible == 0 then
          return { { type = "text", text = "No portfolio history available" } }
        end
        local coverage_start = visible[1].date
        local period_start = host.period_start(coverage_start, ctx.grain)
        for i = 1, #curve do
          local d = curve[i].date
          if d >= period_start and d <= visible[1].date then
            coverage_start = d
            break
          end
        end
        return {
          title_suffix = " · " .. ctx.grain,
          blocks = {
            { type = "spacer" },
            { type = "candles", bars = visible, grain = "day", height = 12 },
            { type = "text", tone = "muted", text = string.format("Window: %s → %s", coverage_start, visible[#visible].date) },
            { type = "text", tone = "muted", text = "←/→ move one candle · z day/week/month/quarter/year" },
            { type = "text", tone = "muted", text = "Model candles use daily closes; wicks are not intraday highs/lows." },
          },
        }
      end,
    },

    {
      id = "drawdown",
      title = "DRAWDOWN / HISTORY",
      supports_interval = true,
      supports_pan = true,
      render = function(ctx)
        local plot_w = math.max(8, math.min(54, ctx.width - 28))
        local all = host.aggregate_drawdown(host.drawdown(ctx.curve), ctx.grain)
        local visible = host.point_window(all, math.max(1, math.floor(plot_w / 2)), ctx.offset)
        return {
          title_suffix = " · " .. string.upper(ctx.grain),
          blocks = {
            { type = "text", tone = "muted", text = "Worst daily decline from prior peak in each period (%)" },
            { type = "spacer" },
            { type = "drawdown", points = visible, height = 7 },
            { type = "text", tone = "muted", text = "z interval · ←/→ move one period · 0% = previous peak" },
          },
        }
      end,
    },

    {
      id = "allocation",
      title = "TARGET ALLOCATION",
      render = function(ctx)
        local positions = positive_positions(ctx.positions)
        local rows = {}
        for i = 1, #positions do
          rows[#rows + 1] = {
            label = positions[i].symbol,
            value = positions[i].allocation * 100,
            suffix = "%",
            precision = 1,
            hide_sign = true,
          }
        end
        return {
          { type = "bars", scale = 100, bar_width_cap = 48, label_width = 10, rows = rows },
          { type = "text", tone = "muted", text = "Normalized target weights; not drifting market weights." },
        }
      end,
    },

    {
      id = "returns",
      title = "PERIOD RETURNS",
      supports_interval = true,
      supports_pan = true,
      render = function(ctx)
        local p = ctx.portfolio
        local capital = ctx.capital
        if not capital or capital <= 0 then capital = 10000 end
        local all_returns = host.returns(ctx.curve, ctx.grain)
        local gains = host.gains(ctx.curve, ctx.grain, capital)
        local gain_by_date = {}
        for i = 1, #gains do
          gain_by_date[gains[i].date] = gains[i].value
        end
        local plot_w = math.max(8, math.min(54, ctx.width - 28))
        local visible = host.point_window(all_returns, math.max(1, math.floor(plot_w / 2)), ctx.offset)

        local blocks = {}
        if #visible > 0 then
          local latest = visible[#visible]
          local latest_gain = gain_by_date[latest.date] or 0
          blocks[#blocks + 1] = {
            type = "spans",
            spans = {
              { text = "Latest  " .. latest.date .. "  " },
              { text = fmt_pct(latest.value), tone = sign_tone(latest.value) },
              { text = "  " },
              { text = fmt_amt(latest_gain, p.base_currency), tone = sign_tone(latest_gain) },
            },
          }
          blocks[#blocks + 1] = { type = "spacer" }
        end
        blocks[#blocks + 1] = {
          type = "return_bars",
          points = visible,
          height = 7,
          capital = capital,
          currency = p.base_currency,
        }
        blocks[#blocks + 1] = {
          type = "spans",
          spans = {
            { text = "━━/░░ Gain above 0%", tone = "gain" },
            { text = "   " },
            { text = "━━/░░ Loss below 0%", tone = "loss" },
          },
        }
        blocks[#blocks + 1] = {
          type = "text",
          tone = "muted",
          text = "Y axis: " .. p.base_currency .. " model gain/loss · bar heights are percentage returns",
        }
        if #visible > 0 then
          local best, worst = visible[1], visible[1]
          for i = 2, #visible do
            if visible[i].value > best.value then best = visible[i] end
            if visible[i].value < worst.value then worst = visible[i] end
          end
          blocks[#blocks + 1] = {
            type = "spans",
            spans = {
              { text = "Best " },
              { text = fmt_pct(best.value), tone = sign_tone(best.value) },
              { text = "   Worst " },
              { text = fmt_pct(worst.value), tone = sign_tone(worst.value) },
            },
          }
        end
        blocks[#blocks + 1] = {
          type = "text",
          tone = "muted",
          text = "z interval · ←/→ previous/next periods · first period may be partial",
        }
        return {
          title_suffix = " · " .. string.upper(ctx.grain),
          blocks = blocks,
        }
      end,
    },

    {
      id = "drift",
      title = "WEIGHT DRIFT",
      supports_interval = true,
      supports_pan = true,
      render = function(ctx)
        local snaps = ctx.snapshots
        local targets = ctx.targets
        local positions = positive_positions(ctx.positions)
        if #snaps == 0 or #positions ~= #targets then
          return {
            title_suffix = " · " .. string.upper(ctx.grain),
            blocks = { { type = "text", tone = "muted", text = "No weight history available." } },
          }
        end
        local latest = snaps[#snaps]
        if #latest.weights ~= #targets then
          return {
            title_suffix = " · " .. string.upper(ctx.grain),
            blocks = { { type = "text", tone = "muted", text = "No weight history available." } },
          }
        end

        local drift_rows = {}
        local max_row_drift = 0
        for i = 1, #positions do
          local target = targets[i] * 100
          local actual = latest.weights[i] * 100
          local d = actual - target
          if math.abs(d) > max_row_drift then max_row_drift = math.abs(d) end
          drift_rows[#drift_rows + 1] = {
            index = i,
            symbol = positions[i].symbol,
            target = target,
            actual = actual,
            drift = d,
          }
        end
        table.sort(drift_rows, function(a, b) return math.abs(a.drift) > math.abs(b.drift) end)

        local bar_rows = {}
        for i = 1, #drift_rows do
          local row = drift_rows[i]
          bar_rows[#bar_rows + 1] = {
            swatch = row.index - 1,
            label = row.symbol,
            prefix = string.format("%5.1f%% → %5.1f%% ", row.target, row.actual),
            value = row.drift,
            extra = { { text = fmt_pp(row.drift), tone = sign_tone(row.drift) } },
          }
        end

        local blocks = {
          { type = "text", tone = "muted", text = "LATEST ALLOCATION" },
          {
            type = "stacked_bar",
            rows = {
              { label = "TARGET", weights = targets },
              { label = "ACTUAL", weights = latest.weights },
            },
          },
          { type = "spacer" },
          { type = "text", tone = "muted", text = "DRIFT BY ASSET · largest first" },
          { type = "bars", scale = max_row_drift, min_one = true, bar_width_cap = 14, label_width = 7, rows = bar_rows },
        }

        local hist_cap = math.max(8, math.min(24, math.floor(ctx.width / 3)))
        local history = host.point_window(host.max_drift(snaps, targets, ctx.grain), hist_cap, ctx.offset)
        if #history > 0 then
          local peak = history[#history]
          for i = 1, #history do
            if history[i].value > peak.value then peak = history[i] end
          end
          blocks[#blocks + 1] = { type = "spacer" }
          blocks[#blocks + 1] = {
            type = "spans",
            spans = {
              { text = "Max drift  " },
              { text = fmt_pp(peak.value), tone = sign_tone(peak.value) },
              { text = " · peak " .. peak.date },
            },
          }
          blocks[#blocks + 1] = { type = "text", tone = "muted", text = "MAX ABSOLUTE DRIFT OVER TIME" }
          blocks[#blocks + 1] = { type = "drift_history", points = history, height = 4 }
        end
        blocks[#blocks + 1] = {
          type = "text",
          tone = "muted",
          text = "z interval · ←/→ drift history · target weights normalized to 100%",
        }
        return {
          title_suffix = " · " .. string.upper(ctx.grain),
          blocks = blocks,
        }
      end,
    },

    {
      id = "contribution",
      title = "ASSET CONTRIBUTION",
      supports_interval = true,
      supports_pan = true,
      render = function(ctx)
        local p = ctx.portfolio
        local snaps = ctx.snapshots
        local targets = ctx.targets
        local periods = host.contributions(p.rebalance, snaps, targets, ctx.grain)
        if #periods == 0 then
          return {
            title_suffix = " · " .. string.upper(ctx.grain),
            blocks = { { type = "text", tone = "muted", text = "No contribution history available." } },
          }
        end
        local idx = math.max(1, math.min(#periods - ctx.offset, #periods))
        local period = periods[idx]
        local positions = positive_positions(ctx.positions)
        local capital = ctx.capital
        if not capital or capital <= 0 then capital = 10000 end
        local scale = 0
        for i = 1, #period.assets do
          if math.abs(period.assets[i]) > scale then scale = math.abs(period.assets[i]) end
        end
        local rows = {}
        for i = 1, math.min(#positions, #period.assets) do
          local v = period.assets[i]
          local amt = capital * v / 100
          rows[#rows + 1] = {
            label = positions[i].symbol,
            value = v,
            extra = {
              { text = fmt_pct(v), tone = sign_tone(v) },
              { text = "  " },
              { text = fmt_amt(amt, p.base_currency), tone = sign_tone(amt) },
            },
          }
        end
        return {
          title_suffix = " · " .. string.upper(ctx.grain),
          blocks = {
            { type = "text", tone = "muted", text = "Period ending " .. period.date },
            { type = "bars", scale = scale, bar_width_cap = 18, label_width = 7, rows = rows },
            { type = "spacer" },
            {
              type = "spans",
              spans = {
                { text = "Total      " },
                { text = fmt_pct(period.total), tone = sign_tone(period.total) },
                { text = "  " },
                { text = fmt_amt(capital * period.total / 100, p.base_currency), tone = sign_tone(period.total) },
              },
            },
            {
              type = "text",
              tone = "muted",
              text = "Contribution to model return · rebalance days use target weights\nz interval · ←/→ previous periods",
            },
          },
        }
      end,
    },
  },
}
