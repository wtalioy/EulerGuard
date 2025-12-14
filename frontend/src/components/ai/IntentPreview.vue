<!-- Intent Preview Component - Phase 4 -->
<script setup lang="ts">
import { computed } from 'vue'
import { CheckCircle2, AlertTriangle, ArrowRight, Search, FileCode, Bot, Settings } from 'lucide-vue-next'
import type { Intent } from '../../composables/useAI'

const props = defineProps<{
  intent: Intent
}>()

defineEmits<{
  execute: []
}>()

// Get action description based on intent type
const actionDescription = computed(() => {
  const { type, params } = props.intent
  
  switch (type) {
    case 'create_rule':
      return {
        icon: FileCode,
        title: 'Create Security Rule',
        description: 'Open Policy Studio to create a new security rule based on your description',
        actionText: 'Go to Policy Studio',
        destination: 'Policy Studio'
      }
    case 'query_events':
      return {
        icon: Search,
        title: 'Search Events',
        description: 'Open Investigation page and search for events matching your query',
        actionText: 'Search in Investigation',
        destination: 'Investigation'
      }
    case 'explain_event':
      return {
        icon: Search,
        title: 'Explain Event',
        description: params.eventId 
          ? `View detailed explanation for event ${params.eventId}`
          : 'Open Investigation to view event details',
        actionText: 'View in Investigation',
        destination: 'Investigation'
      }
    case 'analyze_process':
      return {
        icon: Search,
        title: 'Analyze Process',
        description: params.pid
          ? `View detailed analysis for process ${params.pid}`
          : 'Open Investigation to analyze processes and workloads',
        actionText: 'Analyze in Investigation',
        destination: 'Investigation'
      }
    case 'promote_rule':
      return {
        icon: FileCode,
        title: 'Promote Rule',
        description: params.ruleName
          ? `Promote testing rule "${params.ruleName}" to production`
          : 'Open Policy Studio to manage rules',
        actionText: 'Go to Policy Studio',
        destination: 'Policy Studio'
      }
    case 'navigation':
      return {
        icon: Settings,
        title: 'Navigate',
        description: params.page 
          ? `Navigate to ${params.page}`
          : 'Navigate to page',
        actionText: 'Go',
        destination: params.page || 'Page'
      }
    default:
      return {
        icon: Bot,
        title: 'AI Action',
        description: 'Execute the identified action',
        actionText: 'Execute',
        destination: 'Action'
      }
  }
})

const canExecute = computed(() => {
  const { type, params } = props.intent
  
  // Check if required params are present
  switch (type) {
    case 'explain_event':
      return !!params.eventId
    case 'analyze_process':
      // For analyze_process, we can still execute even without PID
      // It will just navigate to investigation page
      return true
    case 'promote_rule':
      return !!params.ruleName
    default:
      return true
  }
})
</script>

<template>
  <div class="intent-preview">
    <div class="intent-header">
      <div class="intent-type">
        <CheckCircle2 v-if="intent.confidence > 0.8" :size="16" class="icon-success" />
        <AlertTriangle v-else :size="16" class="icon-warning" />
        <span class="intent-type-label">{{ intent.type.replace('_', ' ') }}</span>
        <span class="confidence-badge" :class="{
          'high': intent.confidence > 0.8,
          'medium': intent.confidence > 0.5 && intent.confidence <= 0.8,
          'low': intent.confidence <= 0.5
        }">
          {{ Math.round(intent.confidence * 100) }}%
        </span>
      </div>
    </div>

    <!-- Action Description -->
    <div class="action-description">
      <component :is="actionDescription.icon" :size="18" class="action-icon" />
      <div class="action-content">
        <div class="action-title">{{ actionDescription.title }}</div>
        <div class="action-desc">{{ actionDescription.description }}</div>
        <div class="action-destination">
          <ArrowRight :size="12" />
          <span>Will open: {{ actionDescription.destination }}</span>
        </div>
      </div>
    </div>

    <div v-if="intent.ambiguous" class="clarification">
      <AlertTriangle :size="14" />
      <span>{{ intent.clarification || 'Please clarify your intent' }}</span>
    </div>

    <div v-if="intent.warnings && intent.warnings.length > 0" class="warnings">
      <div v-for="(warning, idx) in intent.warnings" :key="idx" class="warning-item">
        <AlertTriangle :size="14" />
        <span>{{ warning }}</span>
      </div>
    </div>

    <div v-if="intent.preview" class="preview">
      <div class="preview-header">Preview:</div>
      <pre class="preview-content">{{ intent.preview.content }}</pre>
    </div>

    <button 
      class="execute-btn" 
      :class="{ 'disabled': !canExecute }"
      :disabled="!canExecute"
      @click="$emit('execute')"
    >
      <span>{{ actionDescription.actionText }}</span>
      <ArrowRight :size="16" />
    </button>
  </div>
