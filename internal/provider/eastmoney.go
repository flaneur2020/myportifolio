package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"portifolio/internal/config"
	"portifolio/internal/domain"
)

type Eastmoney struct{ client *http.Client }

func NewEastmoney() *Eastmoney         { return &Eastmoney{client: HTTPClient(20 * time.Second)} }
func (e *Eastmoney) Name() string      { return "eastmoney" }
func (e *Eastmoney) Markets() []string { return []string{config.MarketCN} }

func (e *Eastmoney) History(ctx context.Context, p config.Position, start, end time.Time) ([]domain.Bar, error) {
	if p.Market != config.MarketCN {
		return nil, fmt.Errorf("eastmoney supports China market only")
	}
	code := strings.TrimSpace(p.Symbol)
	if code == "" {
		return nil, fmt.Errorf("empty symbol")
	}
	exchange := "0"
	if strings.HasPrefix(code, "5") || strings.HasPrefix(code, "6") || strings.HasPrefix(code, "9") {
		exchange = "1"
	}
	q := url.Values{}
	q.Set("secid", exchange+"."+code)
	q.Set("fields1", "f1,f2,f3,f4,f5,f6")
	q.Set("fields2", "f51,f52,f53,f54,f55,f56,f57,f58,f59,f60,f61")
	q.Set("klt", "101")
	q.Set("fqt", "1")
	q.Set("beg", start.Format("20060102"))
	q.Set("end", end.Format("20060102"))
	q.Set("lmt", "10000")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://push2his.eastmoney.com/api/qt/stock/kline/get?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 portifolio/0.1")
	req.Header.Set("Referer", "https://quote.eastmoney.com/")
	resp, err := e.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("eastmoney returned HTTP %s", resp.Status)
	}
	var body struct {
		Data *struct {
			Code   string   `json:"code"`
			Klines []string `json:"klines"`
		} `json:"data"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	if body.Data == nil || len(body.Data.Klines) == 0 {
		return nil, fmt.Errorf("eastmoney returned no daily prices for %s", code)
	}
	out := make([]domain.Bar, 0, len(body.Data.Klines))
	for _, line := range body.Data.Klines {
		parts := strings.Split(line, ",")
		if len(parts) < 6 {
			continue
		}
		d, err := time.Parse("2006-01-02", parts[0])
		if err != nil {
			continue
		}
		var open, closeVal, high, low float64
		if _, err = fmt.Sscan(parts[1], &open); err != nil {
			continue
		}
		fmt.Sscan(parts[2], &closeVal)
		fmt.Sscan(parts[3], &high)
		fmt.Sscan(parts[4], &low)
		out = append(out, domain.Bar{Date: d, Open: open, Close: closeVal, High: high, Low: low, AdjustedClose: closeVal, Currency: "CNY", Source: e.Name()})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("eastmoney returned no valid prices")
	}
	return out, nil
}
