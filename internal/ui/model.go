package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"portifolio/internal/config"
	"portifolio/internal/domain"
	"portifolio/internal/marketdata"
	"portifolio/internal/plugin"
	"portifolio/internal/ui/theme"
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
	bars   []domain.Bar
	err    error
}

type directoryLoaded struct {
	assets  []domain.Asset
	status  string
	refresh bool
}

type directoryTick struct{}

type Model struct {
	cfg     config.Config
	cfgPath string
	data    *marketdata.Service

	page          page
	width, height int
	status        string

	assets []domain.Asset
	bars   map[string][]domain.Bar
	errors map[string]string

	requested      map[string]bool
	rangeRequested map[string]bool
	priceLoading   map[string]bool
	priceFrame     int
	dirLoading     bool
	dirFrame       int
	dirStatus      string

	setup    setupState
	holdings holdingsState
	add      addState
	weight   weightState
	settings settingsState
	detail   detailState
	analysis analysisState
}

func New(c *config.Config, path string, data *marketdata.Service) Model {
	m := Model{
		cfgPath:        path,
		data:           data,
		page:           setupPage,
		bars:           map[string][]domain.Bar{},
		errors:         map[string]string{},
		requested:      map[string]bool{},
		rangeRequested: map[string]bool{},
		priceLoading:   map[string]bool{},
		analysis:       newAnalysisState(path),
	}
	if c != nil {
		m.cfg = *c
		m.page = holdingsPage
	}
	return m
}

func (m *Model) Close() {
	plugin.CloseAll(m.analysis.pluginRaw)
}

func (m Model) Init() tea.Cmd {
	if m.page == setupPage {
		return nil
	}
	return m.loadAll()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = v.Width
		m.height = v.Height
		return m, nil
	case priceTick:
		if len(m.priceLoading) > 0 {
			m.priceFrame++
			return m, priceTickCmd()
		}
		return m, nil
	case directoryTick:
		if m.dirLoading {
			m.dirFrame++
			return m, directoryTickCmd()
		}
		return m, nil
	case directoryLoaded:
		m.dirLoading = false
		if v.assets != nil {
			m.assets = v.assets
			m.add.cursor = 0
		}
		m.dirStatus = v.status
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
		if m.page == detailPage && v.symbol == m.detail.asset.Symbol {
			return m, m.changeDetailRange(m.detail.grain)
		}
		return m, nil
	case tea.KeyMsg:
		if v.String() == "ctrl+c" {
			return m, tea.Quit
		}
		switch m.page {
		case setupPage:
			return m, m.updateSetup(v)
		case settingsPage:
			return m, m.updateSettings(v)
		case weightPage:
			return m, m.updateWeight(v)
		case detailPage:
			return m, m.updateDetail(v)
		case addPage:
			return m, m.updateAdd(v)
		case analysisPage:
			return m, m.updateAnalysis(v)
		case holdingsPage:
			return m, m.updateHoldings(v)
		}
	}
	return m, m.loadAll()
}

func (m Model) View() string {
	width := m.width
	if width == 0 {
		width = 100
	}
	if m.page == setupPage {
		return m.viewSetup()
	}
	p := m.cfg.Portfolio
	header := theme.Accent.Render("PORTIFOLIO / "+strings.ToUpper(p.Market)) + " · " + p.PriceCurrency + "\n" +
		theme.Muted.Render("1 My Portfolio   2 Analysis · Tab switch · a Add · / search · q quit") + "\n\n"
	if len(m.priceLoading) > 0 {
		header += m.priceAnimation() + "\n"
	}
	switch m.page {
	case weightPage:
		return header + m.viewWeight()
	case settingsPage:
		return header + m.viewSettings(width)
	case analysisPage:
		return header + m.viewAnalysisViewport(width)
	case detailPage:
		return header + m.viewDetail(width)
	case addPage:
		return header + m.viewAdd(width)
	default:
		return header + m.viewHoldings(width)
	}
}

func (m Model) positionIndex(symbol string) int {
	for i, p := range m.cfg.Portfolio.Positions {
		if p.Symbol == symbol {
			return i
		}
	}
	return -1
}

func (m *Model) cycleRebalance() {
	c := m.cfg.Clone()
	c.Portfolio.Rebalance = cycleString(config.RebalanceOptions, c.Portfolio.Rebalance, 1)
	if err := config.Save(m.cfgPath, c); err != nil {
		m.status = err.Error()
		return
	}
	m.cfg = c
}

func (m *Model) openAddPage() tea.Cmd {
	m.page = addPage
	m.add.query = ""
	m.add.cursor = 0
	m.holdings.searching = false
	m.dirStatus = "Loading asset directory…"
	return m.beginDirectory(false)
}

func trimLast(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return s
	}
	return string(r[:len(r)-1])
}

func cycleString(options []string, current string, delta int) string {
	index := 0
	for i, v := range options {
		if v == current {
			index = i
		}
	}
	n := len(options)
	return options[((index+delta)%n+n)%n]
}

func truncateText(s string, limit int) string {
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	return string(runes[:max(1, limit-1)]) + "…"
}
