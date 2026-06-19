package main

import (
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ── Styles ────────────────────────────────────────────────────────────────────

var (
	purple = lipgloss.Color("#7C3AED")
	gray   = lipgloss.Color("#9CA3AF")
	muted  = lipgloss.Color("#6B7280")
	red    = lipgloss.Color("#EF4444")

	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(purple).Padding(0, 1)
	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(purple)
	dimStyle      = lipgloss.NewStyle().Foreground(gray)
	statusStyle   = lipgloss.NewStyle().Foreground(muted)
	errStyle      = lipgloss.NewStyle().Foreground(red)
	helpStyle     = lipgloss.NewStyle().Foreground(gray)
	dotStyle      = lipgloss.NewStyle().Foreground(purple)
)

// ── App state ─────────────────────────────────────────────────────────────────

type appState int

const (
	stateFeedList    appState = iota
	stateArticleList
	stateArticleView
	stateAddFeed
)

// ── List items ────────────────────────────────────────────────────────────────

type feedItem struct{ f *Feed }

func (fi feedItem) FilterValue() string { return fi.f.Title }

type articleItem struct{ a *Article }

func (ai articleItem) FilterValue() string { return ai.a.Title }

// ── Feed list delegate ────────────────────────────────────────────────────────

type feedDelegate struct{}

func (d feedDelegate) Height() int                              { return 2 }
func (d feedDelegate) Spacing() int                             { return 0 }
func (d feedDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d feedDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	fi, ok := item.(feedItem)
	if !ok {
		return
	}
	title := fi.f.Title
	if title == "" {
		title = fi.f.URL
	}
	unread := 0
	for _, a := range fi.f.Articles {
		if !a.Read {
			unread++
		}
	}
	var line1 string
	if index == m.Index() {
		line1 = selectedStyle.Render("▸ " + title)
	} else {
		line1 = "  " + title
	}
	sub := fmt.Sprintf("  %d articles", len(fi.f.Articles))
	if unread > 0 {
		sub = fmt.Sprintf("  %d unread / %d total", unread, len(fi.f.Articles))
	}
	fmt.Fprintf(w, "%s\n%s", line1, dimStyle.Render(sub))
}

// ── Article list delegate ─────────────────────────────────────────────────────

type articleDelegate struct{}

func (d articleDelegate) Height() int                              { return 2 }
func (d articleDelegate) Spacing() int                             { return 0 }
func (d articleDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d articleDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	ai, ok := item.(articleItem)
	if !ok {
		return
	}
	title := ai.a.Title
	if title == "" {
		title = "(no title)"
	}
	var line1 string
	if index == m.Index() {
		line1 = selectedStyle.Render("▸ " + title)
	} else if !ai.a.Read {
		line1 = dotStyle.Render("● ") + title
	} else {
		line1 = "  " + title
	}
	fmt.Fprintf(w, "%s\n%s", line1, dimStyle.Render("  "+ai.a.Published))
}

// ── Messages ──────────────────────────────────────────────────────────────────

type feedFetchedMsg struct {
	url  string
	feed *Feed
	err  error
}

// ── Model ─────────────────────────────────────────────────────────────────────

type Model struct {
	state   appState
	feeds   []*Feed
	data    *storedData

	feedList    list.Model
	articleList list.Model
	viewport    viewport.Model
	input       textinput.Model

	currentFeedIdx int
	pendingFetches int

	width  int
	height int

	statusMsg string
	errMsg    string
}

func newModel() (*Model, error) {
	data, err := loadData()
	if err != nil {
		return nil, err
	}

	fl := list.New([]list.Item{}, feedDelegate{}, 80, 20)
	fl.Title = "goread"
	fl.SetShowStatusBar(false)
	fl.SetFilteringEnabled(false)
	fl.SetShowHelp(false)
	fl.Styles.Title = titleStyle

	al := list.New([]list.Item{}, articleDelegate{}, 80, 20)
	al.SetShowStatusBar(false)
	al.SetFilteringEnabled(true)
	al.SetShowHelp(false)
	al.Styles.Title = titleStyle

	vp := viewport.New(80, 20)

	ti := textinput.New()
	ti.Placeholder = "https://example.com/feed.rss"
	ti.CharLimit = 512

	m := &Model{
		data:        data,
		feedList:    fl,
		articleList: al,
		viewport:    vp,
		input:       ti,
	}

	for _, url := range data.FeedURLs {
		title := data.Titles[url]
		if title == "" {
			title = url
		}
		m.feeds = append(m.feeds, &Feed{URL: url, Title: title})
	}
	m.syncFeedList()

	return m, nil
}

func (m *Model) Init() tea.Cmd {
	if len(m.feeds) == 0 {
		return nil
	}
	m.pendingFetches = len(m.feeds)
	m.statusMsg = "Fetching feeds..."
	return m.fetchAll()
}

// ── Update ────────────────────────────────────────────────────────────────────

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resize()
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

	case feedFetchedMsg:
		if m.pendingFetches > 0 {
			m.pendingFetches--
		}
		if msg.err == nil {
			for i, f := range m.feeds {
				if f.URL == msg.url {
					for j := range msg.feed.Articles {
						if m.data.Read[msg.feed.Articles[j].GUID] {
							msg.feed.Articles[j].Read = true
						}
					}
					m.feeds[i] = msg.feed
					m.data.Titles[msg.url] = msg.feed.Title
					break
				}
			}
			_ = saveData(m.data)
		} else {
			m.errMsg = fmt.Sprintf("fetch error: %v", msg.err)
		}
		if m.pendingFetches == 0 {
			m.statusMsg = "Updated " + time.Now().Format("15:04")
		}
		m.syncFeedList()
		if m.state == stateArticleList {
			m.syncArticleList()
		}
		return m, nil
	}

	switch m.state {
	case stateFeedList:
		return m.updateFeedList(msg)
	case stateArticleList:
		return m.updateArticleList(msg)
	case stateArticleView:
		return m.updateArticleView(msg)
	case stateAddFeed:
		return m.updateAddFeed(msg)
	}
	return m, nil
}

