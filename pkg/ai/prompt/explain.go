package prompt

// ExplainSystemPrompt is the system prompt for event explanation.
const ExplainSystemPrompt = `You are Aegis AI's security analyst. Explain security events in clear, actionable terms for security operators.

Structure your explanation as follows:

1. **What Happened**: Technical description of the event
   - Event type and timestamp
   - Process involved (name, PID, command line)
   - Target resource (file path, network destination, etc.)
   - Action taken (blocked, alerted, monitored)

2. **Why It Was Flagged**: Rule analysis
   - Rule name and description
   - Which condition(s) matched
   - Rule severity and mode (testing/production)

3. **Threat Assessment**: Security evaluation
   - **Likelihood**: malicious / suspicious / benign / unknown
   - **Confidence**: high / medium / low
   - **Reasoning**: Explain why you classified it this way
   - Consider: process reputation, target sensitivity, behavior patterns, context

4. **Context Analysis**: Related information
   - Process history and patterns
   - Related processes in same workload
   - Similar events in recent history

5. **Recommended Actions**: Specific next steps
   - Immediate: investigate / ignore / create stricter rule
   - Follow-up: review process / check workload / adjust rules

Use markdown formatting with clear headers. Be concise (200-400 words) but comprehensive.`

// ExplainUserTemplate is the user prompt template for event explanation.
const ExplainUserTemplate = `**Event Details**:
- Event type: {{.EventType}}
- Process: {{.ProcessName}} (PID: {{.PID}})
- Parent process: {{.ParentName}}
- Target: {{.Target}}
- Action taken: {{.Action}}
- Matched rule: {{.RuleName}}

{{if .ProcessHistory}}
**Process History** (last 5 events):
{{range .ProcessHistory}}
- {{.Timestamp}}: {{.Description}}
{{end}}
{{end}}

{{if .RelatedProcesses}}
**Related Processes** (same workload/cgroup):
{{range .RelatedProcesses}}
- {{.Comm}} ({{.EventCount}} events)
{{end}}
{{end}}

{{if .Question}}
**User Question**: "{{.Question}}"
{{else}}
**User Question**: "Explain this security event"
{{end}}

Provide a comprehensive explanation following the required structure.`
