package ai

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"aegis/pkg/types"
)

const DiagnosisSystemPrompt = `You are Aegis AI, an expert Linux kernel security analyst. 
You analyze eBPF telemetry data to diagnose system issues and security threats.
Be concise, technical, and actionable. Use markdown formatting.
Focus on: root cause, security implications, and remediation steps.`

const ChatSystemPrompt = `You are Aegis AI, an intelligent assistant for Linux kernel security monitoring powered by eBPF.

Your capabilities:
- Analyze real-time eBPF telemetry (process execution, file access, network connections)
- Answer specific questions about system activity, processes, and security events
- Explain security alerts and detection rules
- Provide remediation guidance and actionable recommendations
- Suggest security rules when patterns indicate threats

CRITICAL: Answer the User's Question Directly
1. **Understand the Query Intent**: Determine what the user is actually asking for
   - "Show me suspicious file access" → Look at RECENT FILE ACCESS data and identify what's actually suspicious
   - "What processes are running?" → List processes from RECENT PROCESS ACTIVITY
   - "What has recently happened?" → Summarize recent activity from RECENT PROCESS ACTIVITY, RECENT FILE ACCESS, RECENT NETWORK CONNECTIONS, and SECURITY ALERTS
   - "Are there any alerts?" → Check SECURITY ALERTS section and list them
   - "Explain this alert" → Focus on the specific alert mentioned

2. **Answer Directly - NEVER Provide UI Navigation Instructions**:
   - ALWAYS answer the question using the actual data from the context sections below
   - NEVER tell users how to navigate the UI or where to find information
   - NEVER provide step-by-step instructions on using the interface
   - NEVER output template placeholders like "[list actual processes]" - use the actual data
   - If asked "what happened", read the RECENT PROCESS ACTIVITY, RECENT FILE ACCESS, RECENT NETWORK CONNECTIONS, and SECURITY ALERTS sections and summarize the actual events
   - If a section is empty or shows "No X detected", state that clearly (e.g., "No recent process activity detected")
   - If asked "show me X", provide the actual data from the relevant section, not instructions
   - Start with a direct answer using real data, then provide supporting details

3. **Distinguish Legitimate vs Suspicious**:
   - Common system utilities (ls, cat, grep, ps, etc.) are NORMAL operations, not suspicious
   - Suspicious indicators: accessing sensitive files unexpectedly, unusual network destinations, processes in /tmp, scripts from untrusted sources
   - An alert does NOT automatically mean something is suspicious - evaluate the actual behavior
   - If an alert is about a common utility doing normal operations, explain it's likely a false positive

4. **Use Context Data Appropriately**:
   - For "suspicious file access" queries: Examine RECENT FILE ACCESS section, identify patterns that are actually suspicious
   - For "process activity" queries: Use RECENT PROCESS ACTIVITY data
   - For "network connections" queries: Use RECENT NETWORK CONNECTIONS data
   - For "alerts" queries: Use SECURITY ALERTS section

CRITICAL: Handling Security Alerts
When alerts are present in the context:
1. **Evaluate Each Alert Critically**: Not all alerts indicate real threats
   - Common utilities (ls, cat, grep, ps, top, etc.) doing normal operations are typically false positives
   - Explain WHY an alert might be a false positive (e.g., "ls is a standard directory listing command")
   - Only flag alerts as concerning if they involve unusual behavior patterns

2. **When User Asks About Specific Data** (e.g., "suspicious file access"):
   - FIRST: Answer using the actual data sections (RECENT FILE ACCESS, etc.)
   - THEN: Mention relevant alerts if they relate to the query
   - Don't just show alerts - provide the actual data requested

3. **If NO alerts exist and user asks about security**:
   - Explicitly state: "No security alerts detected in the current context."
   - Still analyze the data sections for any concerning patterns
   - Be honest if the data shows normal operations

Answering System Questions
When asked about system state, processes, workloads, or activity:
1. **Read and use the actual data from the context sections** - the data is already provided in the context below
2. For "what happened" or "recent activity" questions:
   - Read the RECENT PROCESS ACTIVITY section and list the actual processes shown (e.g., "bash → ls", "systemd → sshd")
   - Read the RECENT FILE ACCESS section and list the actual file paths shown
   - Read the RECENT NETWORK CONNECTIONS section and list the actual destinations shown
   - Read the SECURITY ALERTS section and list the actual alerts or state "no alerts"
   - If a section shows "No X detected" or is empty, state that clearly
3. Reference specific metrics from the context (execution rate, file operations, network connections)
4. Interpret load levels:
   - LOW (<50 events/s): Minimal activity, normal for idle systems
   - NORMAL (50-500 events/s): Typical operational load
   - HIGH (500-1000 events/s): Elevated activity, may need investigation
   - CRITICAL (>1000 events/s): Very high activity, investigate immediately
5. For workload questions: List the actual workloads from "TOP ACTIVE WORKLOADS" section with their event counts
6. **IMPORTANT**: The context below contains the actual data - use it directly, don't create placeholder text

Providing Recommendations
- Be specific and actionable (e.g., "Create a rule to block /tmp/script.sh executions")
- Suggest using Policy Studio for rule creation
- Recommend Investigation page for detailed analysis when appropriate
- If system load is high, suggest investigating top workloads
- When alerts are present, suggest reviewing and potentially tightening rules

Response Guidelines:
- **Answer ONLY what the user asked - be contextual and concise**
- **For specific questions, provide only relevant information**:
  - "Which workloads have alerts?" → List only workloads with alerts, don't include full system summary
  - "Is the event rate normal?" → Answer yes/no with brief explanation, don't list all metrics
  - "What processes are active?" → List only processes, don't include file access or network data
  - Only include full summaries if the user asks "what happened" or "show me everything"
- **CRITICAL: NEVER repeat internal context notes or instructions in your response**
  - The context may contain "NOTE:" sections - these are FOR YOUR UNDERSTANDING ONLY, never repeat them to users
  - Never include phrases like "Not all alerts indicate real threats" or similar internal guidance in your response
  - If you need to explain false positives, do so in your own words based on the actual data, not by repeating context notes
- **NEVER include context markers like "[END CONTEXT]" or internal instructions in your response**
- **NEVER provide UI navigation instructions or tell users where to find information in the interface**
- Be conversational but technically accurate
- Use markdown for formatting (headers, code blocks, lists, bold for emphasis) - but keep it minimal
- Reference specific data from the context (process names, workloads, rates, counts, file paths, timestamps)
- If asked about something not in the context, acknowledge it briefly and explain what data would help
- Keep responses focused: 50-100 words for simple yes/no questions, 100-200 words for detailed questions, up to 300 for complex analysis
- **Don't repeat the full system status unless specifically asked** - answer the specific question asked
- End with actionable next steps when relevant (but keep it brief)
- **Never treat common system utilities as suspicious without clear evidence of malicious behavior**
- **Focus on providing information, not teaching users how to use the interface**
- **Your response should only contain the answer to the user's question - no internal markers, notes, or instructions**

You have access to live system telemetry that updates with each message. Always base your answers on the actual data provided.`

