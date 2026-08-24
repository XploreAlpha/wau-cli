// Package client - cluster.go
//
// P4.6 (v1.0.1, 2026-08-24) — `wau cluster` 子命令组的 client 侧支持。
//
// 设计要点:
//   - **纯组合**:不新增 kernel 端点,组合已有的 /health + /kernel/info + /registry/agents
//   - **并发调 3 endpoint**:每个 +1 RTT 改进巨大;用 sync.WaitGroup + errgroup 模式
//   - **Partial OK**:任一 endpoint fail 不 abort(只标记字段 nil),除非全部 fail
//
// kernel 端 `/v1/cluster/*` 还没规划(kernel v0.5.1);后续 kernel v0.6+ 可加。
package client

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ClusterStatus 集群状态汇总(P4.6 new)。
//
// 字段可能为 nil(对应 endpoint fail),partial fetch 时用 nil 区分。
type ClusterStatus struct {
	Endpoint    string         // 目标 server URL(debug 用)
	Health      *HealthResponse // /health response
	Kernel      *KernelInfo    // /kernel/info response
	AgentsTotal int            // /registry/agents 总数(从 len(raw array) 推)
	Modules     []string       // 从 KernelInfo.Modules 提取(便利字段)
	FetchedAt   time.Time      // 拉取时间戳

	// HealthErr / KernelErr / AgentsErr 任一非 nil 表示对应 endpoint fail(partial)
	HealthErr error
	KernelErr error
	AgentsErr error
}

// GetClusterStatus 并发调 3 个 endpoint 拿 cluster 状态。
//
// 返回:
//   - nil err + ClusterStatus:全部成功
//   - non-nil err:全部 endpoint 都 fail(连不上 / 5xx)
//   - status != nil + err != nil(partial):至少 1 个 endpoint OK,其它失败
//
// 实际用法:调用方判断 status != nil 即认为有数据;再单独检查 HealthErr/KernelErr/AgentsErr。
func (c *Client) GetClusterStatus(ctx context.Context) (*ClusterStatus, error) {
	status := &ClusterStatus{
		Endpoint:  c.baseURL,
		FetchedAt: time.Now(),
	}

	var wg sync.WaitGroup
	var mu sync.Mutex // 保护 status 字段写

	setHealth := func(h *HealthResponse, err error) {
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			status.HealthErr = err
			return
		}
		status.Health = h
	}

	setKernel := func(k *KernelInfo, err error) {
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			status.KernelErr = err
			return
		}
		status.Kernel = k
		status.Modules = k.Modules
	}

	setAgents := func(total int, err error) {
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			status.AgentsErr = err
			return
		}
		status.AgentsTotal = total
	}

	wg.Add(3)
	go func() {
		defer wg.Done()
		h, err := c.Health(ctx)
		setHealth(h, err)
	}()
	go func() {
		defer wg.Done()
		k, err := c.GetKernelInfo(ctx)
		setKernel(k, err)
	}()
	go func() {
		defer wg.Done()
		// 只拉 page=1,pageSize=1 → 只要 total 数,不要明细(节省带宽)
		_, total, err := c.ListAgentsRaw(ctx, 1, 1, "", "", "")
		setAgents(total, err)
	}()
	wg.Wait()

	// 全部 fail → 报"kernel unreachable"类错误
	if status.Health == nil && status.Kernel == nil && status.AgentsErr != nil {
		return status, fmt.Errorf("kernel unreachable (health=%v, kernel=%v, agents=%v)",
			status.HealthErr, status.KernelErr, status.AgentsErr)
	}

	return status, nil
}

// ListAgentsRaw 直接调 /registry/agents 不走 tolerant decoder,只数 array 长度。
//
// 跟 ListAgents 区别:ListAgents 返回 *AgentListResponse(struct),ListAgentsRaw 只数数组长度。
// 用于 P4.6 cluster status — 只关心 total 不关心明细。
func (c *Client) ListAgentsRaw(ctx context.Context, page, pageSize int, skill, status, search string) (agents []Agent, total int, err error) {
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

	// 先试 array 格式(live server 返回 raw array,per 2026-08-24 probe)
	var arr []Agent
	if err := c.Get(ctx, path, &arr); err == nil {
		return arr, len(arr), nil
	}
	// 再试 object 格式(可能未来 server 改)
	var obj AgentListResponse
	if err := c.Get(ctx, path, &obj); err != nil {
		return nil, 0, err
	}
	return obj.Agents, int(obj.Total), nil
}