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

// LokiClient — Loki se baat karne ka tool
type LokiClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewLokiClient — naya client banao
func NewLokiClient(baseURL string) *LokiClient {
	return &LokiClient{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// FetchErrorLogs — deployment ke last X duration ke ERROR logs laao
func (l *LokiClient) FetchErrorLogs(
	ctx context.Context,
	deploymentName string,
	namespace string,
	duration time.Duration, // kitne time pehle tak ke logs chahiye
) ([]string, error) {

	return l.fetchLogs(ctx, deploymentName, namespace, duration, "(?i)error|fatal|panic")
}

// FetchRecentLogs — deployment ke recent logs bina error filter ke
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

	// LogQL query banao
	// Matlab: "is deployment ke ERROR logs do"
	query := fmt.Sprintf(
		`{namespace="%s",pod=~"%s-.*"}`,
		namespace,
		deploymentName,
	)
	if filterRegex != "" {
		query = fmt.Sprintf(`%s |~ "%s"`, query, filterRegex)
	}

	// Time range set karo
	now := time.Now()
	start := now.Add(-duration) // duration pehle se

	// URL banao
	endpoint := fmt.Sprintf("%s/loki/api/v1/query_range", l.BaseURL)

	params := url.Values{}
	params.Set("query", query)
	params.Set("start", fmt.Sprintf("%d", start.UnixNano()))
	params.Set("end", fmt.Sprintf("%d", now.UnixNano()))
	params.Set("limit", "20") // max 20 log lines
	fullURL := endpoint + "?" + params.Encode()

	// HTTP request bhejo
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


	// Loki ka response format:
	// {
	//   "status": "success",
	//   "data": {
	//     "result": [
	//       {
	//         "stream": {"app": "crash-test"},
	//         "values": [
	//           ["timestamp", "ERROR: out of memory"],
	//           ["timestamp", "FATAL: crash at line 42"]
	//         ]
	//       }
	//     ]
	//   }
	// }
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

	// Sab log lines ek slice mein collect karo
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
