package ui

// Curated common assets, not a real-time popularity feed. Only assets already
// returned by the directory are ranked; this never invents directory entries.
var commonAssets = map[string][]string{
	"us": {"SPY", "QQQ", "VTI", "VOO", "AAPL", "MSFT", "NVDA", "AMZN", "GOOGL", "META", "TSLA", "IWM", "DIA", "BND", "TLT", "GLD"},
	"cn": {"510300", "510500", "510050", "159915", "588000", "159919", "512100", "518880", "600519", "600036", "601318", "000858", "300750"},
}

func popularRank(asset security) int {
	for i, symbol := range commonAssets[asset.Market] {
		if asset.Symbol == symbol {
			return i
		}
	}
	return 10000
}
