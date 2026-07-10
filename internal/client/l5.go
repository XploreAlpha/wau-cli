// Package client - l5.go
//
// v1.0.0 M11 P4.5 ⭐L5 包管理器 HTTP client(per D72/D73/D74,2026-07-10)。
//
// 对应 WAU-core-kernel /v1/l5/* 5 端点:
//   - POST /v1/l5/install       — 装 agent
//   - POST /v1/l5/uninstall     — 卸 agent
//   - POST /v1/l5/update        — 更新 agent
//   - POST /v1/l5/search        — 搜 wau-registry
//   - POST /v1/l5/login         — 登入
package client

import "context"

// L5InstallRequest 装 agent 请求
type L5InstallRequest struct {
	UserID    string            `json:"user_id"`
	AgentName string            `json:"agent_name"`
	Version   string            `json:"version,omitempty"`
	Purge     bool              `json:"purge,omitempty"`
	Config    map[string]string `json:"config,omitempty"`
}

// L5InstallResponse 装 agent 响应
type L5InstallResponse struct {
	OK              bool    `json:"ok"`
	AgentID         string  `json:"agent_id,omitempty"`
	Version         string  `json:"version,omitempty"`
	InstalledAt     int64   `json:"installed_at,omitempty"`
	DurationMS      float64 `json:"duration_ms,omitempty"`
	SandboxDockerID string  `json:"sandbox_docker_id,omitempty"`
	Error           string  `json:"error,omitempty"`
}

// L5Install 装 agent(POST /v1/l5/install)
func (c *Client) L5Install(ctx context.Context, req *L5InstallRequest) (*L5InstallResponse, error) {
	var resp L5InstallResponse
	if err := c.Post(ctx, "/v1/l5/install", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// L5UninstallRequest 卸 agent 请求
type L5UninstallRequest struct {
	UserID    string `json:"user_id"`
	AgentName string `json:"agent_name"`
	Purge     bool   `json:"purge,omitempty"`
}

// L5UninstallResponse 卸 agent 响应
type L5UninstallResponse struct {
	OK            bool   `json:"ok"`
	UninstalledAt int64  `json:"uninstalled_at,omitempty"`
	SnapshotPath  string `json:"snapshot_path,omitempty"`
	Error         string `json:"error,omitempty"`
}

// L5Uninstall 卸 agent(POST /v1/l5/uninstall)
func (c *Client) L5Uninstall(ctx context.Context, req *L5UninstallRequest) (*L5UninstallResponse, error) {
	var resp L5UninstallResponse
	if err := c.Post(ctx, "/v1/l5/uninstall", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// L5UpdateRequest 更新 agent 请求
type L5UpdateRequest struct {
	UserID        string `json:"user_id"`
	AgentName     string `json:"agent_name,omitempty"`
	TargetVersion string `json:"target_version,omitempty"`
}

// L5UpdateResponse 更新 agent 响应
type L5UpdateResponse struct {
	OK            bool     `json:"ok"`
	UpdatedCount  int      `json:"updated_count,omitempty"`
	UpdatedAgents []string `json:"updated_agents,omitempty"`
	Error         string   `json:"error,omitempty"`
}

// L5Update 更新 agent(POST /v1/l5/update)
func (c *Client) L5Update(ctx context.Context, req *L5UpdateRequest) (*L5UpdateResponse, error) {
	var resp L5UpdateResponse
	if err := c.Post(ctx, "/v1/l5/update", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// L5SearchRequest 搜 wau-registry 请求
type L5SearchRequest struct {
	UserID   string `json:"user_id"`
	Query    string `json:"query"`
	Universe string `json:"universe,omitempty"`
	Limit    int    `json:"limit,omitempty"`
}

// L5SearchResponse 搜响应
type L5SearchResponse struct {
	OK      bool          `json:"ok"`
	Results []L5SearchHit `json:"results,omitempty"`
	Total   int           `json:"total,omitempty"`
	Error   string        `json:"error,omitempty"`
}

// L5SearchHit 单条搜索结果
type L5SearchHit struct {
	Name        string  `json:"name"`
	Version     string  `json:"version"`
	Description string  `json:"description"`
	Author      string  `json:"author"`
	Universe    string  `json:"universe"`
	Homepage    string  `json:"homepage"`
	TrustScore  float64 `json:"trust_score"`
}

// L5Search 搜 wau-registry(POST /v1/l5/search)
func (c *Client) L5Search(ctx context.Context, req *L5SearchRequest) (*L5SearchResponse, error) {
	var resp L5SearchResponse
	if err := c.Post(ctx, "/v1/l5/search", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// L5LoginRequest 登入请求
type L5LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Endpoint string `json:"endpoint,omitempty"`
}

// L5LoginResponse 登入响应
type L5LoginResponse struct {
	OK           bool   `json:"ok"`
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresAt    int64  `json:"expires_at,omitempty"`
	UserID       string `json:"user_id,omitempty"`
	Error        string `json:"error,omitempty"`
}

// L5Login 登入(POST /v1/l5/login)
func (c *Client) L5Login(ctx context.Context, req *L5LoginRequest) (*L5LoginResponse, error) {
	var resp L5LoginResponse
	if err := c.Post(ctx, "/v1/l5/login", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}