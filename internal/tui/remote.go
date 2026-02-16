// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package tui

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"grimm.is/flywall/internal/alerting"
	"grimm.is/flywall/internal/config"
	"grimm.is/flywall/internal/ctlplane"
)

// RemoteBackend implements Backend using the HTTP API
type RemoteBackend struct {
	BaseURL string
	Client  *http.Client
	APIKey  string
}

// NewRemoteBackend creates a new remote backend
func NewRemoteBackend(baseURL, apiKey string, insecure bool) *RemoteBackend {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: insecure},
	}

	return &RemoteBackend{
		BaseURL: baseURL,
		APIKey:  apiKey,
		Client: &http.Client{
			Timeout:   10 * time.Second,
			Transport: transport,
		},
	}
}

func (b *RemoteBackend) do(method, path string) (*http.Response, error) {
	url := b.BaseURL + path
	DebugLog("REQ %s %s", method, url)
	start := time.Now()

	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		DebugLog("ERR init %s: %v", url, err)
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+b.APIKey)
	req.Header.Set("X-API-Key", b.APIKey) // Support both standard and custom header
	req.Header.Set("Accept", "application/json")

	resp, err := b.Client.Do(req)
	duration := time.Since(start)

	if err != nil {
		DebugLog("ERR %s %s (%s): %v", method, url, duration, err)
		return nil, err
	}

	DebugLog("RES %s %s %d (%s)", method, url, resp.StatusCode, duration)
	return resp, nil
}

func (b *RemoteBackend) GetStatus() (*EnrichedStatus, error) {
	resp, err := b.do("GET", "/api/status")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("api error: %s", resp.Status)
	}

	// API returns specific status JSON, we map it to EnrichedStatus
	// For now, let's assume we decode a map and extract
	var data map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	status := &EnrichedStatus{
		Running: true,
		Uptime:  "Online", // TODO: Parse duration
	}

	if val, ok := data["uptime"].(string); ok {
		status.Uptime = val
	}

	return status, nil
}

// apiFlow matches the JSON response from /api/flows
type apiFlow struct {
	ID    int64  `json:"id"`
	Proto string `json:"proto"`
	Src   string `json:"src"`
	Dst   string `json:"dst"`
	State string `json:"state"`
}

func (b *RemoteBackend) GetFlows(filter string) ([]Flow, error) {
	resp, err := b.do("GET", "/api/flows")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("api error: %s", resp.Status)
	}

	var data struct {
		Flows []apiFlow `json:"flows"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	flows := make([]Flow, len(data.Flows))
	for i, f := range data.Flows {
		flows[i] = Flow{
			ID:    f.ID,
			Proto: f.Proto,
			Src:   f.Src,
			Dst:   f.Dst,
			State: f.State,
		}
	}

	return flows, nil
}

func (b *RemoteBackend) ApproveFlow(id int64) error {
	data := map[string]int64{"id": id}
	body, _ := json.Marshal(data)

	url := b.BaseURL + "/api/flows/approve"
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+b.APIKey)
	req.Header.Set("X-API-Key", b.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := b.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to approve flow: %s", resp.Status)
	}
	return nil
}

func (b *RemoteBackend) DenyFlow(id int64) error {
	data := map[string]int64{"id": id}
	body, _ := json.Marshal(data)

	url := b.BaseURL + "/api/flows/deny"
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+b.APIKey)
	req.Header.Set("X-API-Key", b.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := b.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to deny flow: %s", resp.Status)
	}
	return nil
}

func (b *RemoteBackend) GetConfig() (*config.Config, error) {
	resp, err := b.do("GET", "/api/config")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("api error: %s", resp.Status)
	}

	var cfg config.Config
	if err := json.NewDecoder(resp.Body).Decode(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (b *RemoteBackend) GetSystemStats() (*ctlplane.SystemStats, error) {
	resp, err := b.do("GET", "/api/system/stats")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("api error: %s", resp.Status)
	}

	var data struct {
		Stats ctlplane.SystemStats `json:"stats"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	return &data.Stats, nil
}

func (b *RemoteBackend) ApplyConfig(cfg *config.Config) error {
	// 1. Push config to staging
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}

	url := b.BaseURL + "/api/config"
	req, err := http.NewRequest("POST", url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+b.APIKey)
	req.Header.Set("X-API-Key", b.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := b.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to stage config: %s", resp.Status)
	}

	// 2. Commit (Apply) staged config
	resp2, err := b.do("POST", "/api/config/apply")
	if err != nil {
		return err
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to apply config: %s", resp2.Status)
	}

	return nil
}

func (b *RemoteBackend) ReloadConfig() error {
	// Fallback to restarting firewall service for now as generic reload
	return b.RestartService("firewall")
}

func (b *RemoteBackend) ListBackups() ([]ctlplane.BackupInfo, error) {
	resp, err := b.do("GET", "/api/backups")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("api error: %s", resp.Status)
	}

	var data struct {
		Backups []ctlplane.BackupInfo `json:"backups"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	return data.Backups, nil
}

func (b *RemoteBackend) RestoreBackup(version int) error {
	data := map[string]int{"version": version}
	body, _ := json.Marshal(data)

	url := b.BaseURL + "/api/backups/restore"
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+b.APIKey)
	req.Header.Set("X-API-Key", b.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := b.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to restore backup: %s", resp.Status)
	}

	return nil
}

func (b *RemoteBackend) Reboot() error {
	resp, err := b.do("POST", "/api/system/reboot")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to reboot: %s", resp.Status)
	}

	return nil
}

func (b *RemoteBackend) RestartService(name string) error {
	data := map[string]string{"service": name}
	body, _ := json.Marshal(data)

	url := b.BaseURL + "/api/system/services/restart"
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+b.APIKey)
	req.Header.Set("X-API-Key", b.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := b.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to restart service: %s", resp.Status)
	}
	return nil
}

func (b *RemoteBackend) GetServices() ([]ctlplane.ServiceStatus, error) {
	resp, err := b.do("GET", "/api/services")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("api error: %s", resp.Status)
	}

	var data struct {
		Services []ctlplane.ServiceStatus `json:"services"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	return data.Services, nil
}

func (b *RemoteBackend) GetBandwidth(window string) ([]ctlplane.BandwidthPoint, error) {
	// Calculate From/To based on window, but actually the API might handle it?
	// The API handler likely expects query params.
	// For now let's construct defaults similar to adapter.
	to := time.Now()
	from := to.Add(-1 * time.Hour)
	if window == "24h" {
		from = to.Add(-24 * time.Hour)
	}

	// Format as RFC3339
	q := fmt.Sprintf("?from=%s&to=%s", from.Format(time.RFC3339), to.Format(time.RFC3339))
	resp, err := b.do("GET", "/api/analytics/bandwidth"+q)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("api error: %s", resp.Status)
	}

	var data struct {
		Points []ctlplane.BandwidthPoint `json:"points"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	return data.Points, nil
}

func (b *RemoteBackend) GetAlerts(limit int) ([]alerting.AlertEvent, error) {
	q := fmt.Sprintf("?limit=%d", limit)
	resp, err := b.do("GET", "/api/alerts/history"+q)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("api error: %s", resp.Status)
	}

	var data struct {
		Events []alerting.AlertEvent `json:"events"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	return data.Events, nil
}
