package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUSDirectory(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "Symbol|Security Name|ETF|Test Issue\nVTI|Vanguard Total|Y|N\nAAPL|Apple|N|N\nTEST|Test|N|Y\nFile Creation Time: now\n")
	}))
	defer srv.Close()
	d := NewDirectory()
	d.USURL = srv.URL
	assets, err := d.Assets(context.Background(), "us")
	if err != nil {
		t.Fatal(err)
	}
	if len(assets) != 2 || assets[0].Kind != "etf" || assets[1].Symbol != "AAPL" {
		t.Fatalf("unexpected assets %#v", assets)
	}
}
func TestChinaDirectoryPagination(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
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
	defer srv.Close()
	d := NewDirectory()
	d.CNURL = srv.URL
	assets, err := d.Assets(context.Background(), "cn")
	if err != nil {
		t.Fatal(err)
	}
	if calls != 3 || len(assets) != 3 || assets[2].Kind != "etf" {
		t.Fatalf("calls %d assets %#v", calls, assets)
	}
}
func TestDirectoryFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(429) }))
	defer srv.Close()
	d := NewDirectory()
	d.USURL = srv.URL
	if _, err := d.Assets(context.Background(), "us"); err == nil {
		t.Fatal("HTTP failure hidden")
	}
}
