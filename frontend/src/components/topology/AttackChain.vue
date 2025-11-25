<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { VueFlow, useVueFlow } from '@vue-flow/core'
import { Background } from '@vue-flow/background'
import type { Node, Edge } from '@vue-flow/core'
import ProcessNode from './ProcessNode.vue'
import type { ProcessInfo, Alert } from '../../lib/api'
import { Camera, Download, Copy, RefreshCw } from 'lucide-vue-next'

const props = defineProps<{
  ancestors: ProcessInfo[]
  alert: Alert | null
  loading?: boolean
}>()

const emit = defineEmits<{
  refresh: []
}>()

// Vue Flow setup
const { fitView } = useVueFlow()

// Node dimensions for layout
const NODE_WIDTH = 180
const NODE_HEIGHT = 100
const NODE_SPACING_Y = 40

// Convert ancestors to Vue Flow nodes and edges
const elements = computed(() => {
  if (!props.ancestors.length) {
    return { nodes: [], edges: [] }
  }

  const nodes: Node[] = []
  const edges: Edge[] = []

  // Ancestors come from oldest (root) to newest (alert source)
  // We want to display from top (root) to bottom (alert source)
  const sortedAncestors = [...props.ancestors]
  
  // Calculate center X position
  const centerX = 200

  sortedAncestors.forEach((proc, index) => {
    const isAlertSource = index === sortedAncestors.length - 1 && props.alert
    
    nodes.push({
      id: `proc-${proc.pid}`,
      type: 'process',
      position: { 
        x: centerX - NODE_WIDTH / 2, 
        y: index * (NODE_HEIGHT + NODE_SPACING_Y) 
      },
      data: {
        pid: proc.pid,
        ppid: proc.ppid,
        comm: proc.comm,
        timestamp: proc.timestamp,
        isTarget: isAlertSource,
        isAlertSource: isAlertSource,
        inContainer: proc.cgroupId !== '1' && proc.cgroupId !== '0',
        severity: isAlertSource ? props.alert?.severity as 'high' | 'warning' | 'info' : undefined
      }
    })

    // Create edge to next node (child)
    if (index < sortedAncestors.length - 1) {
      const child = sortedAncestors[index + 1]
      edges.push({
        id: `edge-${proc.pid}-${child.pid}`,
        source: `proc-${proc.pid}`,
        target: `proc-${child.pid}`,
        type: 'smoothstep',
        animated: index === sortedAncestors.length - 2, // Animate last edge
        style: {
          stroke: index === sortedAncestors.length - 2 
            ? 'var(--status-critical)' 
            : 'var(--border-default)',
          strokeWidth: 2
        }
      })
    }
  })

  return { nodes, edges }
})

const nodes = computed(() => elements.value.nodes)
const edges = computed(() => elements.value.edges)

// Auto-fit view when data changes
watch(() => props.ancestors, () => {
  setTimeout(() => {
    fitView({ padding: 0.3 })
  }, 100)
}, { deep: true })

// Export functions
const copyChainAsText = () => {
  if (!props.ancestors.length) return
  
  const text = props.ancestors
    .map((p, i) => {
      const indent = '  '.repeat(i)
      const arrow = i > 0 ? '└─ ' : ''
      return `${indent}${arrow}${p.comm} (PID: ${p.pid})`
    })
    .join('\n')
  
  navigator.clipboard.writeText(text)
}

const copyChainAsJson = () => {
  if (!props.ancestors.length) return
  navigator.clipboard.writeText(JSON.stringify(props.ancestors, null, 2))
}
</script>

