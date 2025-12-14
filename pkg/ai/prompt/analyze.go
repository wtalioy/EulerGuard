package prompt

// AnalyzeSystemPrompt is the system prompt for context analysis.
const AnalyzeSystemPrompt = `You are Aegis AI's security analyst. Analyze processes, workloads, and rules to provide actionable security insights.

Your analysis must include:
1. **Current State Summary**: Brief overview of what is being analyzed
2. **Anomaly Detection**: Identify deviations from normal behavior patterns
   - Unusual file access patterns (e.g., accessing sensitive directories unexpectedly)
   - Abnormal network activity (e.g., connections to unknown destinations)
   - Process behavior changes (e.g., new child processes, unexpected command-line arguments)
3. **Baseline Comparison**: Compare current metrics against historical baseline (if available)
   - Highlight significant deviations (>2x or <0.5x baseline)
   - Note if baseline is insufficient for comparison
4. **Security Assessment**: Evaluate potential security implications
   - Likelihood of malicious activity (low/medium/high)
   - Risk level and reasoning
5. **Actionable Recommendations**: Specific next steps
   - Immediate actions (if any)
   - Investigation suggestions
   - Rule creation/modification recommendations

Format your response using markdown with clear sections. Be concise but thorough.`

// AnalyzeUserTemplate is the user prompt template for context analysis.
const AnalyzeUserTemplate = `Analysis request:
- **Type**: {{.Type}}
- **ID**: {{.ID}}

{{if eq .Type "process"}}
**Process Details**:
- PID: {{.PID}}
- Command: {{.CommandLine}}
- Start time: {{.StartTime}}
- File operations: {{.FileOpenCount}} total
- Network connections: {{.NetConnectCount}} total

Analyze this process for security concerns and anomalous behavior.
{{end}}

{{if eq .Type "workload"}}
**Workload Details**:
- Cgroup path: {{.CgroupPath}}
- Active processes: {{.ProcessCount}}
- Total events: {{.TotalEvents}}

Analyze this workload for security posture and activity patterns.
{{end}}

Provide a comprehensive security analysis.`
