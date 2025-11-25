<script setup lang="ts">
import { computed } from 'vue'
import { Handle, Position } from '@vue-flow/core'
import type { NodeProps } from '@vue-flow/core'
import { Box, AlertTriangle, Container } from 'lucide-vue-next'

export interface ProcessNodeData {
  pid: number
  ppid: number
  comm: string
  timestamp: number
  isTarget?: boolean
  isAlertSource?: boolean
  inContainer?: boolean
  severity?: 'high' | 'warning' | 'info'
}

const props = defineProps<NodeProps<ProcessNodeData>>()

const formatTime = (timestamp: number) => {
  if (!timestamp) return ''
  return new Date(timestamp).toLocaleTimeString('en-US', {
    hour12: false,
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit'
  })
}

const nodeClass = computed(() => ({
  'is-target': props.data.isTarget,
  'is-alert-source': props.data.isAlertSource,
  [`severity-${props.data.severity}`]: props.data.severity
}))
</script>

<template>
  <div class="process-node" :class="nodeClass">
    <!-- Input handle (from parent) -->
    <Handle type="target" :position="Position.Top" class="node-handle" />
    
    <div class="node-header">
      <div class="node-icon">
        <AlertTriangle v-if="data.isAlertSource" :size="14" class="alert-icon" />
        <Container v-else-if="data.inContainer" :size="14" class="container-icon" />
        <Box v-else :size="14" />
      </div>
      <span class="node-comm">{{ data.comm }}</span>
      <span v-if="data.severity" class="severity-badge" :class="data.severity">
        {{ data.severity.toUpperCase() }}
      </span>
    </div>
    
    <div class="node-meta">
      <div class="meta-row">
        <span class="meta-label">PID:</span>
        <span class="meta-value">{{ data.pid }}</span>
      </div>
      <div v-if="data.ppid" class="meta-row">
        <span class="meta-label">PPID:</span>
        <span class="meta-value">{{ data.ppid }}</span>
      </div>
      <div v-if="data.timestamp" class="meta-row">
        <span class="meta-label">Time:</span>
        <span class="meta-value">{{ formatTime(data.timestamp) }}</span>
      </div>
    </div>
    
    <div v-if="data.inContainer" class="container-badge">
      <Container :size="10" />
      <span>Container</span>
    </div>
    
    <!-- Output handle (to children) -->
    <Handle type="source" :position="Position.Bottom" class="node-handle" />
  </div>
</template>

<style scoped>
.process-node {
  background: var(--bg-elevated);
  border: 1px solid var(--border-default);
  border-radius: var(--radius-md);
  padding: 12px 16px;
  min-width: 160px;
  transition: all 0.2s ease;
}

.process-node:hover {
  background: var(--bg-overlay);
  border-color: var(--border-focus);
}

.process-node.is-target {
  background: var(--bg-overlay);
  border-color: var(--accent-primary);
  box-shadow: var(--glow-accent);
}

.process-node.is-alert-source {
  border-color: var(--status-critical);
}

.process-node.is-alert-source.severity-high {
  box-shadow: var(--glow-critical);
}

.process-node.is-alert-source.severity-warning {
  border-color: var(--status-warning);
  box-shadow: 0 0 20px rgba(245, 158, 11, 0.3);
}

.node-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
}

.node-icon {
  color: var(--text-muted);
  display: flex;
  align-items: center;
}

.node-icon .alert-icon {
  color: var(--status-critical);
}

.node-icon .container-icon {
  color: var(--status-info);
}

.node-comm {
  font-family: var(--font-mono);
  font-size: 14px;
  font-weight: 600;
  color: var(--text-primary);
  flex: 1;
}

.severity-badge {
  padding: 2px 6px;
  border-radius: var(--radius-sm);
  font-size: 9px;
  font-weight: 700;
  text-transform: uppercase;
}

.severity-badge.high {
  background: var(--status-critical-dim);
  color: var(--status-critical);
}

.severity-badge.warning {
  background: var(--status-warning-dim);
  color: var(--status-warning);
}

.severity-badge.info {
  background: var(--status-info-dim);
  color: var(--status-info);
}

.node-meta {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.meta-row {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 11px;
}

.meta-label {
  color: var(--text-muted);
}

.meta-value {
  font-family: var(--font-mono);
  color: var(--text-secondary);
}

.container-badge {
  display: flex;
  align-items: center;
  gap: 4px;
  margin-top: 8px;
  padding: 4px 8px;
  background: var(--status-info-dim);
  border-radius: var(--radius-sm);
  font-size: 10px;
  color: var(--status-info);
}

.node-handle {
  width: 8px;
  height: 8px;
  background: var(--bg-overlay);
  border: 2px solid var(--border-default);
}

.node-handle:hover {
  background: var(--accent-primary);
  border-color: var(--accent-primary);
}
</style>