<template>
  <div class="attack-chain">
    <!-- Toolbar -->
    <div class="chain-toolbar">
      <div class="toolbar-title">
        <span class="title-text">Attack Chain</span>
        <span v-if="ancestors.length" class="title-count">{{ ancestors.length }} processes</span>
      </div>
      <div class="toolbar-actions">
        <button class="toolbar-btn" @click="fitView({ padding: 0.3 })" title="Fit View">
          <Camera :size="16" />
        </button>
        <button class="toolbar-btn" @click="copyChainAsText" title="Copy as Text">
          <Copy :size="16" />
        </button>
        <button class="toolbar-btn" @click="copyChainAsJson" title="Copy as JSON">
          <Download :size="16" />
        </button>
        <button class="toolbar-btn" @click="$emit('refresh')" title="Refresh">
          <RefreshCw :size="16" :class="{ spinning: loading }" />
        </button>
      </div>
    </div>

    <!-- Vue Flow Canvas -->
    <div class="chain-canvas">
      <!-- Loading State -->
      <div v-if="loading" class="chain-loading">
        <div class="loading-spinner"></div>
        <span>Loading ancestry chain...</span>
      </div>

      <!-- Empty State -->
      <div v-else-if="!alert" class="chain-empty">
        <div class="empty-icon">🔍</div>
        <div class="empty-title">Select an Alert</div>
        <div class="empty-description">
          Click on an alert to view its process ancestry chain
        </div>
      </div>

      <!-- No Ancestors -->
      <div v-else-if="!ancestors.length" class="chain-empty">
        <div class="empty-icon">🌲</div>
        <div class="empty-title">No Ancestry Data</div>
        <div class="empty-description">
          Process ancestry information is not available for this alert
        </div>
      </div>

      <!-- Vue Flow Graph -->
      <VueFlow
        v-else
        :nodes="nodes"
        :edges="edges"
        :default-viewport="{ x: 0, y: 0, zoom: 1 }"
        :min-zoom="0.3"
        :max-zoom="2"
        fit-view-on-init
        class="vue-flow-container"
      >
        <Background pattern-color="rgba(255, 255, 255, 0.03)" :gap="20" />
        <template #node-process="nodeProps">
          <ProcessNode v-bind="nodeProps" />
        </template>
      </VueFlow>
    </div>

    <!-- Alert Details Footer -->
    <div v-if="alert" class="chain-footer">
      <div class="footer-label">Alert:</div>
      <div class="footer-value">{{ alert.ruleName }}</div>
      <div class="footer-separator">|</div>
      <div class="footer-label">Severity:</div>
      <div class="footer-value" :class="`severity-${alert.severity}`">
        {{ alert.severity.toUpperCase() }}
      </div>
    </div>
  </div>
</template>

<style scoped>
.attack-chain {
  display: flex;
  flex-direction: column;
  height: 100%;
  background: var(--bg-surface);
  border-radius: var(--radius-lg);
  border: 1px solid var(--border-subtle);
  overflow: hidden;
}

.chain-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 16px;
  border-bottom: 1px solid var(--border-subtle);
  background: var(--bg-elevated);
}

.toolbar-title {
  display: flex;
  align-items: center;
  gap: 12px;
}

.title-text {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-primary);
}

.title-count {
  font-size: 12px;
  color: var(--text-muted);
  font-family: var(--font-mono);
}

.toolbar-actions {
  display: flex;
  align-items: center;
  gap: 4px;
}

.toolbar-btn {
  width: 32px;
  height: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: var(--radius-md);
  color: var(--text-secondary);
  transition: all var(--transition-fast);
}

.toolbar-btn:hover {
  background: var(--bg-overlay);
  color: var(--text-primary);
}

.toolbar-btn .spinning {
  animation: spin 1s linear infinite;
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

.chain-canvas {
  flex: 1;
  position: relative;
  min-height: 300px;
}

.vue-flow-container {
  width: 100%;
  height: 100%;
}

/* Loading State */
.chain-loading {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 16px;
  color: var(--text-muted);
}

.loading-spinner {
  width: 32px;
  height: 32px;
  border: 3px solid var(--border-subtle);
  border-top-color: var(--accent-primary);
  border-radius: 50%;
  animation: spin 1s linear infinite;
}

/* Empty State */
.chain-empty {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  padding: 40px;
  text-align: center;
}

.empty-icon {
  font-size: 48px;
  opacity: 0.5;
}

.empty-title {
  font-size: 16px;
  font-weight: 600;
  color: var(--text-primary);
}

.empty-description {
  font-size: 14px;
  color: var(--text-muted);
  max-width: 280px;
}

/* Footer */
.chain-footer {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 16px;
  border-top: 1px solid var(--border-subtle);
  background: var(--bg-elevated);
  font-size: 12px;
}

.footer-label {
  color: var(--text-muted);
}

.footer-value {
  font-family: var(--font-mono);
  color: var(--text-secondary);
}

.footer-value.severity-high { color: var(--status-critical); }
.footer-value.severity-warning { color: var(--status-warning); }
.footer-value.severity-info { color: var(--status-info); }

.footer-separator {
  color: var(--border-default);
  margin: 0 4px;
}

/* Vue Flow overrides */
:deep(.vue-flow__edge-path) {
  stroke-width: 2;
}

:deep(.vue-flow__edge.animated path) {
  stroke-dasharray: 5;
  animation: dash 0.5s linear infinite;
}

@keyframes dash {
  to {
    stroke-dashoffset: -10;
  }
}

:deep(.vue-flow__background) {
  background: var(--bg-void);
}

:deep(.vue-flow__controls) {
  background: var(--bg-elevated);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
}

:deep(.vue-flow__controls button) {
  background: transparent;
  border: none;
  color: var(--text-secondary);
}

:deep(.vue-flow__controls button:hover) {
  background: var(--bg-overlay);
}
</style>

