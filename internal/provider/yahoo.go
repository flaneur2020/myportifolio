package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"portifolio/internal/config"
)

type Yahoo struct{ client *http.Client }

func NewYahoo() *Yahoo             { return &Yahoo{client: HTTPClient(15 * time.Second)} }
func (y *Yahoo) Name() string      { return "yahoo" }
func (y *Yahoo) Markets() []string { return []string{"us"} }

func (y *Yahoo) History(ctx context.Context, p config.Position, start, end time.Time) ([]Bar, error) {
	if p.Market != "us" {
		return nil, fmt.Errorf("yahoo supports US market only")
	}
	u := fmt.Sprintf("https://query1.finance.yahoo.com/v8/finance/chart/%s?period1=%d&period2=%d&interval=1d&events=div%%2Csplits", url.PathEscape(p.Symbol), start.Unix(), end.AddDate(0, 0, 1).Unix())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 portifolio/0.1 personal-research")
	resp, err := y.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("yahoo returned HTTP %s", resp.Status)
	}
	var payload struct {
		Chart struct {
			Error *struct {
				Code        string `json:"code"`
				Description string `json:"description"`
			} `json:"error"`
			Result []struct {
				Meta struct {
					Currency string `json:"currency"`
				} `json:"meta"`
				Timestamp  []int64 `json:"timestamp"`
				Indicators struct {
					Quote []struct {
						Open  []*float64 `json:"open"`
						High  []*float64 `json:"high"`
						Low   []*float64 `json:"low"`
						Close []*float64 `json:"close"`
					} `json:"quote"`
					AdjClose []struct {
						AdjClose []*float64 `json:"adjclose"`
					} `json:"adjclose"`
				} `json:"indicators"`
			} `json:"result"`
		} `json:"chart"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	if payload.Chart.Error != nil {
		return nil, fmt.Errorf("yahoo: %s", payload.Chart.Error.Description)
	}
	if len(payload.Chart.Result) == 0 {
		return nil, fmt.Errorf("yahoo returned no result")
	}
	r := payload.Chart.Result[0]
	if len(r.Indicators.Quote) == 0 {
		return nil, fmt.Errorf("yahoo returned no quotes")
	}
	q := r.Indicators.Quote[0]
	out := make([]Bar, 0, len(r.Timestamp))
	for i, ts := range r.Timestamp {
		if i >= len(q.Close) || q.Close[i] == nil {
			continue
		}
		close := *q.Close[i]
		adj := close
		if len(r.Indicators.AdjClose) > 0 && i < len(r.Indicators.AdjClose[0].AdjClose) && r.Indicators.AdjClose[0].AdjClose[i] != nil {
			adj = *r.Indicators.AdjClose[0].AdjClose[i]
		}
		b := Bar{Date: time.Unix(ts, 0).UTC(), Close: close, AdjustedClose: adj, Currency: r.Meta.Currency, Source: y.Name()}
		if i < len(q.Open) {
			b.Open = floatValue(q.Open[i])
		}
		if i < len(q.High) {
			b.High = floatValue(q.High[i])
		}
		if i < len(q.Low) {
			b.Low = floatValue(q.Low[i])
		}
		out = append(out, b)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("yahoo returned no valid prices")
	}
	return out, nil
}
func floatValue(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}
