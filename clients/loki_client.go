package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// LokiClient
type LokiClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewLokiClient
func NewLokiClient(baseURL string) *LokiClient {
	return &LokiClient{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// FetchErrorLogs to last X duration logs with ERROR
func (l *LokiClient) FetchErrorLogs(
	ctx context.Context,
	deploymentName string,
	namespace string,
	duration time.Duration, 
) ([]string, error) {

	return l.fetchLogs(ctx, deploymentName, namespace, duration, "(?i)error|fatal|panic")
}

// FetchRecentLogs to recent logs without ERROR filter
func (l *LokiClient) FetchRecentLogs(
	ctx context.Context,
	deploymentName string,
	namespace string,
	duration time.Duration,
) ([]string, error) {

	return l.fetchLogs(ctx, deploymentName, namespace, duration, "")
}

func (l *LokiClient) fetchLogs(
	ctx context.Context,
	deploymentName string,
	namespace string,
	duration time.Duration,
	filterRegex string,
) ([]string, error) {

	// LogQL query
	query := fmt.Sprintf(
		`{namespace="%s",pod=~"%s-.*"}`,
		namespace,
		deploymentName,
	)
	if filterRegex != "" {
		query = fmt.Sprintf(`%s |~ "%s"`, query, filterRegex)
	}

	// Time range
	now := time.Now()
	start := now.Add(-duration)

	endpoint := fmt.Sprintf("%s/loki/api/v1/query_range", l.BaseURL)

	params := url.Values{}
	params.Set("query", query)
	params.Set("start", fmt.Sprintf("%d", start.UnixNano()))
	params.Set("end", fmt.Sprintf("%d", now.UnixNano()))
	params.Set("limit", "20")
	fullURL := endpoint + "?" + params.Encode()

	// HTTP request 
	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("Error creating request: %w", err)
	}

	resp, err := l.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Error calling Loki: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Loki status=%d body=%s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Error reading response: %w", err)
	}

	// Loki response format
	var result struct {
		Status string `json:"status"`
		Data   struct {
			Result []struct {
				Stream map[string]string `json:"stream"`
				Values [][]string        `json:"values"`
			} `json:"result"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("Error parsing JSON: %w", err)
	}
	if result.Status != "success" {
		return nil, fmt.Errorf("Loki response status=%s", result.Status)
	}

	// Collect log lines
	var logs []string
	for _, stream := range result.Data.Result {
		for _, value := range stream.Values {
			// value[0] = timestamp
			// value[1] = actual log line
			if len(value) >= 2 {
				logs = append(logs, value[1])
			}
		}
	}

	return logs, nil
}
