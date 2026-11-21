package ui

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"portifolio/internal/config"
	"portifolio/internal/domain"
	"portifolio/internal/provider"
	"portifolio/internal/storage"
)

type page int

const (
	setupPage page = iota
	addPage
	holdingsPage
	analysisPage
	weightPage
	settingsPage
	detailPage
)

type priceTick struct{}
type assetLoaded struct {
	symbol string
	bars   []provider.Bar
	err    error
}
type directoryLoaded struct {
	assets  []provider.Asset
	status  string
	refresh bool
}
type directoryTick struct{}
type Model struct {
	settingsField                      int
	settingsRebalance                  string
	settingsCapital                    string
	settingsDate                       string
	analysisScroll                     int
	analysisCard                       int
	analysisActive                     bool
	chartOffsets                       [5]int
	rangeRequested                     map[string]bool
	priceLoading                       map[string]bool
	priceFrame                         int
	detailAsset                        security
	detailGrain                        string
	detailOffset                       int
	directoryLoading                   bool
	directoryFrame                     int
	directory                          provider.AssetLister
	assets                             []security
	directoryStatus                    string
	requested                          map[string]bool
	weightSymbol                       string
	cfg                                config.Config
	cfgPath                            string
	store                              *storage.Store
	providers                          *provider.Registry
	page                               page
	width, height, cursor, marketIndex int
	query, input, status               string
	searching                          bool
	bars                               map[string][]provider.Bar
	errors                             map[string]string
	pending                            int
	grain                              string
	window, offset                     int
}

var accent = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#55d6be"))
var muted = lipgloss.NewStyle().Foreground(lipgloss.Color("#8995a5"))
var highlight = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#ffd166"))