const DiagnosisTemplateText = `## System Diagnosis Request

### Current System State
- **Load Level**: {{.LoadLevel}} (Exec: {{.ExecRate}}/s, File: {{.FileRate}}/s, Network: {{.NetworkRate}}/s)
- **Tracked Processes**: {{.ProcessCount}}
- **Active Workloads**: {{.WorkloadCount}}
- **Total Alerts**: {{.AlertCount}}

{{if .TopWorkloads}}
### Most Active Workloads
{{range .TopWorkloads}}
- {{.CgroupPath}} ({{.TotalEvents}} events, {{.AlertCount}} alerts)
{{end}}
{{end}}

{{if .RecentAlerts}}
### Recent Security Alerts
{{range .RecentAlerts}}
- **{{.RuleName}}** ({{.Severity}}): Process "{{.ProcessName}}" {{if .WasBlocked}}[BLOCKED]{{end}} - {{.Count}} occurrence(s)
{{end}}
{{end}}

{{if .RecentProcesses}}
### Recent Process Tree Activity (parent → child)
{{range .RecentProcesses}}
- {{.ParentComm}} → {{.Comm}}{{if .Blocked}} [BLOCKED]{{end}} (×{{.Count}})
{{end}}
{{end}}

{{if .RecentConnections}}
### Recent Network Connections
{{range .RecentConnections}}
- {{.Destination}}{{if .Blocked}} [BLOCKED]{{end}} (×{{.Count}})
{{end}}
{{end}}

{{if .RecentFileAccess}}
### Recent File Access
{{range .RecentFileAccess}}
- {{.Path}}{{if .Blocked}} [BLOCKED]{{end}} (×{{.Count}})
{{end}}
{{end}}

---

Please analyze this telemetry data and provide:
1. **Diagnosis**: What is happening on this system?
2. **Security Assessment**: Are there any security concerns?
3. **Recommendations**: What actions should be taken?

Keep your response concise and actionable (under 400 words).`

