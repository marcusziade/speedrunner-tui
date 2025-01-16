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

// RequestBody represents the POST request body
type RequestBody struct {
	U int `json:"u"`
	I int `json:"i"`
}

// Let's just use map[string]interface{} initially to see the structure
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

func (c *Client) GetNotifications() (map[string]interface{}, error) {
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

	// Read the raw response first
	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	// Print raw response for debugging
	fmt.Printf("Raw response: %s\n", string(rawBody))

	var result map[string]interface{}
	if err := json.Unmarshal(rawBody, &result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return result, nil
}

func main() {
	sessionID := flag.String("session", "", "Speedrun.com PHPSESSID cookie value")
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

	// Pretty print the result
	prettyJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fmt.Printf("Error formatting JSON: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nParsed JSON response:\n%s\n", string(prettyJSON))
}

