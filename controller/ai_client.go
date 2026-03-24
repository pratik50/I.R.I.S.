package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// AIAnalysis — AI ka response
type AIAnalysis struct {
	RootCause  string  `json:"root_cause"`
	RiskScore  float64 `json:"risk_score"`
	Action     string  `json:"action"` // "rollback", "monitor", "alert"
	Suggestion string  `json:"suggestion"`
}

// AIClient — Groq API se baat karne ka tool
type AIClient struct {
	APIKey     string
	HTTPClient *http.Client
}

// NewAIClient — naya client banao
func NewAIClient(apiKey string) *AIClient {
	return &AIClient{
		APIKey: apiKey,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Analyze — metrics + logs bhejo, root cause lo
func (a *AIClient) Analyze(
	ctx context.Context,
	deploymentName string,
	namespace string,
	metrics *MetricsSummary,
	logs []string,
) (*AIAnalysis, error) {

	// Logs ko ek string mein join karo
	logsText := "No logs available"
	if len(logs) > 0 {
		logsText = strings.Join(logs, "\n")
	}

	// User prompt — AI ko sab data denge
	userPrompt := fmt.Sprintf(`
Kubernetes deployment failure detected. Analyze and respond in JSON only.

Deployment: %s
Namespace: %s

Metrics:
- Available Replicas: %.0f
- CPU Usage: %.4f cores
- Memory Usage: %.2f MB

Recent Error Logs:
%s

Respond ONLY with this JSON format, no other text:
{
  "root_cause": "brief description of what caused the failure",
  "risk_score": 0.0 to 1.0,
  "action": "rollback" or "monitor" or "alert",
  "suggestion": "what developer should fix"
}

Rules:
- risk_score > 0.8 = serious, needs rollback
- risk_score 0.5-0.8 = moderate, needs alert
- risk_score < 0.5 = minor, just monitor
`,
		deploymentName,
		namespace,
		metrics.AvailableReplicas,
		metrics.CPUUsage,
		metrics.MemoryMB(),
		logsText,
	)

	// Groq API request body
	requestBody := map[string]interface{}{
		"model": "llama-3.1-8b-instant", // Free + Fast
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": "You are an expert DevOps engineer and SRE. Analyze Kubernetes deployment failures and provide root cause analysis. Always respond with valid JSON only, no markdown, no extra text.",
			},
			{
				"role":    "user",
				"content": userPrompt,
			},
		},
		"temperature": 0.1, // Low = consistent responses
		"max_tokens":  500,
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("request marshal failed: %w", err)
	}

	// Groq API call
	req, err := http.NewRequestWithContext(
		ctx,
		"POST",
		"https://api.groq.com/openai/v1/chat/completions",
		bytes.NewBuffer(body),
	)
	if err != nil {
		return nil, fmt.Errorf("request create failed: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+a.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("groq api call failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("response read failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("groq api error: status=%d body=%s",
			resp.StatusCode, string(respBody))
	}

	// Groq response parse karo
	var groqResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(respBody, &groqResp); err != nil {
		return nil, fmt.Errorf("groq response parse failed: %w", err)
	}

	if len(groqResp.Choices) == 0 {
		return nil, fmt.Errorf("no response from groq")
	}

	// AI ka actual text response
	aiText := groqResp.Choices[0].Message.Content

	// JSON parse karo AI response se
	var analysis AIAnalysis
	if err := json.Unmarshal([]byte(aiText), &analysis); err != nil {
		// Agar JSON parse fail ho toh default response
		return &AIAnalysis{
			RootCause:  aiText,
			RiskScore:  0.5,
			Action:     "alert",
			Suggestion: "Manual investigation required",
		}, nil
	}

	return &analysis, nil
}