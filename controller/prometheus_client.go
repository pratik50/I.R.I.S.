package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// MetricsSummary — ye struct hold karega
// sab metrics ek jagah
type MetricsSummary struct {
	DeploymentName string
	Namespace      string
	CPUUsage       float64 // cores mein
	MemoryBytes    float64 // bytes mein
	AvailableReplicas float64
	FetchedAt      time.Time
}

// PrometheusClient — Prometheus se baat karne ka tool
type PrometheusClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewPrometheusClient — naya client banao
func NewPrometheusClient(baseURL string) *PrometheusClient {
	return &PrometheusClient{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second, // 10 sec mein jawab nahi = timeout
		},
	}
}

// QueryInstant — Prometheus se ek query karo
// result float64 mein milega
func (p *PrometheusClient) QueryInstant(ctx context.Context, query string) (float64, error) {

	// URL banao
	// Example: http://prometheus:9090/api/v1/query?query=up
	endpoint := fmt.Sprintf("%s/api/v1/query", p.BaseURL)

	// Query ko URL safe banao
	// "kube deployment{name}" → "kube+deployment%7Bname%7D"
	params := url.Values{}
	params.Set("query", query)
	fullURL := endpoint + "?" + params.Encode()

	// HTTP GET request banao
	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return 0, fmt.Errorf("request banane mein error: %w", err)
	}

	// Request bhejo
	resp, err := p.HTTPClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("prometheus call failed: %w", err)
	}
	defer resp.Body.Close() // response band karo baad mein

	// Response body padhो
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("response padhne mein error: %w", err)
	}

	// JSON parse karo
	// Prometheus ka response format:
	// {
	//   "status": "success",
	//   "data": {
	//     "result": [
	//       {
	//         "value": [timestamp, "1.234"]
	//       }
	//     ]
	//   }
	// }
	var result struct {
		Status string `json:"status"`
		Data   struct {
			Result []struct {
				Value []interface{} `json:"value"`
			} `json:"result"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return 0, fmt.Errorf("JSON parse error: %w", err)
	}

	// Koi result nahi aaya?
	if len(result.Data.Result) == 0 {
		return 0, nil // 0 return karo — metric nahi mili
	}

	// Value nikalo — string hai, float mein convert karo
	valueStr, ok := result.Data.Result[0].Value[1].(string)
	if !ok {
		return 0, fmt.Errorf("value string nahi hai")
	}

	value, err := strconv.ParseFloat(valueStr, 64)
	if err != nil {
		return 0, fmt.Errorf("float convert error: %w", err)
	}

	return value, nil
}

// FetchDeploymentMetrics — deployment ki sari metrics ek saath laao
func (p *PrometheusClient) FetchDeploymentMetrics(
	ctx context.Context,
	deploymentName string,
	namespace string,
) (*MetricsSummary, error) {

	summary := &MetricsSummary{
		DeploymentName: deploymentName,
		Namespace:      namespace,
		FetchedAt:      time.Now(),
	}

	// Query 1: Available Replicas
	// "crash-test namespace=default mein kitne pods available hain?"
	replicasQuery := fmt.Sprintf(
		`kube_deployment_status_replicas_available{deployment="%s",namespace="%s"}`,
		deploymentName, namespace,
	)
	replicas, err := p.QueryInstant(ctx, replicasQuery)
	if err != nil {
		return nil, fmt.Errorf("replicas query failed: %w", err)
	}
	summary.AvailableReplicas = replicas

	
	// ✅ CHANGED: Query 2: CPU — last 5 min average
	// Pod crash ho bhi toh historical data milega
	cpuQuery := fmt.Sprintf(
		`avg_over_time(container_cpu_usage_seconds_total{pod=~"%s.*",namespace="%s",container!=""}[5m])`,
		deploymentName, namespace,
	)
	cpu, err := p.QueryInstant(ctx, cpuQuery)
	if err != nil {
		cpu = 0
	}
	summary.CPUUsage = cpu

	// ✅ CHANGED: Query 3: Memory — last 5 min ka peak
	// Crash se pehle max memory kya thi?
	memQuery := fmt.Sprintf(
		`max_over_time(container_memory_working_set_bytes{pod=~"%s.*",namespace="%s",container!=""}[5m])`,
		deploymentName, namespace,
	)
	mem, err := p.QueryInstant(ctx, memQuery)
	if err != nil {
		mem = 0
	}
	summary.MemoryBytes = mem

	return summary, nil
}

func (m *MetricsSummary) MemoryMB() float64 {
	return m.MemoryBytes / 1024 / 1024
}