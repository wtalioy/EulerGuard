<script setup lang="ts">
import { computed } from 'vue'
import { ShieldOff, AlertTriangle } from 'lucide-vue-next'
import type { Alert } from '../../lib/api'

const props = defineProps<{
  alert: Alert
  isSelected: boolean
}>()

defineEmits<{
  select: [alert: Alert]
}>()

const severityLabel = computed(() => {
  switch (props.alert.severity) {
    case 'critical': return 'CRIT'
    case 'high': return 'HIGH'
    case 'warning': return 'WARN'
    default: return 'INFO'
  }
})

const actionLabel = computed(() => {
  if (props.alert.blocked) return 'BLOCKED'
  return props.alert.action?.toUpperCase() || 'ALERT'
})

const formatTime = (timestamp: number) => {
  return new Date(timestamp).toLocaleTimeString('en-US', {
    hour12: false,
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit'
  })
}
</script>

<template>
  <div 
    class="alert-card" 
    :class="[
      `severity-${alert.severity}`, 
      { 'is-selected': isSelected, 'is-blocked': alert.blocked }
    ]"
    @click="$emit('select', alert)"
  >
    <div class="alert-indicator"></div>
    <div class="alert-content">
      <div class="alert-header">
        <div class="alert-badges">
          <span class="alert-severity" :class="alert.severity">{{ severityLabel }}</span>
          <span v-if="alert.blocked" class="alert-action blocked">
            <ShieldOff :size="10" />
            {{ actionLabel }}
          </span>
          <span v-else class="alert-action alerted">
            <AlertTriangle :size="10" />
            ALERT
          </span>
        </div>
        <span class="alert-time font-mono">{{ formatTime(alert.timestamp) }}</span>
      </div>
      <div class="alert-title">{{ alert.ruleName }}</div>
      <div class="alert-process">
        <code>{{ alert.processName }}</code>
        <span class="alert-pid">PID {{ alert.pid }}</span>
        <span v-if="alert.cgroupId" class="alert-cgroup font-mono">cg:{{ alert.cgroupId.slice(0, 8) }}</span>
      </div>
    </div>
  </div>
</template>

<style scoped>
.alert-card {
  position: relative;
  padding: 12px 16px 12px 20px;
  background: var(--bg-elevated);
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.2s ease;
  margin-bottom: 8px;
}

.alert-card:hover {
  background: var(--bg-overlay);
}

.alert-indicator {
  position: absolute;
  left: 0;
  top: 0;
  bottom: 0;
  width: 4px;
  border-radius: 8px 0 0 8px;
  background: var(--text-muted);
}

.severity-critical .alert-indicator { background: var(--status-blocked); }
.severity-high .alert-indicator { background: var(--status-critical); }
.severity-warning .alert-indicator { background: var(--status-warning); }
.severity-info .alert-indicator { background: var(--status-info); }

/* Blocked indicator override - pulsing effect */
.is-blocked .alert-indicator {
  background: var(--status-blocked);
  animation: blocked-pulse 2s ease-in-out infinite;
}

@keyframes blocked-pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.5; }
}

.alert-card.is-selected {
  background: var(--bg-overlay);
  box-shadow: 0 0 0 2px var(--border-focus);
}

.severity-critical.is-selected,
.is-blocked.is-selected {
  box-shadow: 0 0 0 2px var(--status-blocked-glow), 0 0 20px var(--status-blocked-glow);
}

.severity-high.is-selected {
  box-shadow: 0 0 0 2px rgba(239, 68, 68, 0.5), var(--glow-critical);
}

.severity-warning.is-selected {
  box-shadow: 0 0 0 2px rgba(245, 158, 11, 0.5), 0 0 20px rgba(245, 158, 11, 0.3);
}

.alert-content {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.alert-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.alert-badges {
  display: flex;
  align-items: center;
  gap: 6px;
}

.alert-severity {
  padding: 2px 8px;
  border-radius: var(--radius-sm);
  font-size: 10px;
  font-weight: 600;
  text-transform: uppercase;
}

.alert-severity.critical {
  background: var(--status-blocked-dim);
  color: var(--status-blocked);
}

.alert-severity.high {
  background: var(--status-critical-dim);
  color: var(--status-critical);
}

.alert-severity.warning {
  background: var(--status-warning-dim);
  color: var(--status-warning);
}

.alert-severity.info {
  background: var(--status-info-dim);
  color: var(--status-info);
}

.alert-action {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 2px 6px;
  border-radius: var(--radius-sm);
  font-size: 9px;
  font-weight: 700;
  text-transform: uppercase;
}

.alert-action.blocked {
  background: var(--status-blocked-dim);
  color: var(--status-blocked);
  border: 1px solid var(--status-blocked);
}

.alert-action.alerted {
  background: var(--status-warning-dim);
  color: var(--status-warning);
  opacity: 0.7;
}

.alert-time {
  font-size: 11px;
  color: var(--text-muted);
}

.alert-title {
  font-size: 13px;
  font-weight: 500;
  color: var(--text-primary);
  line-height: 1.3;
}

.alert-process {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
}

.alert-process code {
  font-family: var(--font-mono);
  color: var(--text-secondary);
  background: var(--bg-surface);
  padding: 2px 6px;
  border-radius: var(--radius-sm);
}

.alert-pid {
  color: var(--text-muted);
  font-family: var(--font-mono);
  font-size: 11px;
}

.alert-cgroup {
  color: var(--text-muted);
  font-size: 10px;
  padding: 1px 4px;
  background: var(--bg-surface);
  border-radius: var(--radius-sm);
}

/* Blocked card has subtle background tint */
.is-blocked {
  background: linear-gradient(135deg, var(--bg-elevated) 0%, rgba(220, 38, 38, 0.05) 100%);
}

.is-blocked:hover {
  background: linear-gradient(135deg, var(--bg-overlay) 0%, rgba(220, 38, 38, 0.08) 100%);
}

/* New alert pulse animation */
@keyframes alert-pulse {
  0%, 100% { 
    box-shadow: 0 0 0 0 var(--glow-critical); 
  }
  50% { 
    box-shadow: 0 0 20px 4px var(--glow-critical); 
  }
}

.alert-card.severity-high.is-new {
  animation: alert-pulse 0.6s ease-out 2;
}

.alert-card.is-blocked.is-new {
  animation: alert-pulse 0.6s ease-out 3;
}
</style>
