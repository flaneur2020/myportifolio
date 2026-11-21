package provider

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"portifolio/internal/config"
	"portifolio/internal/domain"
)

// AssetLister is independent of historical-price providers: a directory source
// need not implement prices, and price providers need not expose a directory.
type AssetLister interface {
	Assets(context.Context, string) ([]domain.Asset, error)
}

type Directory struct {
	Client       *http.Client
	USURL, CNURL string
}

func NewDirectory() *Directory {
	return &Directory{
		Client: HTTPClient(20 * time.Second),
		USURL:  "https://www.nasdaqtrader.com/dynamic/SymDir/nasdaqtraded.txt",
		CNURL:  "https://push2.eastmoney.com/api/qt/clist/get",
	}
}

func (d *Directory) get(ctx context.Context, address string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 portifolio/0.1")
	resp, err := d.Client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("asset directory: HTTP %s", resp.Status)
	}
	return resp, nil
}

func (d *Directory) Assets(ctx context.Context, market string) ([]domain.Asset, error) {
	switch market {
	case config.MarketUS:
		return d.us(ctx)
	case config.MarketCN:
		return d.cn(ctx)
	default:
		return nil, fmt.Errorf("unsupported market %q", market)
	}
}

func (d *Directory) us(ctx context.Context) ([]domain.Asset, error) {
	resp, err := d.get(ctx, d.USURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	r := csv.NewReader(io.LimitReader(resp.Body, 20<<20))
	r.Comma = '|'
	r.FieldsPerRecord = -1
	header, err := r.Read()
	if err != nil {
		return nil, err
	}
	columns := map[string]int{}
	for i, s := range header {
		columns[s] = i
	}
	for _, key := range []string{"Symbol", "Security Name", "ETF", "Test Issue"} {
		if _, ok := columns[key]; !ok {
			return nil, fmt.Errorf("directory missing %s column", key)
		}
	}
	var out []domain.Asset
	for {
		row, e := r.Read()
		if e == io.EOF {
			break
		}
		if e != nil {
			return nil, e
		}
		get := func(key string) string {
			i := columns[key]
			if i >= len(row) {
				return ""
			}
			return strings.TrimSpace(row[i])
		}
		symbol := get("Symbol")
		if symbol == "" || strings.HasPrefix(row[0], "File Creation Time") || get("Test Issue") != "N" {
			continue
		}
		kind := config.KindStock
		if get("ETF") == "Y" {
			kind = config.KindETF
		}
		out = append(out, domain.Asset{Symbol: symbol, Name: get("Security Name"), Market: config.MarketUS, Kind: kind})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("empty US directory")
	}
	return out, nil
}

func (d *Directory) cn(ctx context.Context) ([]domain.Asset, error) {
	var out []domain.Asset
	seen := map[string]bool{}
	groups := []struct{ filter, kind string }{
		{"m:0+t:6,m:0+t:80,m:1+t:2,m:1+t:23,m:0+t:81+s:2048", config.KindStock},
		{"b:MK0021,b:MK0022,b:MK0023,b:MK0024", config.KindETF},
	}
	for _, g := range groups {
		for page := 1; page <= 500; page++ {
			u, err := url.Parse(d.CNURL)
			if err != nil {
				return nil, err
			}
			q := u.Query()
			q.Set("pn", strconv.Itoa(page))
			q.Set("pz", "100")
			q.Set("po", "0")
			q.Set("np", "1")
			q.Set("fid", "f12")
			q.Set("fs", g.filter)
			q.Set("fields", "f12,f14")
			q.Set("fltt", "2")
			u.RawQuery = q.Encode()
			resp, err := d.get(ctx, u.String())
			if err != nil {
				return nil, err
			}
			var body struct {
				Data *struct {
					Total int `json:"total"`
					Diff  []struct {
						Symbol string `json:"f12"`
						Name   string `json:"f14"`
					} `json:"diff"`
				} `json:"data"`
			}
			err = json.NewDecoder(io.LimitReader(resp.Body, 5<<20)).Decode(&body)
			resp.Body.Close()
			if err != nil {
				return nil, err
			}
			if body.Data == nil || body.Data.Total == 0 {
				return nil, fmt.Errorf("empty China %s directory", g.kind)
			}
			if len(body.Data.Diff) == 0 {
				return nil, fmt.Errorf("incomplete China directory at page %d", page)
			}
			added := 0
			for _, a := range body.Data.Diff {
				if a.Symbol != "" && !seen[a.Symbol] {
					out = append(out, domain.Asset{Symbol: a.Symbol, Name: a.Name, Market: config.MarketCN, Kind: g.kind})
					seen[a.Symbol] = true
					added++
				}
			}
			if page*100 >= body.Data.Total {
				break
			}
			if added == 0 || page == 500 {
				return nil, fmt.Errorf("directory pagination did not complete")
			}
		}
	}
	return out, nil
}
