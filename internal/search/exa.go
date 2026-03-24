package search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	BaseURL string
	APIKey  string
	HTTP    *http.Client
}

type Result struct {
	Title   string
	URL     string
	Text    string
	Snippet string
}

type searchRequest struct {
	Query       string `json:"query"`
	Type        string `json:"type"`
	NumResults  int    `json:"num_results"`
	Contents    any    `json:"contents,omitempty"`
	Category    string `json:"category,omitempty"`
	MaxAgeHours int    `json:"maxAgeHours,omitempty"`
}

type searchResponse struct {
	Results []struct {
		Title   string `json:"title"`
		URL     string `json:"url"`
		Text    string `json:"text"`
		Snippet string `json:"snippet"`
	} `json:"results"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func New(baseURL, apiKey string) *Client {
	if baseURL == "" {
		baseURL = "https://api.exa.ai/search"
	}
	return &Client{BaseURL: strings.TrimRight(baseURL, "/"), APIKey: apiKey, HTTP: &http.Client{Timeout: 60 * time.Second}}
}

func (c *Client) Search(ctx context.Context, query string, numResults int) ([]Result, error) {
	if strings.TrimSpace(c.APIKey) == "" {
		return nil, fmt.Errorf("EXA_API_KEY is not set")
	}
	if numResults <= 0 {
		numResults = 5
	}
	reqBody := searchRequest{
		Query:      query,
		Type:       "auto",
		NumResults: numResults,
		Contents:   map[string]any{"text": map[string]any{"max_characters": 12000}},
	}
	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.APIKey)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var parsed searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, err
	}
	if parsed.Error != nil {
		return nil, fmt.Errorf("%s", parsed.Error.Message)
	}
	out := make([]Result, 0, len(parsed.Results))
	for _, item := range parsed.Results {
		out = append(out, Result{Title: item.Title, URL: item.URL, Text: item.Text, Snippet: item.Snippet})
	}
	return out, nil
}
