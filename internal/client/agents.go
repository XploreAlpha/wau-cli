package client

import (
	"context"
	"fmt"
	"strings"
)

// Health checks the kernel health.
func (c *Client) Health(ctx context.Context) (*HealthResponse, error) {
	var resp HealthResponse
	if err := c.Get(ctx, "/health", &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetKernelInfo returns kernel information.
func (c *Client) GetKernelInfo(ctx context.Context) (*KernelInfo, error) {
	var resp KernelInfo
	if err := c.Get(ctx, "/kernel/info", &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ListAgents returns paginated list of online agents.
//
// Tolerant decoder(per 2026-08-20 visa demo):
//   - server may return either {"agents":[...], "total":N} OR raw [...]
//   - 先试 object 格式,失败再试 array 格式
func (c *Client) ListAgents(ctx context.Context, page, pageSize int, skill, status, search string) (*AgentListResponse, error) {
	path := fmt.Sprintf("/registry/agents?page=%d&pageSize=%d", page, pageSize)
	if skill != "" {
		path += "&skill=" + skill
	}
	if status != "" {
		path += "&status=" + status
	}
	if search != "" {
		path += "&search=" + search
	}
	var resp AgentListResponse
	if err := c.Get(ctx, path, &resp); err != nil {
		// object decode 失败 → 重试 array 格式(无 retry,只 1 次试探)
		if isArrayDecodeErr(err) {
			var arr []Agent
			if err2 := c.GetWithOpts(ctx, path, &arr, RequestOpts{MaxRetries: 0}); err2 != nil {
				return nil, err2
			}
			resp.Agents = arr
			resp.Total = int64(len(arr))
			return &resp, nil
		}
		return nil, err
	}
	return &resp, nil
}

// isArrayDecodeErr 判断 err 是否为 "cannot unmarshal array into struct" 类型。
func isArrayDecodeErr(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "cannot unmarshal array")
}

// GetAgent returns agent details by name.
func (c *Client) GetAgent(ctx context.Context, name string) (*AgentStatus, error) {
	var resp AgentStatus
	if err := c.Get(ctx, fmt.Sprintf("/registry/agents/%s/status", name), &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetAgentScore returns agent's score.
func (c *Client) GetAgentScore(ctx context.Context, name string) (*AgentScore, error) {
	var resp AgentScore
	if err := c.Get(ctx, fmt.Sprintf("/registry/agents/%s/score", name), &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// RegisterAgent registers a new agent.
func (c *Client) RegisterAgent(ctx context.Context, req *AgentRegisterRequest) error {
	return c.Post(ctx, "/registry/agents/register", req, nil)
}

// DeregisterAgent removes an agent by name.
func (c *Client) DeregisterAgent(ctx context.Context, name string) error {
	return c.Delete(ctx, fmt.Sprintf("/registry/agents/%s", name), nil)
}
