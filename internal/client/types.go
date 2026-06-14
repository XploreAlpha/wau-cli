package client

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
	Version     string `json:"version"`
	StartTime   string `json:"startTime"`
	Uptime      int64  `json:"uptime"`
	AgentsCount int    `json:"agentsCount"`
	TasksCount  int    `json:"tasksCount"`
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
type AgentStatus struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Trust  float64 `json:"trust"`
	Load   struct {
		ActiveTasks int     `json:"activeTasks"`
		MaxCapacity int     `json:"maxCapacity"`
		CPUUsage    float64 `json:"cpuUsage"`
		MemoryUsage float64 `json:"memoryUsage"`
	} `json:"load"`
	Circuit string `json:"circuit"`
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
