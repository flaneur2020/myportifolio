package builtin

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"portifolio/core"
	luart "portifolio/runtime/lua"
)

func TestBuiltinEmbeddedPluginsRegisterCleanly(t *testing.T) {
	k := core.NewKernel()
	if errs := Register(k, nil); len(errs) != 0 {
		t.Fatalf("builtin plugin registration errors: %v", errs)
	}
	markets := k.Markets()
	if len(markets) != 2 || markets[0].ID != "us" || markets[1].ID != "cn" {
		t.Fatalf("unexpected builtin markets: %#v", markets)
	}
	if len(k.Cards()) != 7 {
		t.Fatalf("expected 7 builtin cards, got %d", len(k.Cards()))
	}
	if theme := k.ActiveTheme(""); theme.ID != "default" || theme.Accent == "" {
		t.Fatalf("default theme missing: %#v", theme)
	}
}

func TestBuiltinUSDirectoryAndYahooStooqProviders(t *testing.T) {
	nasdaqSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "Symbol|Security Name|ETF|Test Issue\nVTI|Vanguard Total|Y|N\nAAPL|Apple|N|N\nTEST|Test|N|Y\nFile Creation Time: now\n")
	}))
	defer nasdaqSrv.Close()

	yahooSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer yahooSrv.Close()

	stooqSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "Date,Open,High,Low,Close,Volume\n2024-01-02,100,105,99,102,1000\n")
	}))
	defer stooqSrv.Close()

	k := core.NewKernel()
	host := luart.NewHost(luart.HostOptions{
		HTTPClient: nasdaqSrv.Client(),
		URLRewrites: map[string]string{
			"https://www.nasdaqtrader.com/dynamic/SymDir/nasdaqtraded.txt": nasdaqSrv.URL,
			"https://query1.finance.yahoo.com":                             yahooSrv.URL,
			"https://stooq.com":                                            stooqSrv.URL,
		},
	})
	if errs := Register(k, host); len(errs) != 0 {
		t.Fatalf("register failed: %v", errs)
	}

	assets, err := k.FetchDirectory(context.Background(), "us")
	if err != nil {
		t.Fatal(err)
	}
	if len(assets) != 2 || assets[0].Kind != "etf" || assets[1].Symbol != "AAPL" {
		t.Fatalf("unexpected US assets: %#v", assets)
	}

	// Yahoo fails with 429 -> automatic fallback to Stooq!
	bars, err := k.FetchHistory(context.Background(), core.Position{Market: "us", Symbol: "VTI"}, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2024, 1, 5, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(bars) != 1 || bars[0].Source != "stooq" || bars[0].Close != 102 {
		t.Fatalf("unexpected fallback bars: %#v", bars)
	}
}

func TestBuiltinCNDirectoryPaginationAndEastmoneyPrices(t *testing.T) {
	dirCalls := 0
	dirSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dirCalls++
		q := r.URL.Query()
		if q.Get("fs") == "b:MK0021,b:MK0022,b:MK0023,b:MK0024" {
			fmt.Fprint(w, `{"data":{"total":1,"diff":[{"f12":"510300","f14":"ETF"}]}}`)
			return
		}
		if q.Get("pn") == "1" {
			fmt.Fprint(w, `{"data":{"total":101,"diff":[{"f12":"600519","f14":"Moutai"}]}}`)
		} else {
			fmt.Fprint(w, `{"data":{"total":101,"diff":[{"f12":"000001","f14":"Bank"}]}}`)
		}
	}))
	defer dirSrv.Close()

	priceSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":{"code":"510300","klines":["2024-01-02,3.50,3.60,3.65,3.48,1000"]}}`)
	}))
	defer priceSrv.Close()

	k := core.NewKernel()
	host := luart.NewHost(luart.HostOptions{
		HTTPClient: dirSrv.Client(),
		URLRewrites: map[string]string{
			"https://push2.eastmoney.com/api/qt/clist/get":          dirSrv.URL,
			"https://push2his.eastmoney.com/api/qt/stock/kline/get": priceSrv.URL,
		},
	})
	if errs := Register(k, host); len(errs) != 0 {
		t.Fatalf("register failed: %v", errs)
	}

	assets, err := k.FetchDirectory(context.Background(), "cn")
	if err != nil {
		t.Fatal(err)
	}
	if dirCalls != 3 || len(assets) != 3 || assets[2].Kind != "etf" {
		t.Fatalf("calls=%d assets=%#v", dirCalls, assets)
	}

	bars, err := k.FetchHistory(context.Background(), core.Position{Market: "cn", Symbol: "510300"}, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2024, 1, 5, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(bars) != 1 || bars[0].Close != 3.60 || bars[0].Currency != "CNY" || bars[0].Source != "eastmoney" {
		t.Fatalf("unexpected CN bars: %#v", bars)
	}
}
