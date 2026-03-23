package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
	"crypto/tls"
)

// ArgoCDClient — ArgoCD se baat karne ka tool
type ArgoCDClient struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

// NewArgoCDClient — naya client banao
func NewArgoCDClient(baseURL, token string) *ArgoCDClient {
	return &ArgoCDClient{
		BaseURL: baseURL,
		Token:   token,
		HTTPClient: &http.Client{
			// TLS skip karo — local development mein
			// self-signed certificate hai ArgoCD ka
			Transport: &http.Transport{
				TLSClientConfig: tlsSkipVerify(),
			},
			Timeout: 15 * time.Second,
		},
	}
}

// RollbackApp — app ko previous version pe rollback karo
func (a *ArgoCDClient) RollbackApp(ctx context.Context, appName string) error {

	// ArgoCD REST API endpoint
	// POST /api/v1/applications/{name}/rollback
	url := fmt.Sprintf("%s/api/v1/applications/%s/rollback", a.BaseURL, appName)

	// Request body — id: 0 matlab previous revision
	payload := map[string]interface{}{
		"id": 0,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("payload marshal failed: %w", err)
	}

	// HTTP POST request banao
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("request banane mein error: %w", err)
	}

	// Auth token + Content-Type headers
	req.Header.Set("Authorization", "Bearer "+a.Token)
	req.Header.Set("Content-Type", "application/json")

	// Request bhejo
	resp, err := a.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("argocd call failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	// Success check karo
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("rollback failed: status=%d body=%s",
			resp.StatusCode, string(respBody))
	}

	return nil
}

// GetAppStatus — app ki current status laao
func (a *ArgoCDClient) GetAppStatus(ctx context.Context, appName string) (string, error) {

	url := fmt.Sprintf("%s/api/v1/applications/%s", a.BaseURL, appName)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+a.Token)

	resp, err := a.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		Status struct {
			Health struct {
				Status string `json:"status"`
			} `json:"health"`
		} `json:"status"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.Status.Health.Status, nil
}

func tlsSkipVerify() *tls.Config {
	return &tls.Config{InsecureSkipVerify: true}
}