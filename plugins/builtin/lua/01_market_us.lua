-- Built-in US market plugin: market definition, Yahoo/Stooq price providers,
-- and Nasdaq Trader asset directory.
local function trim(s)
  if type(s) ~= "string" then return "" end
  return (s:gsub("^%s+", ""):gsub("%s+$", ""))
end

return {
  api_version = 1,
  capabilities = { "http" },
  allowed_hosts = {
    "query1.finance.yahoo.com",
    "stooq.com",
    "www.nasdaqtrader.com",
  },

  markets = {
    {
      id = "us",
      name = "United States",
      description = "United States · USD · Yahoo / Stooq",
      currency = "USD",
      asset_kinds = { "stock", "etf" },
      price_providers = { "yahoo", "stooq" },
      directory_provider = "nasdaqtrader",
      common_symbols = {
        "SPY", "QQQ", "VTI", "VOO", "AAPL", "MSFT", "NVDA", "AMZN",
        "GOOGL", "META", "TSLA", "IWM", "DIA", "BND", "TLT", "GLD",
      },
    },
  },

  providers = {
    {
      name = "yahoo",
      markets = { "us" },
      history = function(req)
        if req.market ~= "us" then
          return nil, "yahoo supports US market only"
        end
        local period2 = req.end_unix + 86400
        local url = string.format(
          "https://query1.finance.yahoo.com/v8/finance/chart/%s?period1=%d&period2=%d&interval=1d&events=div%%2Csplits",
          host.url.path_escape(req.symbol),
          math.floor(req.start_unix),
          math.floor(period2)
        )
        local resp, err = host.http.get(url, {
          headers = { ["User-Agent"] = "Mozilla/5.0 portifolio/0.1 personal-research" },
        })
        if err then
          return nil, err
        end
        if resp.status ~= 200 then
          return nil, "yahoo returned HTTP " .. resp.status_text
        end
        local payload, jerr = host.json.decode(resp.body)
        if jerr or type(payload) ~= "table" or type(payload.chart) ~= "table" then
          return nil, "yahoo: invalid JSON response"
        end
        if payload.chart.error then
          return nil, "yahoo: " .. tostring(payload.chart.error.description or "error")
        end
        local results = payload.chart.result
        if type(results) ~= "table" or #results == 0 then
          return nil, "yahoo returned no result"
        end
        local r = results[1]
        local quotes = r.indicators and r.indicators.quote
        if type(quotes) ~= "table" or #quotes == 0 then
          return nil, "yahoo returned no quotes"
        end
        local q = quotes[1]
        local timestamps = r.timestamp or {}
        local adjTable = r.indicators.adjclose and r.indicators.adjclose[1] and r.indicators.adjclose[1].adjclose
        local currency = (r.meta and r.meta.currency) or "USD"
        local out = {}
        for i = 1, #timestamps do
          local closeVal = q.close and q.close[i]
          if closeVal ~= nil then
            local adj = closeVal
            if adjTable and adjTable[i] ~= nil then
              adj = adjTable[i]
            end
            out[#out + 1] = {
              date = host.time.format_unix(timestamps[i]),
              open = (q.open and q.open[i]) or 0,
              high = (q.high and q.high[i]) or 0,
              low = (q.low and q.low[i]) or 0,
              close = closeVal,
              adjusted_close = adj,
              currency = currency,
              provider = "yahoo",
            }
          end
        end
        if #out == 0 then
          return nil, "yahoo returned no valid prices"
        end
        return out, nil
      end,
    },

    {
      name = "stooq",
      markets = { "us" },
      history = function(req)
        if req.market ~= "us" then
          return nil, "stooq supports US market only"
        end
        local sym = string.lower(req.symbol)
        local url = "https://stooq.com/q/d/l/?s="
          .. host.url.query_escape(sym)
          .. "&d1=" .. host.time.compact(req.start)
          .. "&d2=" .. host.time.compact(req["end"])
          .. "&i=d"
        local resp, err = host.http.get(url)
        if err then
          return nil, err
        end
        if resp.status ~= 200 then
          return nil, "stooq returned HTTP " .. resp.status_text
        end
        local rows, cerr = host.csv.parse(resp.body)
        if cerr then
          return nil, cerr
        end
        if type(rows) ~= "table" or #rows < 2 then
          return nil, "stooq returned no prices"
        end
        local out = {}
        for i = 2, #rows do
          local r = rows[i]
          if #r >= 5 then
            local o = tonumber(r[2])
            local h = tonumber(r[3]) or 0
            local l = tonumber(r[4]) or 0
            local c = tonumber(r[5])
            if o and c then
              out[#out + 1] = {
                date = r[1],
                open = o,
                high = h,
                low = l,
                close = c,
                adjusted_close = c,
                currency = "USD",
                provider = "stooq",
              }
            end
          end
        end
        if #out == 0 then
          return nil, "stooq returned no valid prices"
        end
        return out, nil
      end,
    },
  },

  directories = {
    {
      name = "nasdaqtrader",
      markets = { "us" },
      assets = function(req)
        local url = "https://www.nasdaqtrader.com/dynamic/SymDir/nasdaqtraded.txt"
        local resp, err = host.http.get(url, {
          headers = { ["User-Agent"] = "Mozilla/5.0 portifolio/0.1" },
          max_bytes = 20 * 1024 * 1024,
        })
        if err then
          return nil, err
        end
        if resp.status ~= 200 then
          return nil, "asset directory: HTTP " .. resp.status_text
        end
        local rows, headers = host.csv.parse(resp.body, {
          delimiter = "|",
          header = true,
          columns = { "Symbol", "Security Name", "ETF", "Test Issue" },
        })
        if not rows then
          return nil, tostring(headers)
        end
        local have = {}
        for i = 1, #headers do
          have[headers[i]] = true
        end
        for _, col in ipairs({ "Symbol", "Security Name", "ETF", "Test Issue" }) do
          if not have[col] then
            return nil, "directory missing " .. col .. " column"
          end
        end
        local out = {}
        for i = 1, #rows do
          local row = rows[i]
          local first = row._first or ""
          local symbol = trim(row["Symbol"])
          if symbol ~= "" and string.sub(first, 1, 18) ~= "File Creation Time" and trim(row["Test Issue"]) == "N" then
            local kind = "stock"
            if trim(row["ETF"]) == "Y" then
              kind = "etf"
            end
            out[#out + 1] = {
              symbol = symbol,
              name = trim(row["Security Name"]),
              market = "us",
              kind = kind,
            }
          end
        end
        if #out == 0 then
          return nil, "empty US directory"
        end
        return out, nil
      end,
    },
  },
}