</template>

<style scoped>
.intent-preview {
  padding: 16px 20px;
  border-top: 1px solid var(--border-subtle);
  background: var(--bg-elevated);
}

.intent-header {
  margin-bottom: 12px;
}

.intent-type {
  display: flex;
  align-items: center;
  gap: 8px;
}

.intent-type-label {
  text-transform: capitalize;
  font-weight: 600;
  color: var(--text-primary);
}

.confidence-badge {
  padding: 2px 8px;
  border-radius: var(--radius-sm);
  font-size: 11px;
  font-weight: 600;
}

.confidence-badge.high {
  background: rgba(34, 197, 94, 0.1);
  color: rgb(34, 197, 94);
}

.confidence-badge.medium {
  background: rgba(251, 191, 36, 0.1);
  color: rgb(251, 191, 36);
}

.confidence-badge.low {
  background: rgba(239, 68, 68, 0.1);
  color: rgb(239, 68, 68);
}

.icon-success {
  color: rgb(34, 197, 94);
}

.icon-warning {
  color: rgb(251, 191, 36);
}

.clarification,
.warnings {
  margin-top: 12px;
  padding: 10px;
  background: rgba(251, 191, 36, 0.1);
  border: 1px solid rgba(251, 191, 36, 0.2);
  border-radius: var(--radius-md);
  font-size: 13px;
  color: var(--text-secondary);
}

.clarification {
  display: flex;
  align-items: center;
  gap: 8px;
}

.warning-item {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 6px;
}

.warning-item:first-child {
  margin-top: 0;
}

.preview {
  margin-top: 12px;
}

.preview-header {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-muted);
  margin-bottom: 8px;
}

.preview-content {
  padding: 12px;
  background: var(--bg-void);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  font-size: 12px;
  font-family: 'Monaco', 'Menlo', monospace;
  color: var(--text-primary);
  overflow-x: auto;
  white-space: pre-wrap;
  word-break: break-all;
}

.action-description {
  margin-top: 16px;
  padding: 16px;
  background: var(--bg-void);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  display: flex;
  gap: 12px;
}

.action-icon {
  color: var(--accent-primary);
  flex-shrink: 0;
  margin-top: 2px;
}

.action-content {
  flex: 1;
}

.action-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-primary);
  margin-bottom: 6px;
}

.action-desc {
  font-size: 13px;
  color: var(--text-secondary);
  line-height: 1.5;
  margin-bottom: 8px;
}

.action-destination {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: var(--text-muted);
  padding-top: 8px;
  border-top: 1px solid var(--border-subtle);
}

.action-destination svg {
  color: var(--accent-primary);
}

.execute-btn {
  margin-top: 16px;
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 12px;
  background: var(--accent-primary);
  color: white;
  border: none;
  border-radius: var(--radius-md);
  font-weight: 600;
  cursor: pointer;
  transition: all 0.15s;
}

.execute-btn:hover:not(.disabled) {
  opacity: 0.9;
  transform: translateY(-1px);
}

.execute-btn.disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
</style>

