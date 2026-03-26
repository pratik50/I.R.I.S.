package clients

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// SlackClient handles Slack notifications
type SlackClient struct {
	WebhookURL string
	ChannelID  string
	BotToken   string
	HTTPClient *http.Client
	Enabled    bool
}

// SlackMessage represents a Slack message payload
type SlackMessage struct {
	Channel   string       `json:"channel,omitempty"`
	Text      string       `json:"text,omitempty"`
	Blocks    []SlackBlock `json:"blocks,omitempty"`
	Username  string       `json:"username,omitempty"`
	IconEmoji string       `json:"icon_emoji,omitempty"`
}

// SlackBlock represents a block in Slack message
type SlackBlock struct {
	Type string                 `json:"type"`
	Text *SlackText             `json:"text,omitempty"`
	Fields []*SlackText         `json:"fields,omitempty"`
	Elements []interface{}      `json:"elements,omitempty"`
}

// SlackText represents text in Slack blocks
type SlackText struct {
	Type string `json:"type"`
	Text string `json:"text"`
	Emoji bool   `json:"emoji,omitempty"`
}

// NewSlackClient creates a new Slack client
func NewSlackClient(botToken, channelID string, enabled bool) *SlackClient {
	return &SlackClient{
		BotToken:   botToken,
		ChannelID:  channelID,
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
		Enabled:    enabled,
	}
}

// SendMessage sends a simple text message to Slack
func (s *SlackClient) SendMessage(text string) error {
	if !s.Enabled {
		return nil
	}

	msg := SlackMessage{
		Channel:   s.ChannelID,
		Text:      text,
		Username:  "I.R.I.S.",
		IconEmoji: ":robot_face:",
	}

	return s.send(msg)
}

// SendRichMessage sends a rich formatted message with blocks
func (s *SlackClient) SendRichMessage(title, text, color string, fields map[string]string) error {
	if !s.Enabled {
		return nil
	}

	blocks := []SlackBlock{
		{
			Type: "header",
			Text: &SlackText{
				Type: "plain_text",
				Text: title,
			},
		},
		{
			Type: "divider",
		},
		{
			Type: "section",
			Text: &SlackText{
				Type: "mrkdwn",
				Text: text,
			},
		},
	}

	// Add fields if any
	if len(fields) > 0 {
		var fieldBlocks []*SlackText
		for k, v := range fields {
			fieldBlocks = append(fieldBlocks, &SlackText{
				Type: "mrkdwn",
				Text: fmt.Sprintf("*%s:*\n%s", k, v),
			})
		}
		
		blocks = append(blocks, SlackBlock{
			Type:   "section",
			Fields: fieldBlocks,
		})
	}

	// Add color indicator using context block
	if color != "" {
		blocks = append(blocks, SlackBlock{
			Type: "context",
			Elements: []interface{}{
				map[string]string{
					"type": "mrkdwn",
					"text": color,
				},
			},
		})
	}

	msg := SlackMessage{
		Channel:   s.ChannelID,
		Blocks:    blocks,
		Username:  "I.R.I.S.",
		IconEmoji: ":robot_face:",
	}

	return s.send(msg)
}

// SendRollbackSuccess sends notification for successful rollback
func (s *SlackClient) SendRollbackSuccess(deployment, namespace, rootCause string, riskScore float64) error {
	if !s.Enabled {
		return nil
	}

	title := "✅ Auto-Rollback Successful"
	text := fmt.Sprintf("Deployment `%s/%s` has been automatically rolled back.", namespace, deployment)
	fields := map[string]string{
		"Risk Score": fmt.Sprintf("%.2f (High)", riskScore),
		"Root Cause": rootCause,
		"Action":     "Rolled back to previous stable version",
	}

	return s.SendRichMessage(title, text, ":large_green_circle:", fields)
}

// SendRollbackFailure sends notification for failed rollback
func (s *SlackClient) SendRollbackFailure(deployment, namespace, errMsg string) error {
	if !s.Enabled {
		return nil
	}

	title := "❌ Rollback Failed"
	text := fmt.Sprintf("Deployment `%s/%s` failed, but automatic rollback encountered an error.", namespace, deployment)
	fields := map[string]string{
		"Error": errMsg,
		"Action Required": "Manual intervention needed. Check ArgoCD logs and controller logs.",
	}

	return s.SendRichMessage(title, text, ":large_red_circle:", fields)
}

