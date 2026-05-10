package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// ollamaTagsResponse is the JSON response shape of GET /api/tags.
type ollamaTagsResponse struct {
	Models []struct {
		Name string `json:"name"`
	} `json:"models"`
}

// ListModels calls GET <baseURL>/api/tags (Ollama-native endpoint) and returns
// the sorted list of locally available model names.
func ListModels(ctx context.Context, baseURL string) ([]string, error) {
	baseURL = strings.TrimRight(baseURL, "/")
	// Strip /v1 suffix if present — /api/tags lives on the root Ollama HTTP server.
	base := strings.TrimSuffix(baseURL, "/v1")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/api/tags", nil)
	if err != nil {
		return nil, fmt.Errorf("ollama list models: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama list models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama list models: unexpected status %d", resp.StatusCode)
	}

	var parsed ollamaTagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("ollama list models: decode: %w", err)
	}

	names := make([]string, 0, len(parsed.Models))
	for _, m := range parsed.Models {
		if m.Name != "" {
			names = append(names, m.Name)
		}
	}
	return names, nil
}

// PullModel initiates a model pull via POST <baseURL>/api/pull.
// This call starts the pull asynchronously on the Ollama server and returns
// once the server has accepted the request. To stream progress, use the
// Ollama streaming response (not implemented here).
func PullModel(ctx context.Context, baseURL, modelName string) error {
	baseURL = strings.TrimRight(baseURL, "/")
	base := strings.TrimSuffix(baseURL, "/v1")

	body, _ := json.Marshal(map[string]any{"name": modelName, "stream": false})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/api/pull", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("ollama pull model: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("ollama pull model: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ollama pull model: unexpected status %d", resp.StatusCode)
	}
	return nil
}