func (m *Model) updateFeedList(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q":
			return m, tea.Quit
		case "a":
			m.state = stateAddFeed
			m.input.SetValue("")
			return m, m.input.Focus()
		case "d", "x":
			idx := m.feedList.Index()
			if idx >= 0 && idx < len(m.feeds) {
				m.deleteFeed(idx)
			}
			return m, nil
		case "r":
			if len(m.feeds) == 0 {
				return m, nil
			}
			m.pendingFetches = len(m.feeds)
			m.statusMsg = "Refreshing..."
			m.errMsg = ""
			return m, m.fetchAll()
		case "enter", " ":
			idx := m.feedList.Index()
			if idx >= 0 && idx < len(m.feeds) {
				m.currentFeedIdx = idx
				m.state = stateArticleList
				m.syncArticleList()
				m.articleList.ResetSelected()
			}
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.feedList, cmd = m.feedList.Update(msg)
	return m, cmd
}

func (m *Model) updateArticleList(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q":
			m.state = stateFeedList
			return m, nil
		case "enter", " ":
			idx := m.articleList.Index()
			if m.currentFeedIdx < len(m.feeds) {
				feed := m.feeds[m.currentFeedIdx]
				if idx >= 0 && idx < len(feed.Articles) {
					a := &feed.Articles[idx]
					a.Read = true
					m.data.Read[a.GUID] = true
					_ = saveData(m.data)
					m.viewport.SetContent(m.renderArticle(a))
					m.viewport.GotoTop()
					m.state = stateArticleView
					m.syncArticleList()
				}
			}
			return m, nil
		case "o":
			idx := m.articleList.Index()
			if m.currentFeedIdx < len(m.feeds) {
				feed := m.feeds[m.currentFeedIdx]
				if idx >= 0 && idx < len(feed.Articles) {
					_ = openURL(feed.Articles[idx].Link)
				}
			}
			return m, nil
		case "r":
			if m.currentFeedIdx < len(m.feeds) {
				url := m.feeds[m.currentFeedIdx].URL
				m.pendingFetches++
				m.statusMsg = "Refreshing..."
				m.errMsg = ""
				return m, func() tea.Msg {
					f, err := fetchFeed(url)
					return feedFetchedMsg{url: url, feed: f, err: err}
				}
			}
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.articleList, cmd = m.articleList.Update(msg)
	return m, cmd
}

