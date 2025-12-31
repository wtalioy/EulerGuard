package apimodel

type ExecEvent struct {
	Type        string `json:"type"`
	Timestamp   int64  `json:"timestamp"`
	PID         uint32 `json:"pid"`
	PPID        uint32 `json:"ppid"`
	CgroupID    string `json:"cgroupId"`
	Comm        string `json:"comm"`
	ParentComm  string `json:"parentComm"`
	CommandLine string `json:"commandLine,omitempty"`
	Blocked     bool   `json:"blocked"`
}

type FileEvent struct {
	Type      string `json:"type"`
	Timestamp int64  `json:"timestamp"`
	PID       uint32 `json:"pid"`
	CgroupID  string `json:"cgroupId"`
	Flags     uint32 `json:"flags"`
	Ino       uint64 `json:"ino,omitempty"`
	Dev       uint64 `json:"dev,omitempty"`
	Filename  string `json:"filename"`
	Blocked   bool   `json:"blocked"`
}

type ConnectEvent struct {
	Type        string `json:"type"`
	Timestamp   int64  `json:"timestamp"`
	PID         uint32 `json:"pid"`
	ProcessName string `json:"processName,omitempty"`
	CgroupID    string `json:"cgroupId"`
	Family      uint16 `json:"family"`
	Port        uint16 `json:"port"`
	Addr        string `json:"addr"`
	Blocked     bool   `json:"blocked"`
}

type Alert struct {
	ID          string `json:"id"`
	Timestamp   int64  `json:"timestamp"`
	Severity    string `json:"severity"`
	RuleName    string `json:"ruleName"`
	Description string `json:"description"`
	PID         uint32 `json:"pid"`
	ProcessName string `json:"processName"`
	ParentName  string `json:"parentName"`
	CgroupID    string `json:"cgroupId"`
	Action      string `json:"action"`
	Blocked     bool   `json:"blocked"`
}

type Workload struct {
	ID           string `json:"id"`
	CgroupPath   string `json:"cgroupPath"`
	ExecCount    int64  `json:"execCount"`
	FileCount    int64  `json:"fileCount"`
	ConnectCount int64  `json:"connectCount"`
	AlertCount   int64  `json:"alertCount"`
	BlockedCount int64  `json:"blockedCount"`
	FirstSeen    int64  `json:"firstSeen"`
	LastSeen     int64  `json:"lastSeen"`
}

type ProcessInfo struct {
	PID       uint32 `json:"pid"`
	PPID      uint32 `json:"ppid"`
	Comm      string `json:"comm"`
	CgroupID  string `json:"cgroupId"`
	Timestamp int64  `json:"timestamp"`
}

type InsightAction struct {
	ActionID string                 `json:"action_id"`
	Label    string                 `json:"label"`
	Payload  map[string]interface{} `json:"payload,omitempty"`
}

type Insight struct {
	ID         string                 `json:"id"`
	Type       string                 `json:"type"`
	Title      string                 `json:"title"`
	Summary    string                 `json:"summary"`
	Severity   string                 `json:"severity"`
	Confidence float64                `json:"confidence"`
	CreatedAt  interface{}            `json:"created_at"` // Can be string or number
	Actions    []InsightAction        `json:"actions"`
	Data       map[string]interface{} `json:"data"`
}

type AskInsightRequest struct {
	Insight  Insight `json:"insight"`
	Question string  `json:"question"`
}

type AskInsightResponse struct {
	Answer      string      `json:"answer"`
	Confidence  float64     `json:"confidence"`
	RelatedData interface{} `json:"related_data,omitempty"`
}


