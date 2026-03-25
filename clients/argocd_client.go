package clients

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type ArgoCDClient struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

func NewArgoCDClient(baseURL, token string) *ArgoCDClient {
	return &ArgoCDClient{
		BaseURL: baseURL,
		Token:   token,
		HTTPClient: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
			Timeout: 15 * time.Second,
		},
	}
}

// RollbackApp — main function
// Step 1: Auto-sync disable
// Step 2: Previous version pe rollback
func (a *ArgoCDClient) RollbackApp(ctx context.Context, appName string) error {

	// Step 1: Auto-sync disable karo
	// Warna ArgoCD rollback allow nahi karega
	if err := a.disableAutoSync(ctx, appName); err != nil {
		return fmt.Errorf("auto-sync disable failed: %w", err)
	}

	// Thoda wait karo — ArgoCD ko process karne do
	time.Sleep(2 * time.Second)

	// Step 2: Previous revision id nikaalo
	historyID, err := a.getPreviousHistoryID(ctx, appName)
	if err != nil {
		return fmt.Errorf("rollback history error: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/applications/%s/rollback", a.BaseURL, appName)
	payload := map[string]interface{}{"id": historyID}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("request error: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+a.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("argocd call failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("rollback failed: status=%d body=%s",
			resp.StatusCode, string(respBody))
	}

	return nil
}

func (a *ArgoCDClient) getPreviousHistoryID(ctx context.Context, appName string) (int, error) {
	url := fmt.Sprintf("%s/api/v1/applications/%s", a.BaseURL, appName)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+a.Token)

	resp, err := a.HTTPClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("history fetch failed: status=%d body=%s", resp.StatusCode, string(b))
	}

	var result struct {
		Status struct {
			History []struct {
				ID int `json:"id"`
			} `json:"history"`
		} `json:"status"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	if len(result.Status.History) < 2 {
		return 0, fmt.Errorf("not enough history to rollback")
	}

	// Last entry is current, previous is second last.
	return result.Status.History[len(result.Status.History)-2].ID, nil
}

// disableAutoSync — temporarily auto-sync band karo
// Developer fix karke manually re-enable karega
func (a *ArgoCDClient) disableAutoSync(ctx context.Context, appName string) error {
	// ArgoCD patch API expects ApplicationPatchRequest with patch + patchType
	patchURL := fmt.Sprintf("%s/api/v1/applications/%s", a.BaseURL, appName)
	patchBody := map[string]interface{}{
		"spec": map[string]interface{}{
			"syncPolicy": nil,
		},
	}
	patchJSON, _ := json.Marshal(patchBody)
	payload := map[string]interface{}{
		"patch":     string(patchJSON),
		"patchType": "merge",
	}
	body, _ := json.Marshal(payload)

	patchReq, err := http.NewRequestWithContext(ctx, "PATCH", patchURL, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	patchReq.Header.Set("Authorization", "Bearer "+a.Token)
	patchReq.Header.Set("Content-Type", "application/json")

	patchResp, err := a.HTTPClient.Do(patchReq)
	if err != nil {
		return err
	}
	defer patchResp.Body.Close()

	if patchResp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(patchResp.Body)
		return fmt.Errorf("disable auto-sync failed: status=%d body=%s",
			patchResp.StatusCode, string(b))
	}

	return nil
}

// GetAppStatus — rollback ke baad health check
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