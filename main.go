package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const baseURL = "https://www.speedrun.com/api/v2"

var (
	appStyle = lipgloss.NewStyle().
			Padding(1, 1)

	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#000000")).
			Background(lipgloss.Color("#EAB308")).
			Bold(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderLeft(true).
			BorderLeftForeground(lipgloss.Color("#06B6D4")).
			Padding(0, 1)

	selectedItemStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#000000")).
				BorderStyle(lipgloss.NormalBorder()).
				BorderLeft(true).
				BorderLeftForeground(lipgloss.Color("#06B6D4")).
				Padding(0, 1)

	unselectedItemStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.NormalBorder()).
				BorderLeft(true).
				BorderLeftForeground(lipgloss.Color("#06B6D4")).
				Padding(0, 1)

	statusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#94A3B8")).
			Border(lipgloss.NormalBorder()).
			BorderTop(true).
			BorderTopForeground(lipgloss.Color("#164E63")).
			Padding(1, 1)

	unreadCountStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FACC15")).
				Background(lipgloss.Color("#000000")).
				Border(lipgloss.NormalBorder()).
				BorderForeground(lipgloss.Color("#FACC15")).
				Padding(0, 1)

	urlStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#22D3EE")).
			Faint(true)

	readDotStyle = lipgloss.NewStyle().
			SetString("[✓]").
			Foreground(lipgloss.Color("#22C55E"))

	unreadDotStyle = lipgloss.NewStyle().
			SetString("[!]").
			Foreground(lipgloss.Color("#EAB308"))
)

// API types
type Notification struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Path  string `json:"path"`
	Read  bool   `json:"read"`
	Date  int64  `json:"date"`
}

type Pagination struct {
	Count int `json:"count"`
	Page  int `json:"page"`
	Pages int `json:"pages"`
	Per   int `json:"per"`
}

type NotificationResponse struct {
	UnreadCount   int            `json:"unreadCount"`
	Notifications []Notification `json:"notifications"`
	Pagination    Pagination     `json:"pagination"`
}

type RequestBody struct {
	U int `json:"u"`
	I int `json:"i"`
}

// Client for API calls
type Client struct {
	httpClient *http.Client
	sessionID  string
}

func NewClient(sessionID string) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		sessionID: sessionID,
	}
}

func (c *Client) GetNotifications() (*NotificationResponse, error) {
	body := RequestBody{
		U: 1,
		I: 1,
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshaling request body: %w", err)
	}

	req, err := http.NewRequest("POST", baseURL+"/GetNotifications", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://www.speedrun.com")
	req.Header.Set("Referer", "https://www.speedrun.com/notifications")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	req.AddCookie(&http.Cookie{
		Name:  "PHPSESSID",
		Value: c.sessionID,
	})

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var result NotificationResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &result, nil
}

// Model for the TUI
type model struct {
	notifications []Notification
	viewport      viewport.Model
	selected      int
	unreadCount   int
	pagination    Pagination
	err           error
	width         int
	height        int
}

func initialModel(client *Client) model {
	result, err := client.GetNotifications()
	if err != nil {
		return model{err: err}
	}

	v := viewport.New(78, 20)
	v.Style = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#3B82F6"))

	return model{
		notifications: result.Notifications,
		viewport:      v,
		unreadCount:   result.UnreadCount,
		pagination:    result.Pagination,
		selected:      0,
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			if m.selected > 0 {
				m.selected--
			}
		case "down", "j":
			if m.selected < len(m.notifications)-1 {
				m.selected++
			}
		case "enter":
			if m.selected >= 0 && m.selected < len(m.notifications) {
				notification := m.notifications[m.selected]
				url := "https://www.speedrun.com" + notification.Path
				openBrowser(url)
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width - 4
		m.viewport.Height = msg.Height - 8
	}

	m.viewport.SetContent(m.renderContent())
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m model) renderContent() string {
	var b strings.Builder

	for i, n := range m.notifications {
		item := m.renderNotification(n)
		style := unselectedItemStyle
		if i == m.selected {
			style = selectedItemStyle
		}
		b.WriteString(style.Render(item))
		b.WriteString("\n")
	}

	return b.String()
}

func (m model) renderNotification(n Notification) string {
	var b strings.Builder

	// Status and date
	readStatus := unreadDotStyle.String()
	if n.Read {
		readStatus = readDotStyle.String()
	}
	date := time.Unix(n.Date, 0).Format("2006-01-02 15:04:05")
	b.WriteString(fmt.Sprintf("%s %s\n", readStatus, date))

	// Title
	b.WriteString(n.Title)
	b.WriteString("\n")

	// URL
	b.WriteString(urlStyle.Render("speedrun.com" + n.Path))

	return b.String()
}

func (m model) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v", m.err)
	}

	// Header
	header := titleStyle.Render("SPEEDRUN.COM NOTIFICATIONS")
	unreadCount := unreadCountStyle.Render(fmt.Sprintf("%d unread", m.unreadCount))
	header = lipgloss.JoinHorizontal(lipgloss.Center, header, unreadCount)

	// Status bar
	statusBar := statusBarStyle.Render(
		fmt.Sprintf("Page %d/%d • ↑/↓ or j/k to navigate • enter to open • q to quit",
			m.pagination.Page, m.pagination.Pages))

	// Combine all elements
	return appStyle.Render(
		lipgloss.JoinVertical(
			lipgloss.Left,
			header,
			m.viewport.View(),
			statusBar,
		))
}

func openBrowser(url string) error {
	var err error
	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("cmd", "/c", "start", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}
	return err
}

func main() {
	sessionID := flag.String("session", "", "Speedrun.com PHPSESSID cookie value")
	flag.Parse()

	if *sessionID == "" {
		fmt.Println("Please provide your PHPSESSID using the -session flag")
		os.Exit(1)
	}

	client := NewClient(*sessionID)
	p := tea.NewProgram(
		initialModel(client),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}