func New(c *config.Config, path string, s *storage.Store, r *provider.Registry) Model {
	m := Model{priceLoading: map[string]bool{}, directory: provider.NewDirectory(), requested: map[string]bool{}, cfgPath: path, store: s, providers: r, page: setupPage, bars: map[string][]provider.Bar{}, errors: map[string]string{}, grain: "day", window: 365}
	if c != nil {
		m.cfg = *c
		m.page = holdingsPage
	}
	return m
}
func (m Model) Init() tea.Cmd {
	if m.page == setupPage {
		return nil
	}
	return m.loadAll()
}
func (m Model) items() []security {
	var all []security
	if len(m.cfg.Portfolios) == 0 {
		return all
	}
	p := m.cfg.Portfolios[0]
	for _, s := range m.assets {
		if s.Market == p.Market {
			all = append(all, s)
		}
	}
	for _, pos := range p.Positions {
		found := false
		for _, s := range all {
			if s.Symbol == pos.Symbol {
				found = true
			}
		}
		if !found {
			all = append(all, security{pos.Symbol, pos.Symbol, p.Market, pos.Kind})
		}
	}
	query := strings.ToLower(strings.TrimSpace(m.query))
	type match struct {
		asset security
		score int
	}
	var matches []match
	for _, s := range all {
		if m.page == holdingsPage && m.positionIndex(s.Symbol) < 0 {
			continue
		}
		symbol := strings.ToLower(s.Symbol)
		score, ok := fuzzyScore(query, strings.ToLower(s.Symbol+" "+s.Name))
		if !ok {
			continue
		}
		if query != "" {
			if symbol == query {
				score = -10000
			} else if strings.HasPrefix(symbol, query) {
				score = -5000 + len(symbol)
			}
		}
		matches = append(matches, match{s, score})
	}
	if query != "" || m.page == addPage {
		sort.SliceStable(matches, func(i, j int) bool {
			if query != "" && matches[i].score != matches[j].score {
				return matches[i].score < matches[j].score
			}
			if m.page == addPage {
				return popularRank(matches[i].asset) < popularRank(matches[j].asset)
			}
			return false
		})
	}
	out := make([]security, 0, len(matches))
	for _, item := range matches {
		out = append(out, item.asset)
	}
	return out
}
func (m Model) positionIndex(symbol string) int {
	if len(m.cfg.Portfolios) == 0 {
		return -1
	}
	for i, p := range m.cfg.Portfolios[0].Positions {
		if p.Symbol == symbol {
			return i
		}
	}
	return -1
}
func (m *Model) loadAll() tea.Cmd {
	if m.store == nil || m.providers == nil {
		return nil
	}
	var cmds []tea.Cmd
	wasLoading := len(m.priceLoading) > 0
	if m.priceLoading == nil {
		m.priceLoading = map[string]bool{}
	}
	items := m.items()
	var selected []security
	for i, s := range items {
		if i == m.cursor || m.positionIndex(s.Symbol) >= 0 {
			selected = append(selected, s)
		}
	}
	for _, s := range selected {
		if m.requested[s.Symbol] || m.priceLoading[s.Symbol] {
			continue
		}
		m.requested[s.Symbol] = true
		m.priceLoading[s.Symbol] = true
		s := s
		cfg := m.cfg
		cmds = append(cmds, func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
			defer cancel()
			p := cfg.Portfolios[0]
			start, _ := time.Parse("2006-01-02", p.StartDate)
			pos := config.Position{Symbol: s.Symbol, Market: p.Market, Kind: s.Kind, Currency: p.PriceCurrency}
			bars, err := m.store.History(ctx, pos, start, time.Now())
			if len(bars) == 0 || bars[0].Date.After(start.AddDate(0, 0, 7)) {
				fresh, fetchErr := m.providers.History(ctx, cfg.Markets, pos, start, time.Now())
				if fetchErr == nil && len(fresh) > 0 {
					bars = fresh
					err = m.store.Put(ctx, pos, bars)
				} else if fetchErr != nil {
					err = fetchErr
				}
			}
			return assetLoaded{s.Symbol, bars, err}
		})
	}
	if !wasLoading && len(m.priceLoading) > 0 {
		cmds = append(cmds, priceTickCmd())
	}
	return tea.Batch(cmds...)
}
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = v.Width
		m.height = v.Height
	case priceTick:
		if len(m.priceLoading) > 0 {
			m.priceFrame++
			return m, priceTickCmd()
		}
		return m, nil
	case directoryTick:
		if m.directoryLoading {
			m.directoryFrame++
			return m, directoryTickCmd()
		}
		return m, nil
	case directoryLoaded:
		m.directoryLoading = false
		if v.assets != nil {
			m.assets = nil
			for _, a := range v.assets {
				m.assets = append(m.assets, security{a.Symbol, a.Name, a.Market, a.Kind})
			}
			m.cursor = 0
		}
		m.directoryStatus = v.status
		if v.refresh {
			return m, tea.Batch(m.loadAll(), m.beginDirectory(true))
		}
		return m, m.loadAll()
	case assetLoaded:
		delete(m.priceLoading, v.symbol)
		m.bars[v.symbol] = v.bars
		if v.err != nil {
			m.errors[v.symbol] = v.err.Error()
		} else {
			delete(m.errors, v.symbol)
		}
		if m.page == detailPage && v.symbol == m.detailAsset.Symbol {
			return m, m.changeDetailRange(m.detailGrain)
		}
	case tea.KeyMsg:
		k := v.String()
		if k == "ctrl+c" {
			return m, tea.Quit
		}
		if m.page == setupPage {
			switch k {
			case "up", "down", "left", "right":
				m.marketIndex = 1 - m.marketIndex
			case "enter":
				market := "us"
				if m.marketIndex == 1 {
					market = "cn"
				}
				c, _ := config.New(market)
				if err := config.Save(m.cfgPath, c); err != nil {
					m.status = err.Error()
					break
				}
				m.cfg = c
				m.page = holdingsPage
				return m, m.loadAll()
			case "q":
				return m, tea.Quit
			}
			return m, nil
		}
		if m.page == settingsPage {
			return m, m.updatePortfolioSettings(v)
		}
		if m.page == weightPage {
			if k == "esc" {
				m.page = holdingsPage
				return m, nil
			}
			if k == "enter" {
				c := cloneConfig(m.cfg)
				p := &c.Portfolios[0]
				if m.page == weightPage {
					value, err := strconv.ParseFloat(m.input, 64)
					idx := m.positionIndex(m.weightSymbol)
					if err != nil || math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > 100 || idx < 0 {
						m.status = "Invalid allocation; press Esc to return"
						return m, nil
					}
					p.Positions[idx].Allocation = value / 100
				} else {
					if _, err := time.Parse("2006-01-02", m.input); err != nil {
						m.status = "Use YYYY-MM-DD"
						return m, nil
					}
					p.StartDate = m.input
				}
				if err := config.Save(m.cfgPath, c); err != nil {
					m.status = err.Error()
					return m, nil
				}
				m.cfg = c
				m.status = "Saved"
				reloadHistory := m.page == settingsPage
				m.page = holdingsPage
				if reloadHistory {
					m.offset = 0
					m.requested = map[string]bool{}
					return m, m.loadAll()
				}
				return m, nil
			}
			if k == "backspace" {
				m.input = trimLast(m.input)
			} else if v.Type == tea.KeyRunes {
				m.input += string(v.Runes)
			}
			return m, nil
		}
		if (k == "ctrl+up" || k == "ctrl+down") && (m.page == analysisPage || m.page == detailPage) {
			delta := -1
			if k == "ctrl+down" {
				delta = 1
			}
			if m.page == detailPage {
				return m, m.changeDetailRange(stepInterval(m.detailGrain, delta))
			}
			m.grain = stepInterval(m.grain, delta)
			return m, nil
		}
		if m.page == detailPage {
			switch k {
			case "z":
				return m, m.changeDetailRange(cycle([]string{"day", "week", "month", "quarter", "year"}, m.detailGrain, 1))
			case "left":
				m.detailOffset++
			case "right":
				m.detailOffset = max(0, m.detailOffset-1)
			case "home":
				m.detailOffset = 0
			case "esc", "q":
				m.page = holdingsPage
			case "d":
				return m, m.changeDetailRange("day")
			case "up":
				return m, m.changeDetailRange(stepInterval(m.detailGrain, -1))
			case "down":
				return m, m.changeDetailRange(stepInterval(m.detailGrain, 1))
			case "y":
				return m, m.changeDetailRange("year")
			case "w":
				return m, m.changeDetailRange("week")
			case "m":
				return m, m.changeDetailRange("month")
			case "c":
				return m, m.changeDetailRange("quarter")
			case "r":
				delete(m.requested, m.detailAsset.Symbol)
				return m, m.loadAll()
			}
			return m, nil
		}
		if m.page == addPage {
			switch k {
			case "esc":
				m.page = holdingsPage
				m.cursor = 0
				m.query = ""
				m.searching = false
			case "up":
				if m.cursor > 0 {
					m.cursor--
				}
			case "down":
				if m.cursor < len(m.items())-1 {
					m.cursor++
				}
			case "enter":
				items := m.items()
				if m.cursor < len(items) {
					if m.positionIndex(items[m.cursor].Symbol) < 0 {
						m.toggleFavorite()
						if m.positionIndex(items[m.cursor].Symbol) < 0 {
							return m, nil
						}
					}
					m.page = holdingsPage
					m.cursor = 0
					m.query = ""
					m.searching = false
				}
			case "backspace":
				m.query = trimLast(m.query)
				m.cursor = 0
			case "ctrl+u":
				m.query = ""
				m.cursor = 0
			case "ctrl+r":
				return m, m.beginDirectory(true)

			default:
				if v.Type == tea.KeyRunes {
					m.query += string(v.Runes)
					m.cursor = 0
				}
			}
			return m, m.loadAll()
		}
		if m.searching {
			switch k {
			case "esc", "enter":
				m.searching = false
			case "backspace":
				m.query = trimLast(m.query)
				m.cursor = 0
			default:
				if v.Type == tea.KeyRunes {
					m.query += string(v.Runes)
					m.cursor = 0
				}
			}
			return m, nil
		}
		switch k {
		case "q":
			return m, tea.Quit
		case "pgdown":
			if m.page == analysisPage {
				m.analysisScroll += max(1, m.height-8)
			}
		case "pgup":
			if m.page == analysisPage {
				m.analysisScroll = max(0, m.analysisScroll-max(1, m.height-8))
			}
		case "tab":
			if m.page == analysisPage && m.analysisActive {
				m.analysisCard = min(4, m.analysisCard+1)
				m.analysisScroll = 0
				return m, nil
			}
			if m.page == holdingsPage {
				m.page = analysisPage
			} else {
				m.page = holdingsPage
			}
			m.analysisActive = false
			m.cursor = 0
			m.query = ""
		case "1":
			m.page = holdingsPage
			m.analysisActive = false
			m.cursor = 0
			m.query = ""
		case "2":
			m.page = analysisPage
			m.analysisActive = false
			m.query = ""
		case "a", "n":
			m.page = addPage
			m.query = ""
			m.cursor = 0
			m.searching = false
			m.directoryStatus = "Loading asset directory…"
			return m, m.beginDirectory(false)
		case "/":
			m.searching = true
		case "esc":
			if m.page == analysisPage && m.analysisActive {
				m.analysisActive = false
				return m, nil
			}
			m.query = ""
			m.page = holdingsPage
			m.cursor = 0
		case "up", "k":
			if m.page == analysisPage {
				if m.analysisActive {
					if m.analysisCard == 1 || m.analysisCard == 2 || m.analysisCard == 4 {
						m.grain = stepInterval(m.grain, -1)
					}
				} else {
					m.analysisCard = max(0, m.analysisCard-1)
					m.analysisScroll = 0
				}
				return m, nil
			}
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.page == analysisPage {
				if m.analysisActive {
					if m.analysisCard == 1 || m.analysisCard == 2 || m.analysisCard == 4 {
						m.grain = stepInterval(m.grain, 1)
					}
				} else {
					m.analysisCard = min(4, m.analysisCard+1)
					m.analysisScroll = 0
				}
				return m, nil
			}
			limit := len(m.items()) - 1
			if m.page == holdingsPage {
				limit++
			}
			if m.cursor < limit {
				m.cursor++
			}
		case "x", "delete":
			if m.page == holdingsPage {
				return m, m.toggleFavorite()
			}
		case "e":
			if m.page == analysisPage {
				m.openPortfolioSettings()
				return m, nil
			}
			if m.page == holdingsPage {
				items := m.items()
				if m.cursor < len(items) && m.positionIndex(items[m.cursor].Symbol) >= 0 {
					symbol := items[m.cursor].Symbol
					m.input = strconv.FormatFloat(m.cfg.Portfolios[0].Positions[m.positionIndex(symbol)].Allocation*100, 'f', -1, 64)
					m.weightSymbol = symbol
					m.status = symbol
					m.page = weightPage
				}
			}
		case "s":
			m.openPortfolioSettings()
			m.settingsField = 2
		case "b":
			c := cloneConfig(m.cfg)
			c.Portfolios[0].Rebalance = cycle([]string{"never", "monthly", "quarterly", "yearly"}, c.Portfolios[0].Rebalance, 1)
			if err := config.Save(m.cfgPath, c); err != nil {
				m.status = err.Error()
			} else {
				m.cfg = c
			}
		case "r":
			m.requested = map[string]bool{}
			return m, tea.Batch(m.loadAll(), m.beginDirectory(true))
		case "d", "enter":
			if m.page == analysisPage {
				if k == "enter" {
					m.analysisActive = true
				} else {
					m.analysisActive = false
					m.grain = "day"
					m.window = 90
				}
				return m, nil
			}
			if m.page == holdingsPage {
				if m.cursor == len(m.items()) {
					if k == "enter" {
						m.page = addPage
						m.query = ""
						m.cursor = 0
						return m, m.beginDirectory(false)
					}
					return m, nil
				}
				items := m.items()
				if m.cursor < len(items) {
					m.detailAsset = items[m.cursor]
					m.detailGrain = "day"
					m.detailOffset = 0
					cmd := m.loadAll()
					m.page = detailPage
					return m, tea.Batch(cmd, m.changeDetailRange("day"))
				}
			}
		case "y":
			m.grain = "year"
		case "w":
			m.grain = "week"
			m.window = 365
		case "m":
			m.grain = "month"
			m.window = 365 * 5
		case "c":
			m.grain = "quarter"
			m.window = 365 * 15
		case "z":
			if m.page == analysisPage {
				m.grain = cycle([]string{"day", "week", "month", "quarter", "year"}, m.grain, 1)
			}
		case "left":
			if m.page == analysisPage && m.analysisActive {
				if m.analysisCard == 1 {
					m.offset++
				} else if m.analysisCard == 2 || m.analysisCard == 4 {
					m.chartOffsets[m.analysisCard]++
				}
			}
		case "right":
			if m.page == analysisPage && m.analysisActive {
				if m.analysisCard == 1 {
					m.offset = max(0, m.offset-1)
				} else if m.analysisCard == 2 || m.analysisCard == 4 {
					m.chartOffsets[m.analysisCard] = max(0, m.chartOffsets[m.analysisCard]-1)
				}
			}
		case "home":
			m.offset = 0
		}
	}
	return m, m.loadAll()
}
func (m *Model) toggleFavorite() tea.Cmd {
	items := m.items()
	if m.cursor >= len(items) {
		return nil
	}
	s := items[m.cursor]
	c := cloneConfig(m.cfg)
	p := &c.Portfolios[0]
	idx := m.positionIndex(s.Symbol)
	if idx >= 0 {
		p.Positions = append(p.Positions[:idx], p.Positions[idx+1:]...)
	} else {
		p.Positions = append(p.Positions, config.Position{Symbol: s.Symbol, Market: p.Market, Kind: s.Kind, Currency: p.PriceCurrency})
	}
	if err := config.Save(m.cfgPath, c); err != nil {
		m.status = err.Error()
		return nil
	}
	m.cfg = c
	m.status = "Favorites saved · press e to set target weight"
	if m.cursor >= len(m.items()) {
		m.cursor = max(0, len(m.items())-1)
	}
	return nil
}
func cloneConfig(c config.Config) config.Config {
	c.Portfolios = append([]config.Portfolio(nil), c.Portfolios...)
	for i := range c.Portfolios {
		c.Portfolios[i].Positions = append([]config.Position(nil), c.Portfolios[i].Positions...)
	}
	return c
}
func (m Model) View() string {
	width := m.width
	if width == 0 {
		width = 100
	}
	if m.page == setupPage {
		choices := []string{"United States · USD · Yahoo / Stooq", "China A-shares · CNY · Eastmoney"}
		var b strings.Builder
		b.WriteString(accent.Render("WELCOME / CHOOSE A MARKET") + "\n\n")
		for i, c := range choices {
			style := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 2)
			if i == m.marketIndex {
				style = style.BorderForeground(lipgloss.Color("#ffd166"))
				c = "▸ " + c
			}
			b.WriteString(style.Render(c) + "\n")
		}
		b.WriteString("\n↑/↓ choose · Enter open portfolio · q quit\n" + m.status)
		return b.String()
	}
	p := m.cfg.Portfolios[0]
	header := accent.Render("PORTIFOLIO / "+strings.ToUpper(p.Market)) + " · " + p.PriceCurrency + "\n" + muted.Render("1 My Portfolio   2 Analysis · Tab switch · a Add · / search · q quit") + "\n\n"
	if len(m.priceLoading) > 0 {
		header += m.priceAnimation() + "\n"
	}
	if m.page == weightPage {
		return header + "TARGET ALLOCATION (%)\n\n" + highlight.Render(m.input+"▏") + "\n\nEnter save · Esc cancel\n" + m.status
	}
	if m.page == settingsPage {
		return header + m.portfolioSettingsView(width)
	}
	if m.page == analysisPage {
		return header + m.analysisViewport(width)
	}
	if m.page == detailPage {
		return header + m.assetDetails(width)
	}
	if m.page == holdingsPage {
		return header + m.holdingsCards(width)
	}
	if m.page == addPage {
		return header + m.addCards(width)
	}
	var b strings.Builder
	b.WriteString(header)
	label := "ADD POSITION / SEARCH ASSET DIRECTORY"
	if m.page == holdingsPage {
		label = "MY PORTFOLIO"
	}
	help := "↑/↓ browse · a Add · x remove · e target weight · r load prices · s start date · b rebalance"
	if m.page == addPage {
		help = "Type to fuzzy-search · ↑/↓ select · Enter add · Esc cancel · Ctrl+R refresh directory"
		b.WriteString(assetSearchInput(m.query, width) + "\n")
	}
	b.WriteString(accent.Render(label) + "\n" + muted.Render(help) + "\n")
	if m.page == addPage && m.directoryLoading {
		b.WriteString(m.directoryProgress(width) + "\n")
	}
	b.WriteString(fmt.Sprintf("%-3s %-5s %-12s %-30s %12s %10s %9s\n", "", "FAV", "SYMBOL", "NAME", "LAST", "DAY %", "WEIGHT"))
	items := m.items()
	if len(items) == 0 {
		if m.page == addPage {
			b.WriteString("No matching assets. Loading directory or change your search.\n")
		} else {
			b.WriteString("Your portfolio is empty. Press a to Add a position.\n")
		}
	}
	start := max(0, m.cursor-max(4, m.height/3))
	end := min(len(items), start+max(6, m.height/2))
	for i := start; i < end; i++ {
		s := items[i]
		fav := "☆"
		weight := "—"
		if idx := m.positionIndex(s.Symbol); idx >= 0 {
			fav = "★"
			weight = fmt.Sprintf("%.1f%%", p.Positions[idx].Allocation*100)
		}
		price, change := "—", "—"
		bars := m.bars[s.Symbol]
		if len(bars) > 0 {
			price = fmt.Sprintf("%.2f", bars[len(bars)-1].Close)
			if len(bars) > 1 {
				last, prev := bars[len(bars)-1].Close, bars[len(bars)-2].Close
				if prev > 0 {
					change = fmt.Sprintf("%+.2f%%", (last/prev-1)*100)
				}
			}
		}
		prefix := "  "
		if i == m.cursor {
			prefix = "▸ "
		}
		line := fmt.Sprintf("%s %-5s %-12s %-30s %12s %10s %9s", prefix, fav, s.Symbol, s.Name, price, change, weight)
		if i == m.cursor {
			line = highlight.Render(line)
		}
		b.WriteString(line + "\n")
	}
	if m.cursor < len(items) {
		s := items[m.cursor]
		bars := m.bars[s.Symbol]
		if len(bars) > 1 {
			var pts []domain.Point
			for _, bar := range bars {
				pts = append(pts, domain.Point{Date: bar.Date, Value: bar.Close})
			}
			b.WriteString("\n" + s.Symbol + " · latest cached close: " + bars[len(bars)-1].Date.Format("2006-01-02") + "\n" + renderChart(pts, max(20, width-8), 5))
		}
		if err := m.errors[s.Symbol]; err != "" {
			b.WriteString("\nPrice unavailable: " + err)
		}
	}
	b.WriteString(fmt.Sprintf("\n%d favorites · target total %.1f%% · rebalance %s", len(p.Positions), totalAllocation(p.Positions)*100, p.Rebalance))
	if m.page != addPage && (m.searching || m.query != "") {
		b.WriteString("\nSearch: " + m.query + "▏")
	}
	b.WriteString("\n" + m.directoryStatus + "\n" + m.status)
	return b.String()
}
func (m Model) analysisView(width int) string {
	p := m.cfg.Portfolios[0]
	heading := accent.Render("PORTFOLIO ANALYSIS") + "\n" + muted.Render("Esc back · r retry · ↑/↓ select card · Enter activate · z / Ctrl+↑↓ interval · e settings · s start date · b rebalance") + "\n\n"
	style := m.analysisCardStyle(width, 0)
	information := func(body string) string {
		return heading + style.Render(accent.Render("PORTFOLIO INFORMATION")+"\n"+p.Name+"\n"+body)
	}
	if len(p.Positions) == 0 {
		return information("Press a to Add positions, then assign target weights in My Portfolio.")
	}
	total := totalAllocation(p.Positions)
	if total <= 0 {
		return information("Set at least one positive target weight before analysis.")
	}
	// Normalize a copy so analysis never rewrites the user's saved weights.
	p.Positions = append([]config.Position(nil), p.Positions...)
	for i := range p.Positions {
		p.Positions[i].Allocation /= total
	}
	weightNote := fmt.Sprintf("Configured weight total %.1f%% · normalized to 100%% for analysis", total*100)
	var ss []domain.AssetSeries
	for _, pos := range p.Positions {
		if pos.Allocation == 0 {
			continue
		}
		bars := m.bars[pos.Symbol]
		if len(bars) < 2 {
			return information("History missing for " + pos.Symbol + ". Press r to retry.")
		}
		var pts []domain.Point
		for _, bar := range bars {
			v := bar.AdjustedClose
			if v <= 0 {
				v = bar.Close
			}
			pts = append(pts, domain.Point{Date: bar.Date, Value: v})
		}
		ss = append(ss, domain.AssetSeries{Position: pos, Points: pts})
	}
	curve := domain.BuildPortfolio(p, ss)
	if len(curve) < 2 {
		return information("No common history for this portfolio.")
	}
	startNote := effectiveStartNote(p.StartDate, curve[0].Date, ss)
	peak, drawdown := curve[0].Value, 0.0
	for _, pt := range curve {
		peak = max(peak, pt.Value)
		drawdown = min(drawdown, pt.Value/peak-1)
	}
	latest := curve[len(curve)-1]
	capital := p.InitialCapital
	if capital <= 0 {
		capital = 10000
	}
	recent := "—"
	if percent, amount, ok := recentChange(curve, capital); ok {
		recent = latest.Date.Format("2006-01-02") + "  " + coloredPercent(percent) + "  " + coloredAmount(amount, p.BaseCurrency)
	}
	rows := [][2]string{
		{"Model index", fmt.Sprintf("%.2f", latest.Value)},
		{"Total return", coloredPercent(latest.Value - 100)},
		{"Max drawdown", coloredPercent(drawdown * 100)},
		{"Recent day", recent},
		{},
		{"Requested start", p.StartDate},
		{"Effective start", curve[0].Date.Format("2006-01-02")},
		{"Latest data", latest.Date.Format("2006-01-02")},
		{"Rebalance", p.Rebalance},
		{"Initial capital", fmt.Sprintf("%.2f %s", capital, p.BaseCurrency)},
		{"Model value", fmt.Sprintf("%.2f %s", capital*latest.Value/100, p.BaseCurrency)},
	}
	summary := renderMetricRows(rows) + "\n"
	summary += muted.Render(weightNote)
	if startNote != "" {
		summary += "\n" + strings.TrimSpace(startNote)
	}
	return information(summary) + "\n" + m.analysisCandles(curve, width) + "\n" + m.additionalAnalysisCharts(p, curve, width)
}
func renderChart(points []domain.Point, width, height int) string {
	if len(points) < 2 {
		return muted.Render("No price history available")
	}
	width = max(20, width)
	height = max(4, height)
	lo, hi := points[0].Value, points[0].Value
	for _, p := range points {
		lo = min(lo, p.Value)
		hi = max(hi, p.Value)
	}
	if hi == lo {
		hi++
	}
	grid := make([][]rune, height)
	for i := range grid {
		grid[i] = []rune(strings.Repeat(" ", width))
	}
	for x := 0; x < width; x++ {
		v := points[x*(len(points)-1)/(width-1)].Value
		y := height - 1 - int((v-lo)/(hi-lo)*float64(height-1))
		grid[y][x] = '•'
	}
	var b strings.Builder
	for _, row := range grid {
		b.WriteString(string(row) + "\n")
	}
	b.WriteString(points[0].Date.Format("2006-01-02") + " → " + points[len(points)-1].Date.Format("2006-01-02"))
	return b.String()
}
func totalAllocation(positions []config.Position) float64 {
	n := 0.0
	for _, p := range positions {
		n += p.Allocation
	}
	return n
}
func trimLast(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return s
	}
	return string(r[:len(r)-1])
}
func cycle(options []string, current string, delta int) string {
	index := 0
	for i, v := range options {
		if v == current {
			index = i
		}
	}
	return options[(index+delta+len(options))%len(options)]
}
