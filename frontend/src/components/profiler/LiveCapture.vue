<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { Check, Terminal, Globe, FileText } from 'lucide-vue-next'
import { subscribeToAllEvents, type StreamEvent } from '../../lib/api'

interface CapturedPattern {
  id: string
  type: 'exec' | 'connect' | 'file'
  description: string
  timestamp: number
}

const patterns = ref<CapturedPattern[]>([])
const MAX_DISPLAY = 50

let unsubscribe: (() => void) | null = null

const formatPattern = (event: StreamEvent): CapturedPattern => {
  const id = `${event.type}-${event.timestamp}-${event.pid}`
  
  switch (event.type) {
    case 'exec':
      return {
        id,
        type: 'exec',
        description: `Exec: ${event.comm} (from: ${event.parentComm})`,
        timestamp: event.timestamp
      }
    case 'connect':
      return {
        id,
        type: 'connect',
        description: `Connect: port ${event.port}`,
        timestamp: event.timestamp
      }
    case 'file':
      return {
        id,
        type: 'file',
        description: `File: ${event.filename}`,
        timestamp: event.timestamp
      }
  }
}

const handleEvent = (event: StreamEvent) => {
  const pattern = formatPattern(event)
  
  // Check for duplicates
  const exists = patterns.value.some(p => 
    p.type === pattern.type && p.description === pattern.description
  )
  
  if (!exists) {
    patterns.value.unshift(pattern)
    
    // Keep list manageable
    if (patterns.value.length > MAX_DISPLAY) {
      patterns.value = patterns.value.slice(0, MAX_DISPLAY)
    }
  }
}

onMounted(() => {
  unsubscribe = subscribeToAllEvents(handleEvent)
})

onUnmounted(() => {
  unsubscribe?.()
})

const getIcon = (type: string) => {
  switch (type) {
    case 'exec': return Terminal
    case 'connect': return Globe
    case 'file': return FileText
    default: return Terminal
  }
}
</script>

<template>
  <div class="live-capture">
    <div class="capture-header">
      <h3 class="capture-title">Live Capture</h3>
      <span class="capture-count">{{ patterns.length }} unique patterns</span>
    </div>

    <div class="capture-list">
      <TransitionGroup name="list">
        <div 
          v-for="pattern in patterns" 
          :key="pattern.id"
          class="capture-item"
          :class="`type-${pattern.type}`"
        >
          <Check :size="14" class="item-check" />
          <component :is="getIcon(pattern.type)" :size="14" class="item-icon" />
          <span class="item-text">{{ pattern.description }}</span>
        </div>
      </TransitionGroup>

      <div v-if="patterns.length === 0" class="capture-empty">
        Waiting for patterns...
      </div>
    </div>
  </div>
</template>

<style scoped>
.live-capture {
  background: var(--bg-elevated);
  border-radius: var(--radius-lg);
  border: 1px solid var(--border-subtle);
  overflow: hidden;
}

.capture-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 16px;
  background: var(--bg-surface);
  border-bottom: 1px solid var(--border-subtle);
}

.capture-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-primary);
  margin: 0;
}

.capture-count {
  font-size: 11px;
  color: var(--text-muted);
  font-family: var(--font-mono);
}

.capture-list {
  max-height: 300px;
  overflow-y: auto;
  padding: 8px;
}

.capture-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  border-radius: var(--radius-sm);
  font-size: 12px;
  background: var(--bg-surface);
  margin-bottom: 4px;
}

.capture-item:last-child {
  margin-bottom: 0;
}

.item-check {
  color: var(--status-safe);
  flex-shrink: 0;
}

.item-icon {
  flex-shrink: 0;
}

.type-exec .item-icon { color: var(--status-info); }
.type-connect .item-icon { color: var(--status-warning); }
.type-file .item-icon { color: var(--status-safe); }

.item-text {
  color: var(--text-secondary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-family: var(--font-mono);
}

.capture-empty {
  padding: 24px;
  text-align: center;
  color: var(--text-muted);
  font-size: 12px;
}

/* List animation */
.list-enter-active {
  transition: all 0.3s ease;
}

.list-enter-from {
  opacity: 0;
  transform: translateX(-20px);
}
</style>