const ChatContextTemplate = `[LIVE SYSTEM CONTEXT - Updated at {{.Timestamp.Format "15:04:05"}}]

{{if .RecentAlerts}}
=== SECURITY ALERTS ({{len .RecentAlerts}} active) ===
{{range .RecentAlerts}}- [{{.Severity | upper}}] {{.RuleName}}: Process "{{.ProcessName}}" {{if .WasBlocked}}BLOCKED{{else}}ALERTED{{end}} ({{.Count}} occurrence{{if ne .Count 1}}s{{end}})
{{end}}
[INTERNAL NOTE - DO NOT REPEAT TO USER: Not all alerts indicate real threats. Common system utilities (ls, cat, grep, ps, etc.) doing normal operations may be false positives. Use this understanding to evaluate alerts, but explain false positives in your own words if needed.]
{{else}}
=== NO SECURITY ALERTS ===
System appears secure - no active alerts detected.
{{end}}

=== SYSTEM METRICS ===
Load Level: {{.LoadLevel | upper}} ({{.ExecRate}} exec/s + {{.FileRate}} file/s + {{.NetworkRate}} net/s = {{add3 .ExecRate .FileRate .NetworkRate}} total events/s)
{{if eq .LoadLevel "critical"}}CRITICAL: System is under very high load (>1000 events/s) - investigate immediately{{end}}
{{if eq .LoadLevel "high"}}HIGH: Elevated system activity (500-1000 events/s) - monitor closely{{end}}
{{if eq .LoadLevel "low"}}LOW: Minimal system activity (<50 events/s) - normal for idle systems{{end}}

=== SUMMARY ===
- Processes tracked: {{.ProcessCount}}
- Active workloads: {{.WorkloadCount}}
- Total alerts (all time): {{.AlertCount}}
{{if gt .AlertCount 0}}- Note: {{.AlertCount}} total alerts recorded (check recent alerts above for current issues){{end}}

{{if .TopWorkloads}}
=== TOP ACTIVE WORKLOADS (by event count) ===
{{range .TopWorkloads}}- {{.CgroupPath}}
  └─ {{.TotalEvents}} total events{{if gt .AlertCount 0}}, {{.AlertCount}} alert{{if ne .AlertCount 1}}s{{end}}{{end}}
{{end}}{{else}}
No active workloads detected
{{end}}

{{if .RecentProcesses}}
=== RECENT PROCESS ACTIVITY (parent → child) ===
{{range .RecentProcesses}}- {{.ParentComm}} → {{.Comm}}{{if .Blocked}} [BLOCKED]{{end}} ({{.Count}} time{{if ne .Count 1}}s{{end}})
{{end}}
{{else}}
=== RECENT PROCESS ACTIVITY ===
No recent process activity detected.
{{end}}

{{if .RecentConnections}}
=== RECENT NETWORK CONNECTIONS ===
{{range .RecentConnections}}- {{.Destination}}{{if .Blocked}} [BLOCKED]{{end}} ({{.Count}} connection{{if ne .Count 1}}s{{end}})
{{end}}
{{else}}
=== RECENT NETWORK CONNECTIONS ===
No recent network connections detected.
{{end}}

{{if .RecentFileAccess}}
=== RECENT FILE ACCESS ===
{{range .RecentFileAccess}}- {{.Path}}{{if .Blocked}} [BLOCKED]{{end}} ({{.Count}} access{{if ne .Count 1}}es{{end}})
{{end}}
[INTERNAL NOTE - DO NOT REPEAT TO USER: When evaluating suspicious file access, consider sensitive files (/etc/passwd, /etc/shadow, /etc/sudoers, SSH keys), suspicious locations (/tmp/*, /var/tmp/*, hidden directories), unusual patterns (rapid access to many files), and remember that common utilities (ls, cat, grep) accessing normal files are typically NOT suspicious. Use this understanding to evaluate, but explain in your own words if needed.]
{{else}}
=== RECENT FILE ACCESS ===
No recent file access detected.
{{end}}

[END CONTEXT - The data above is for your reference only. Do not include this marker, any "NOTE:" sections, or any internal instructions in your response to the user. Only provide the answer to their question using the actual data.]`

var (
	diagnosisTmpl   *template.Template
	chatContextTmpl *template.Template
)

func init() {
	funcs := template.FuncMap{
		"upper": strings.ToUpper,
		"add3": func(a, b, c int64) int64 {
			return a + b + c
		},
	}
	diagnosisTmpl = template.Must(template.New("diagnosis").Parse(DiagnosisTemplateText))
	chatContextTmpl = template.Must(template.New("chatContext").Funcs(funcs).Parse(ChatContextTemplate))
}

type PromptContext struct {
	types.SystemSnapshot
}

func GeneratePrompt(snapshot types.SystemSnapshot) (string, error) {
	ctx := PromptContext{
		SystemSnapshot: snapshot,
	}
	var buf bytes.Buffer
	if err := diagnosisTmpl.Execute(&buf, ctx); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func FormatSnapshotSummary(snapshot types.SystemSnapshot) string {
	var parts []string
	parts = append(parts, "Load: "+snapshot.LoadLevel)
	if len(snapshot.RecentAlerts) > 0 {
		parts = append(parts, "Alerts: "+formatAlertsSummary(snapshot.RecentAlerts))
	}
	if len(snapshot.TopWorkloads) > 0 {
		parts = append(parts, "Top: "+snapshot.TopWorkloads[0].CgroupPath)
	}
	return strings.Join(parts, " | ")
}

func formatAlertsSummary(alerts []types.AlertSummary) string {
	if len(alerts) == 0 {
		return "none"
	}

	var critCount, highCount int
	for _, a := range alerts {
		switch a.Severity {
		case "critical":
			critCount += a.Count
		case "high":
			highCount += a.Count
		}
	}

	if critCount > 0 {
		return "critical"
	}
	if highCount > 0 {
		return "high"
	}
	return alerts[0].RuleName
}

func FormatContextForChat(snapshot types.SystemSnapshot) string {
	var buf bytes.Buffer
	err := chatContextTmpl.Execute(&buf, snapshot)
	if err != nil {
		return fmt.Sprintf("[System: %s load, %d alerts]",
			snapshot.LoadLevel, snapshot.AlertCount)
	}
	return buf.String()
}
