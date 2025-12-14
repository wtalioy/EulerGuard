// AI API Wrapper - Phase 4
// Separate AI-specific API functions from general api.ts

export interface IntentResponse {
  type: string
  confidence: number
  params: Record<string, any>
  preview?: {
    type: string
    content: string
  }
  warnings?: string[]
  ambiguous?: boolean
  clarification?: string
}

export interface RuleGenRequest {
  description: string
  context?: {
    currentPage?: string
    selectedItem?: string
    recentActions?: string[]
  }
  examples?: any[]
}

export interface RuleGenResponse {
  rule: any
  yaml: string
  reasoning: string
  confidence: number
  warnings: string[]
  simulation?: any
}

export interface ExplainRequest {
  eventId?: string
  eventData?: any
  question?: string
}

export interface ExplainResponse {
  explanation: string
  rootCause: string
  matchedRule?: any
  relatedEvents?: any[]
  suggestedActions?: any[]
}

export interface AnalyzeRequest {
  type: 'process' | 'workload' | 'rule'
  id: string
}

export interface AnalyzeResponse {
  summary: string
  anomalies: any[]
  baselineStatus: string
  recommendations: any[]
  relatedInsights: any[]
}

const API_BASE = '/api/ai'

export async function parseIntent(input: string, context?: any): Promise<IntentResponse | null> {
  try {
    const response = await fetch(`${API_BASE}/intent`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ input, context })
    })
    if (!response.ok) return null
    return await response.json()
  } catch {
    return null
  }
}

export async function generateRule(req: RuleGenRequest): Promise<RuleGenResponse | null> {
  try {
    const response = await fetch(`${API_BASE}/generate-rule`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(req)
    })
    if (!response.ok) return null
    return await response.json()
  } catch {
    return null
  }
}

export async function explainEvent(req: ExplainRequest): Promise<ExplainResponse | null> {
  try {
    const response = await fetch(`${API_BASE}/explain`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(req)
    })
    if (!response.ok) return null
    return await response.json()
  } catch {
    return null
  }
}

export async function analyzeContext(req: AnalyzeRequest): Promise<AnalyzeResponse | null> {
  try {
    const response = await fetch(`${API_BASE}/analyze`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(req)
    })
    if (!response.ok) return null
    return await response.json()
  } catch {
    return null
  }
}

export async function getSentinelInsights(limit = 50): Promise<any[]> {
  try {
    const response = await fetch(`${API_BASE}/sentinel/insights?limit=${limit}`)
    if (!response.ok) return []
    const data = await response.json()
    return data.insights || []
  } catch {
    return []
  }
}