func (m *Model) updateArticleView(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q":
			m.state = stateArticleList
			return m, nil
		case "o":
			idx := m.articleList.Index()
			if m.currentFeedIdx < len(m.feeds) {
				feed := m.feeds[m.currentFeedIdx]
				if idx >= 0 && idx < len(feed.Articles) {
					_ = openURL(feed.Articles[idx].Link)
				}
			}
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m *Model) updateAddFeed(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.state = stateFeedList
			return m, nil
		case "enter":
			url := strings.TrimSpace(m.input.Value())
			if url != "" {
				m.addFeed(url)
				m.state = stateFeedList
				m.pendingFetches++
				m.statusMsg = "Fetching feed..."
				m.errMsg = ""
				return m, func() tea.Msg {
					f, err := fetchFeed(url)
					return feedFetchedMsg{url: url, feed: f, err: err}
				}
			}
			m.state = stateFeedList
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// ── View ──────────────────────────────────────────────────────────────────────

func (m *Model) View() string {
	if m.width == 0 {
		return "Initializing..."
	}
	switch m.state {
	case stateFeedList:
		return m.viewFeedList()
	case stateArticleList:
		return m.viewArticleList()
	case stateArticleView:
		return m.viewArticleView()
	case stateAddFeed:
		return m.viewAddFeed()
	}
	return ""
}

func (m *Model) viewFeedList() string {
	m.feedList.Title = m.makeTitle("goread")
	help := helpStyle.Render("↑/↓ navigate  enter open  a add  d/x delete  r refresh  q quit")
	return m.feedList.View() + "\n" + help
}

func (m *Model) viewArticleList() string {
	feedTitle := ""
	if m.currentFeedIdx < len(m.feeds) {
		feedTitle = m.feeds[m.currentFeedIdx].Title
	}
	m.articleList.Title = m.makeTitle(feedTitle)
	help := helpStyle.Render("↑/↓ navigate  enter read  o open browser  r refresh  esc back")
	return m.articleList.View() + "\n" + help
}

func (m *Model) viewArticleView() string {
	idx := m.articleList.Index()
	var articleTitle, link string
	if m.currentFeedIdx < len(m.feeds) {
		feed := m.feeds[m.currentFeedIdx]
		if idx >= 0 && idx < len(feed.Articles) {
			articleTitle = feed.Articles[idx].Title
			link = feed.Articles[idx].Link
		}
	}
	pct := fmt.Sprintf("%d%%", int(m.viewport.ScrollPercent()*100))
	header := titleStyle.Render(truncate(articleTitle, m.width-6)) +
		"  " + dimStyle.Render(pct)
	linkLine := dimStyle.Render("  " + truncate(link, m.width-4))
	help := helpStyle.Render("↑/↓/PgUp/PgDn scroll  o open browser  esc back")
	return header + "\n" + m.viewport.View() + "\n" + linkLine + "\n" + help
}

func (m *Model) viewAddFeed() string {
	header := titleStyle.Render("Add Feed")
	body := "\n\n  Enter RSS/Atom feed URL:\n\n  " + m.input.View()
	foot := "\n\n" + helpStyle.Render("  enter confirm  esc cancel")
	return header + body + foot
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func (m *Model) resize() {
	// list.SetSize total height includes the title bar (~2 lines)
	m.feedList.SetSize(m.width, m.height-1)
	m.articleList.SetSize(m.width, m.height-1)
	m.viewport.Width = m.width
	m.viewport.Height = m.height - 3 // title + link + help
	m.input.Width = m.width - 6
}

func (m *Model) makeTitle(base string) string {
	if m.errMsg != "" {
		return base + "  " + errStyle.Render(m.errMsg)
	}
	if m.statusMsg != "" {
		return base + "  " + statusStyle.Render(m.statusMsg)
	}
	return base
}

func (m *Model) syncFeedList() {
	items := make([]list.Item, len(m.feeds))
	for i, f := range m.feeds {
		items[i] = feedItem{f: f}
	}
	m.feedList.SetItems(items)
}

func (m *Model) syncArticleList() {
	if m.currentFeedIdx >= len(m.feeds) {
		return
	}
	feed := m.feeds[m.currentFeedIdx]
	items := make([]list.Item, len(feed.Articles))
	for i := range feed.Articles {
		items[i] = articleItem{a: &feed.Articles[i]}
	}
	m.articleList.SetItems(items)
}

func (m *Model) addFeed(url string) {
	for _, f := range m.feeds {
		if f.URL == url {
			return
		}
	}
	m.feeds = append(m.feeds, &Feed{URL: url, Title: url})
	m.data.FeedURLs = append(m.data.FeedURLs, url)
	m.syncFeedList()
	_ = saveData(m.data)
}

func (m *Model) deleteFeed(idx int) {
	if idx < 0 || idx >= len(m.feeds) {
		return
	}
	url := m.feeds[idx].URL
	m.feeds = append(m.feeds[:idx], m.feeds[idx+1:]...)
	for i, u := range m.data.FeedURLs {
		if u == url {
			m.data.FeedURLs = append(m.data.FeedURLs[:i], m.data.FeedURLs[i+1:]...)
			break
		}
	}
	delete(m.data.Titles, url)
	m.syncFeedList()
	_ = saveData(m.data)
}

func (m *Model) fetchAll() tea.Cmd {
	cmds := make([]tea.Cmd, len(m.feeds))
	for i, f := range m.feeds {
		url := f.URL
		cmds[i] = func() tea.Msg {
			feed, err := fetchFeed(url)
			return feedFetchedMsg{url: url, feed: feed, err: err}
		}
	}
	return tea.Batch(cmds...)
}

func (m *Model) renderArticle(a *Article) string {
	var sb strings.Builder
	sb.WriteString(a.Title + "\n")
	sb.WriteString(strings.Repeat("─", min(len(a.Title), m.width-4)) + "\n")
	if a.Published != "" {
		sb.WriteString(a.Published + "\n\n")
	}
	if a.Content != "" {
		sb.WriteString(wordWrap(a.Content, m.width-4))
	} else {
		sb.WriteString("(No content available — press o to open in browser.)")
	}
	return sb.String()
}

func wordWrap(text string, width int) string {
	if width <= 0 {
		return text
	}
	var out strings.Builder
	for _, para := range strings.Split(text, "\n") {
		words := strings.Fields(para)
		if len(words) == 0 {
			out.WriteByte('\n')
			continue
		}
		col := 0
		for i, w := range words {
			if col > 0 && col+1+len(w) > width {
				out.WriteByte('\n')
				col = 0
			} else if i > 0 {
				out.WriteByte(' ')
				col++
			}
			out.WriteString(w)
			col += len(w)
		}
		out.WriteByte('\n')
	}
	return out.String()
}

func truncate(s string, maxLen int) string {
	if maxLen <= 0 || len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

func openURL(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}
