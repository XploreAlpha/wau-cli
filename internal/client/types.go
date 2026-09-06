package client

import (
	"encoding/json"
	"fmt"
)

// HealthResponse is the response from /health endpoint.
type HealthResponse struct {
	Status  string  `json:"status"`
	Version string  `json:"version"`
	Uptime  float64 `json:"uptime"`
	Redis   string  `json:"redis"`
	Error   string  `json:"error,omitempty"`
}

// KernelInfo is the kernel information.
type KernelInfo struct {
	Version     string   `json:"version"`
	StartTime   string   `json:"startTime"`
	Uptime      int64    `json:"uptime"`
	AgentsCount int      `json:"agentsCount"`
	TasksCount  int      `json:"tasksCount"`
	Modules     []string `json:"modules,omitempty"` // P4.6 kernel v0.5+ returns
}

// Agent represents a registered agent.
type Agent struct {
	Name        string   `json:"name"`
	ID          string   `json:"id"`
	URL         string   `json:"url"`
	Description string   `json:"description"`
	Skills      []string `json:"skills"`
	Universes   []string `json:"universes"`
	Trust       float64  `json:"trust"`
	Status      string   `json:"status"`
	LastSeen    string   `json:"lastSeen"`
}

// AgentListResponse is the paginated list of agents.
type AgentListResponse struct {
	Agents     []Agent `json:"agents"`
	Total      int64   `json:"total"`
	Page       int     `json:"page"`
	PageSize   int     `json:"pageSize"`
	TotalPages int     `json:"totalPages"`
}

// AgentRegisterRequest is the request to register an agent.
type AgentRegisterRequest struct {
	Name        string   `json:"name"`
	URL         string   `json:"url"`
	Description string   `json:"description"`
	Skills      []string `json:"skills"`
	Universes   []string `json:"universes"`
}

// AgentScore represents an agent's score.
type AgentScore struct {
	Name        string  `json:"name"`
	TotalScore  float64 `json:"totalScore"`
	TrustScore  float64 `json:"trustScore"`
	SkillMatch  float64 `json:"skillMatch"`
	HealthScore float64 `json:"healthScore"`
	LoadScore   float64 `json:"loadScore"`
}

// AgentStatus represents an agent's comprehensive status.
//
// Per D104 (2026-09-06 v1.3.4 B3 修复):JSON tags 跟 kernel_api.AgentStatus 对齐:
//   - Name      ← kernel AgentID     (json:"agentId")
//   - Online    ← kernel Online bool  (json:"online")
//   - Circuit   ← kernel CircuitState int (json:"circuitState")
//   - Trust     ← json.RawMessage (kernel returns *TrustScore object)
//
// display layer 用 String() / strconv.FormatBool / .String() 转可读字符串。
type AgentStatus struct {
	Name       string          `json:"agentId"`
	Online     bool            `json:"online"`
	LastSeen   int64           `json:"lastSeen"`
	Trust      json.RawMessage `json:"trust,omitempty"`
	CircuitRaw int             `json:"circuitState"`
	Load       struct {
		ActiveTasks int     `json:"activeTasks"`
		MaxCapacity int     `json:"maxCapacity"`
		CPUUsage    float64 `json:"cpuUsage"`
		MemoryUsage float64 `json:"memoryUsage"`
	} `json:"load"`
}

// TrustScoreOrFloat 智能解析 Trust:object 找 score field,float 直接返回。
// 用于 display(如 "Trust: 0.85")。
func (a *AgentStatus) TrustScoreOrFloat() float64 {
	if len(a.Trust) == 0 {
		return 0
	}
	// 1) 试单 float64
	var f float64
	if err := json.Unmarshal(a.Trust, &f); err == nil {
		return f
	}
	// 2) 试 kernel_api.TrustScore object,找 score field
	var obj struct {
		Score float64 `json:"score"`
	}
	if err := json.Unmarshal(a.Trust, &obj); err == nil {
		return obj.Score
	}
	return 0
}

// TrustDisplay 友好显示:object 简化为 score,float 直接 .2f。
func (a *AgentStatus) TrustDisplay() string {
	return fmt.Sprintf("%.2f", a.TrustScoreOrFloat())
}

// StatusDisplay 把 Online bool 转 "online"/"offline"。
func (a *AgentStatus) StatusDisplay() string {
	if a.Online {
		return "online"
	}
	return "offline"
}

// CircuitDisplay 把 CircuitRaw int 转 "closed"/"open"/"half-open"(per kernel_api.CircuitState.String())。
func (a *AgentStatus) CircuitDisplay() string {
	switch a.CircuitRaw {
	case 0:
		return "closed"
	case 1:
		return "open"
	case 2:
		return "half-open"
	default:
		return fmt.Sprintf("unknown(%d)", a.CircuitRaw)
	}
}

// Task represents a task.
type Task struct {
	TaskID        string   `json:"taskId"`
	Message       string   `json:"message"`
	SourcePeer    string   `json:"sourcePeer"`
	SourceAgentID string   `json:"sourceAgentId,omitempty"`
	Status        string   `json:"status"`
	AssignedAgent string   `json:"assignedAgent,omitempty"`
	Result        string   `json:"result,omitempty"`
	CreatedAt     int64    `json:"createdAt"`
	UpdatedAt     int64    `json:"updatedAt"`
	RequiredSkills []string `json:"requiredSkills,omitempty"`
}

// TaskSubmitRequest is the request to submit a task.
// v0.6.0 M3 W7 fix: 字段名从 message/sourcePeer/sourceAgentId/intent 改为
// kernel 真相源 SubmitRequest 的 prompt + timeoutMs (对齐 wau-go-sdk types.go)。
type TaskSubmitRequest struct {
	Prompt    string `json:"prompt" binding:"required"`
	TimeoutMs int    `json:"timeout_ms,omitempty"`
}

// IntentDTO is the intent data transfer object.
type IntentDTO struct {
	Type                string   `json:"type"`
	RequiredSkills      []string `json:"requiredSkills"`
	Urgency             string   `json:"urgency"`
	EstimatedComplexity int      `json:"estimatedComplexity"`
}

// TaskSubmitResponse is the response from submitting a task.
type TaskSubmitResponse struct {
	TaskID        string `json:"taskId"`
	Status        string `json:"status"`
	AssignedAgent string `json:"assignedAgent"`
	Result        string `json:"result,omitempty"`
	Error         string `json:"error,omitempty"`
}