// SendManualDiagnosisRequired sends alert for manual diagnosis
func (s *SlackClient) SendManualDiagnosisRequired(deployment, namespace, rootCause, suggestion string, riskScore float64) error {
	if !s.Enabled {
		return nil
	}

	title := "⚠️ Manual Diagnosis Required"
	text := fmt.Sprintf("Deployment `%s/%s` has issues, but auto-rollback was not triggered.", namespace, deployment)
	fields := map[string]string{
		"Risk Score": fmt.Sprintf("%.2f (Low - Auto-rollback skipped)", riskScore),
		"Root Cause": rootCause,
		"Suggestion": suggestion,
		"Next Steps": "Manual investigation required. Check logs and metrics for details.",
	}

	return s.SendRichMessage(title, text, ":large_yellow_circle:", fields)
}

// SendCrashLoopBackOffAlert sends alert for CrashLoopBackOff detection
func (s *SlackClient) SendCrashLoopBackOffAlert(deployment, namespace string, restartCount int) error {
	if !s.Enabled {
		return nil
	}

	title := "🔴 CrashLoopBackOff Detected"
	text := fmt.Sprintf("Deployment `%s/%s` is in CrashLoopBackOff state with %d restarts.", namespace, deployment, restartCount)
	fields := map[string]string{
		"Restart Count": fmt.Sprintf("%d", restartCount),
		"Action Taken":  "Force rollback triggered immediately",
		"Status":        "Rollback in progress...",
	}

	return s.SendRichMessage(title, text, ":red_circle:", fields)
}

// SendCrashDetected sends alert when a crash is first detected
func (s *SlackClient) SendCrashDetected(deployment, namespace string, restartCount int) error {
	if !s.Enabled {
		return nil
	}

	title := "💥 Crash Detected"
	text := fmt.Sprintf("Deployment `%s/%s` has experienced a container crash.", namespace, deployment)
	fields := map[string]string{
		"Restart Count": fmt.Sprintf("%d", restartCount),
		"Status":        "Analyzing...",
		"Next Step":     "Checking if this is CrashLoopBackOff or first-time failure",
	}

	return s.SendRichMessage(title, text, ":large_yellow_circle:", fields)
}

// SendAIAnalyzing sends alert when AI analysis begins
func (s *SlackClient) SendAIAnalyzing(deployment, namespace string) error {
	if !s.Enabled {
		return nil
	}

	title := "🤖 AI Analysis Started"
	text := fmt.Sprintf("Deployment `%s/%s` failure is being analyzed by AI.", namespace, deployment)
	fields := map[string]string{
		"Status":   "Analyzing metrics, logs, and events",
		"Model":    "Llama 3.1 (8B)",
		"Duration": "~2-3 seconds",
	}

	return s.SendRichMessage(title, text, ":robot_face:", fields)
}

// SendAIDecision sends AI analysis result
func (s *SlackClient) SendAIDecision(deployment, namespace, rootCause, suggestion string, riskScore float64, action string) error {
	if !s.Enabled {
		return nil
	}

	var title, color string
	if action == "rollback" {
		title = "🎯 AI Decision: Rollback Required"
		color = ":large_red_circle:"
	} else {
		title = "👀 AI Decision: Monitor Only"
		color = ":large_yellow_circle:"
	}

	text := fmt.Sprintf("AI has analyzed the failure for `%s/%s`.", namespace, deployment)
	fields := map[string]string{
		"Risk Score":  fmt.Sprintf("%.2f", riskScore),
		"Root Cause":  rootCause,
		"Action":      action,
		"Suggestion":  suggestion,
	}

	return s.SendRichMessage(title, text, color, fields)
}

// send is the internal method to actually send the message
func (s *SlackClient) send(msg SlackMessage) error {
	url := "https://slack.com/api/chat.postMessage"
	
	// Add bot token to message
	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal slack message: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to create slack request: %w", err)
	}
	
	req.Header.Set("Authorization", "Bearer "+s.BotToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send slack message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack API returned status %d", resp.StatusCode)
	}

	return nil
}