-- Copy to <config-dir>/plugins/ to enable.
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

        local latest = ctx.curve[#ctx.curve]
        return {
          { type = "metrics", rows = {
            { "Latest index", string.format("%.2f", latest.value) },
            { "Worst drawdown", string.format("%.2f%%", worst) },
            { "Observations", tostring(#ctx.curve) },
          }},
          { type = "line", title = "Drawdown (%)", points = dd, height = 6 },
        }
      end,
    },
  },
}
