package clients

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

// AI response
type AIAnalysis struct {
	RootCause  string  `json:"root_cause"`
	RiskScore  float64 `json:"risk_score"`
	Action     string  `json:"action"` 		// "rollback" or "alert"
	Suggestion string  `json:"suggestion"`
}

// AIClient for Groq API
type AIClient struct {
	APIKey     string
	HTTPClient *http.Client
}

// NewAIClient 
func NewAIClient(apiKey string) *AIClient {
	return &AIClient{
		APIKey: apiKey,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// AiAnalyze  
func (a *AIClient) Analyze(
	ctx context.Context,
	deploymentName string,
	namespace string,
	metrics *MetricsSummary,
	logs []string,
	events []string,
) (*AIAnalysis, error) {

	// Bind logs to string
	logsText := "No logs available"
	if len(logs) > 0 {
		logsText = strings.Join(logs, "\n")
	}

	// Bind events to string
	eventsText := "No events available"
	if len(events) > 0 {
		eventsText = strings.Join(events, "\n")
	}

	// User prompt
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

Kubernetes Events:
%s

Respond ONLY with this JSON format, no other text:
{
  "root_cause": "brief description of what caused the failure",
  "risk_score": 0.0 to 1.0,
  "action": "rollback" or "monitor" or "alert",
  "suggestion": "what developer should fix"
}

Rules:
- risk_score >= 0.5 = critical, needs rollback
- risk_score < 0.5 = minor, needs manual diagnosis
- Container crash or CrashLoopBackOff → risk_score = 0.8
- Memory limit exceeded → risk_score = 0.7
- Network issues → risk_score = 0.6
- Configuration error → risk_score = 0.5
- Minor warnings → risk_score = 0.3
- No issues found → risk_score = 0.1

ACTION field:
- If risk_score >= 0.5 → action = "rollback"
- If risk_score < 0.5 → action = "alert"
`,
		deploymentName,
		namespace,
		metrics.AvailableReplicas,
		metrics.CPUUsage,
		metrics.MemoryMB(),
		logsText,
		eventsText,
	)

	// Groq API request body
	requestBody := map[string]interface{}{
		"model": "llama-3.1-8b-instant", 
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
		"temperature": 0.1, 
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

	// Parse Groq response
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

	aiText := groqResp.Choices[0].Message.Content

	// JSON parse 
	var analysis AIAnalysis
	if err := json.Unmarshal([]byte(aiText), &analysis); err != nil {
		// If JSON parse fail, return this default response
		return &AIAnalysis{
			RootCause:  aiText,
			RiskScore:  0.5,
			Action:     "alert",
			Suggestion: "Manual investigation required",
		}, nil
	}

	return &analysis, nil
}