package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// languageResponse is the JSON object returned by the language service.
type languageResponse struct {
	Lang string `json:"lang"`
}

// captionText separates cue payloads with newlines so adjacent words do not merge.
func captionText(cues []Cue) string {
	texts := make([]string, 0, len(cues))
	for _, cue := range cues {
		if cue.Text != "" {
			texts = append(texts, cue.Text)
		}
	}

	return strings.Join(texts, "\n")
}

// detectLanguage returns the service's language unchanged. The caller decides
// whether it passes validation; transport and response errors prevent validation.
func detectLanguage(endpoint, text string, timeout time.Duration) (string, error) {
	req, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(text))
	if err != nil {
		return "", fmt.Errorf("create language request: %w", err)
	}

	req.Header.Set("Content-Type", "text/plain; charset=utf-8")

	// Bound the entire exchange and surface redirects as HTTP errors rather than
	// following them (which can also turn a POST into a GET).
	client := &http.Client{Timeout: timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("language request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("language service returned HTTP %d", resp.StatusCode)
	}

	const maxResponse = 1 << 20

	// One extra byte distinguishes an oversized body from one exactly at the limit.
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponse+1))
	if err != nil {
		return "", fmt.Errorf("read language response: %w", err)
	}

	if len(body) > maxResponse {
		return "", fmt.Errorf("language response exceeds 1 MiB")
	}

	// Unmarshal checks the entire body, rejecting trailing JSON or garbage while
	// allowing additional object fields the service may introduce later.
	var result languageResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("invalid language response: %w", err)
	}

	if strings.TrimSpace(result.Lang) == "" {
		return "", fmt.Errorf("language response must contain a nonempty string lang")
	}

	return result.Lang, nil
}
