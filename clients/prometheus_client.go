package clients

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

// MetricsSummary
type MetricsSummary struct {
	DeploymentName string
	Namespace      string
	CPUUsage       float64 
	MemoryBytes    float64 
	AvailableReplicas float64
	FetchedAt      time.Time
}

// PrometheusClient
type PrometheusClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewPrometheusClient
func NewPrometheusClient(baseURL string) *PrometheusClient {
	return &PrometheusClient{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// QueryInstant (Prometheus query)
func (p *PrometheusClient) QueryInstant(ctx context.Context, query string) (float64, error) {

	endpoint := fmt.Sprintf("%s/api/v1/query", p.BaseURL)

	// Query safe URL 
	// "kube deployment{name}" → "kube+deployment%7Bname%7D" (most important took a lot of time to figure out)	
	params := url.Values{}
	params.Set("query", query)
	fullURL := endpoint + "?" + params.Encode()

	// HTTP GET request
	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return 0, fmt.Errorf("Error creating request: %w", err)
	}

	resp, err := p.HTTPClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("prometheus call failed: %w", err)
	}
	defer resp.Body.Close() 

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("prometheus status=%d body=%s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("Error reading response: %w", err)
	}

	// Prometheus response format and parse
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
	if result.Status != "success" {
		return 0, fmt.Errorf("prometheus response status=%s", result.Status)
	}

	if len(result.Data.Result) == 0 {
		return 0, nil 
	}

	var total float64
	for _, item := range result.Data.Result {
		valueStr, ok := item.Value[1].(string)
		if !ok {
			return 0, fmt.Errorf("Value string not found")
		}
		value, err := strconv.ParseFloat(valueStr, 64)
		if err != nil {
			return 0, fmt.Errorf("float convert error: %w", err)
		}
		total += value
	}

	return total, nil
}

// FetchDeploymentMetrics
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
	replicasQuery := fmt.Sprintf(
		`kube_deployment_status_replicas_available{deployment="%s",namespace="%s"}`,
		deploymentName, namespace,
	)
	replicas, err := p.QueryInstant(ctx, replicasQuery)
	if err != nil {
		return nil, fmt.Errorf("replicas query failed: %w", err)
	}
	summary.AvailableReplicas = replicas

	
	// Query 2: CPU (last 5 min rate)
	cpuQuery := fmt.Sprintf(
		`sum(rate(container_cpu_usage_seconds_total{pod=~"%s-.*",namespace="%s",container!=""}[5m]))`,
		deploymentName, namespace,
	)
	cpu, err := p.QueryInstant(ctx, cpuQuery)
	if err != nil {
		cpu = 0
	}
	summary.CPUUsage = cpu

	// Query 3: Memory (last 5 min peak)
	memQuery := fmt.Sprintf(
		`sum(max_over_time(container_memory_working_set_bytes{pod=~"%s-.*",namespace="%s",container!=""}[5m]))`,
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