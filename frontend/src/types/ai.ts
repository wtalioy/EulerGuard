// AI Types - Phase 4

export type IntentType = 
  | 'create_rule'
  | 'query_events'
  | 'explain_event'
  | 'analyze_process'
  | 'promote_rule'
  | 'navigation'

export interface Intent {
  type: IntentType
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

export interface RequestContext {
  currentPage?: string
  selectedItem?: string
  recentActions?: string[]
}

export interface RuleGenRequest {
  description: string
  context?: RequestContext
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

