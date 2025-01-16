package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const (
	baseURL = "https://www.speedrun.com/api/v2"
)

type Notification struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Path  string `json:"path"`
	Read  bool   `json:"read"`
	Date  int64  `json:"date"` // Unix timestamp
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
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36")

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

func formatDate(timestamp int64) string {
	t := time.Unix(timestamp, 0)
	return t.Format("2006-01-02 15:04:05")
}

func main() {
	var (
		sessionID  = flag.String("session", "", "Speedrun.com PHPSESSID cookie value")
		jsonOutput = flag.Bool("json", false, "Output in JSON format")
		showRead   = flag.Bool("all", false, "Show both read and unread notifications")
	)
	flag.Parse()

	if *sessionID == "" {
		fmt.Println("Please provide your PHPSESSID using the -session flag")
		os.Exit(1)
	}

	client := NewClient(*sessionID)
	result, err := client.GetNotifications()
	if err != nil {
		fmt.Printf("Error getting notifications: %v\n", err)
		os.Exit(1)
	}

	if *jsonOutput {
		// Pretty print JSON output
		out, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			fmt.Printf("Error formatting JSON: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(out))
		return
	}

	// Print summary
	fmt.Printf("Total notifications: %d (Unread: %d)\n", result.Pagination.Count, result.UnreadCount)
	if result.Pagination.Pages > 1 {
		fmt.Printf("Page %d of %d\n", result.Pagination.Page, result.Pagination.Pages)
	}
	fmt.Println()

	// Print notifications
	for _, n := range result.Notifications {
		if !*showRead && n.Read {
			continue // Skip read notifications unless -all flag is set
		}

		readStatus := " "
		if !n.Read {
			readStatus = "*"
		}

		fmt.Printf("[%s] %s\n  %s\n  https://www.speedrun.com%s\n\n",
			readStatus,
			formatDate(n.Date),
			n.Title,
			n.Path)
	}
}

