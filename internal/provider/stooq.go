package provider

import (
	"context"
	"encoding/csv"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"portifolio/internal/config"
	"portifolio/internal/domain"
)

type Stooq struct{ client *http.Client }

func NewStooq() *Stooq             { return &Stooq{client: HTTPClient(15 * time.Second)} }
func (s *Stooq) Name() string      { return "stooq" }
func (s *Stooq) Markets() []string { return []string{config.MarketUS} }

func (s *Stooq) History(ctx context.Context, p config.Position, start, end time.Time) ([]domain.Bar, error) {
	if p.Market != config.MarketUS {
		return nil, fmt.Errorf("stooq supports US market only")
	}
	symbol := strings.ToLower(p.Symbol)
	u := "https://stooq.com/q/d/l/?s=" + url.QueryEscape(symbol) + "&d1=" + start.Format("20060102") + "&d2=" + end.Format("20060102") + "&i=d"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("stooq returned HTTP %s", resp.Status)
	}
	rows, err := csv.NewReader(resp.Body).ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) < 2 {
		return nil, fmt.Errorf("stooq returned no prices")
	}
	var out []domain.Bar
	for _, r := range rows[1:] {
		if len(r) < 5 {
			continue
		}
		d, e := time.Parse("2006-01-02", r[0])
		if e != nil {
			continue
		}
		var o, h, l, c float64
		if _, e = fmt.Sscan(r[1], &o); e != nil {
			continue
		}
		fmt.Sscan(r[2], &h)
		fmt.Sscan(r[3], &l)
		fmt.Sscan(r[4], &c)
		out = append(out, domain.Bar{Date: d, Open: o, High: h, Low: l, Close: c, AdjustedClose: c, Currency: "USD", Source: s.Name()})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("stooq returned no valid prices")
	}
	return out, nil
}
