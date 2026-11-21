-- Built-in China A-shares market plugin: market definition, Eastmoney price
-- provider, and Eastmoney paginated asset directory.
local function trim(s)
  if type(s) ~= "string" then return "" end
  return (s:gsub("^%s+", ""):gsub("%s+$", ""))
end

local function split_comma(line)
  local parts = {}
  local idx = 1
  for part in string.gmatch(line, "([^,]+)") do
    parts[idx] = part
    idx = idx + 1
  end
  return parts
end

return {
  api_version = 1,
  capabilities = { "http" },
  allowed_hosts = {
    "push2his.eastmoney.com",
    "push2.eastmoney.com",
  },

  markets = {
    {
      id = "cn",
      name = "China A-shares",
      description = "China A-shares · CNY · Eastmoney",
      currency = "CNY",
      asset_kinds = { "stock", "etf" },
      price_providers = { "eastmoney" },
      directory_provider = "eastmoney_cn",
      common_symbols = {
        "510300", "510500", "510050", "159915", "588000", "159919",
        "512100", "518880", "600519", "600036", "601318", "000858", "300750",
      },
    },
  },

  providers = {
    {
      name = "eastmoney",
      markets = { "cn" },
      history = function(req)
        if req.market ~= "cn" then
          return nil, "eastmoney supports China market only"
        end
        local code = trim(req.symbol)
        if code == "" then
          return nil, "empty symbol"
        end
        local first = string.sub(code, 1, 1)
        local exchange = "0"
        if first == "5" or first == "6" or first == "9" then
          exchange = "1"
        end
        local url = "https://push2his.eastmoney.com/api/qt/stock/kline/get?"
          .. "secid=" .. host.url.query_escape(exchange .. "." .. code)
          .. "&fields1=f1,f2,f3,f4,f5,f6"
          .. "&fields2=f51,f52,f53,f54,f55,f56,f57,f58,f59,f60,f61"
          .. "&klt=101&fqt=1"
          .. "&beg=" .. host.time.compact(req.start)
          .. "&end=" .. host.time.compact(req["end"])
          .. "&lmt=10000"
        local resp, err = host.http.get(url, {
          headers = {
            ["User-Agent"] = "Mozilla/5.0 portifolio/0.1",
            ["Referer"] = "https://quote.eastmoney.com/",
          },
        })
        if err then
          return nil, err
        end
        if resp.status ~= 200 then
          return nil, "eastmoney returned HTTP " .. resp.status_text
        end
        local body, jerr = host.json.decode(resp.body)
        if jerr or type(body) ~= "table" then
          return nil, "eastmoney: invalid JSON"
        end
        local klines = body.data and body.data.klines
        if type(klines) ~= "table" or #klines == 0 then
          return nil, "eastmoney returned no daily prices for " .. code
        end
        local out = {}
        for i = 1, #klines do
          local parts = split_comma(klines[i])
          if #parts >= 5 then
            local o = tonumber(parts[2])
            local c = tonumber(parts[3])
            local h = tonumber(parts[4]) or 0
            local l = tonumber(parts[5]) or 0
            if o and c then
              out[#out + 1] = {
                date = parts[1],
                open = o,
                close = c,
                high = h,
                low = l,
                adjusted_close = c,
                currency = "CNY",
                provider = "eastmoney",
              }
            end
          end
        end
        if #out == 0 then
          return nil, "eastmoney returned no valid prices"
        end
        return out, nil
      end,
    },
  },

  directories = {
    {
      name = "eastmoney_cn",
      markets = { "cn" },
      assets = function(req)
        local out = {}
        local seen = {}
        local groups = {
          { filter = "m:0+t:6,m:0+t:80,m:1+t:2,m:1+t:23,m:0+t:81+s:2048", kind = "stock" },
          { filter = "b:MK0021,b:MK0022,b:MK0023,b:MK0024", kind = "etf" },
        }
        for _, g in ipairs(groups) do
          for page = 1, 500 do
            local url = "https://push2.eastmoney.com/api/qt/clist/get?"
              .. "pn=" .. tostring(page)
              .. "&pz=100&po=0&np=1&fid=f12"
              .. "&fs=" .. host.url.query_escape(g.filter)
              .. "&fields=f12,f14&fltt=2"
            local resp, err = host.http.get(url, {
              headers = { ["User-Agent"] = "Mozilla/5.0 portifolio/0.1" },
              max_bytes = 5 * 1024 * 1024,
            })
            if err then
              return nil, err
            end
            if resp.status ~= 200 then
              return nil, "asset directory: HTTP " .. resp.status_text
            end
            local body, jerr = host.json.decode(resp.body)
            if jerr or type(body) ~= "table" then
              return nil, "eastmoney directory: invalid JSON"
            end
            local data = body.data
            if type(data) ~= "table" or (data.total or 0) == 0 then
              return nil, "empty China " .. g.kind .. " directory"
            end
            local diff = data.diff
            if type(diff) ~= "table" or #diff == 0 then
              return nil, "incomplete China directory at page " .. tostring(page)
            end
            local added = 0
            for i = 1, #diff do
              local item = diff[i]
              local sym = item.f12 or ""
              local name = item.f14 or ""
              if sym ~= "" and not seen[sym] then
                seen[sym] = true
                out[#out + 1] = {
                  symbol = sym,
                  name = name,
                  market = "cn",
                  kind = g.kind,
                }
                added = added + 1
              end
            end
            if page * 100 >= data.total then
              break
            end
            if added == 0 or page == 500 then
              return nil, "directory pagination did not complete"
            end
          end
        end
        return out, nil
      end,
    },
  },
}
