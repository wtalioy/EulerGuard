<script setup lang="ts">
import { Terminal, Globe, FileText, ArrowDown, Database } from 'lucide-vue-next'
import { probes, type ProbeInfo } from '../../data/probes'

defineEmits<{
  selectProbe: [probe: ProbeInfo]
}>()

const getProbeIcon = (category: string) => {
  switch (category) {
    case 'process': return Terminal
    case 'network': return Globe
    case 'file': return FileText
    default: return Terminal
  }
}
</script>

<template>
  <div class="architecture-diagram">
    <!-- User Space -->
    <div class="space-section user-space">
      <div class="space-label">USER SPACE</div>
      <div class="processes">
        <div class="process-box">
          <Terminal :size="18" />
          <span>bash</span>
          <code class="pid">PID: 1234</code>
        </div>
        <div class="process-box">
          <Terminal :size="18" />
          <span>python</span>
          <code class="pid">PID: 5678</code>
        </div>
        <div class="process-box">
          <Globe :size="18" />
          <span>nc</span>
          <code class="pid">PID: 9012</code>
        </div>
      </div>
      
      <!-- Syscall arrows -->
      <div class="syscall-arrows">
        <div class="syscall-arrow">
          <ArrowDown :size="16" />
          <span>execve()</span>
        </div>
        <div class="syscall-arrow">
          <ArrowDown :size="16" />
          <span>open()</span>
        </div>
        <div class="syscall-arrow">
          <ArrowDown :size="16" />
          <span>connect()</span>
        </div>
      </div>
    </div>

    <!-- Kernel Boundary -->
    <div class="kernel-boundary">
      <div class="boundary-line"></div>
      <span class="boundary-label">System Call Interface</span>
      <div class="boundary-line"></div>
    </div>

    <!-- Kernel Space -->
    <div class="space-section kernel-space">
      <div class="space-label">KERNEL SPACE</div>
      
      <!-- eBPF Probes -->
      <div class="probes-row">
        <div 
          v-for="probe in probes"
          :key="probe.id"
          class="probe-node"
          :class="probe.category"
          @click="$emit('selectProbe', probe)"
        >
          <div class="probe-indicator"></div>
          <component :is="getProbeIcon(probe.category)" :size="16" />
          <span class="probe-name">{{ probe.name.replace(' Monitor', '') }}</span>
          <code class="probe-tracepoint">{{ probe.tracepoint.split('/').pop() }}</code>
        </div>
      </div>

      <!-- Arrows to Ring Buffer -->
      <div class="buffer-arrows">
        <div class="arrow-line" v-for="i in 3" :key="i"></div>
      </div>

      <!-- Ring Buffer -->
      <div class="ring-buffer">
        <Database :size="20" />
        <div class="buffer-info">
          <span class="buffer-title">RING BUFFER</span>
          <span class="buffer-size">256KB</span>
        </div>
        <div class="buffer-visual">
          <div class="buffer-fill" style="width: 65%"></div>
        </div>
        <span class="buffer-percent">65% full</span>
      </div>
    </div>

    <!-- Legend -->
    <div class="diagram-legend">
      <span class="legend-title">Click on a probe to learn more</span>
      <div class="legend-items">
        <div class="legend-item">
          <div class="legend-dot process"></div>
          <span>Process Events</span>
        </div>
        <div class="legend-item">
          <div class="legend-dot file"></div>
          <span>File Events</span>
        </div>
        <div class="legend-item">
          <div class="legend-dot network"></div>
          <span>Network Events</span>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.architecture-diagram {
  background: var(--bg-surface);
  border-radius: var(--radius-lg);
  border: 1px solid var(--border-subtle);
  padding: 32px;
  display: flex;
  flex-direction: column;
  gap: 24px;
}

/* Space Sections */
.space-section {
  padding: 24px;
  border-radius: var(--radius-lg);
  position: relative;
}

.user-space {
  background: linear-gradient(135deg, var(--bg-elevated) 0%, var(--bg-surface) 100%);
  border: 1px solid var(--border-subtle);
}

.kernel-space {
  background: linear-gradient(135deg, rgba(96, 165, 250, 0.05) 0%, rgba(139, 92, 246, 0.05) 100%);
  border: 2px solid var(--accent-primary);
}

.space-label {
  position: absolute;
  top: -10px;
  left: 20px;
  padding: 2px 12px;
  background: var(--bg-surface);
  font-size: 11px;
  font-weight: 600;
  color: var(--text-muted);
  letter-spacing: 0.1em;
}

.kernel-space .space-label {
  color: var(--accent-primary);
}

/* Processes */
.processes {
  display: flex;
  justify-content: center;
  gap: 32px;
  margin-bottom: 20px;
}

.process-box {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  padding: 16px 24px;
  background: var(--bg-overlay);
  border-radius: var(--radius-md);
  border: 1px solid var(--border-subtle);
  color: var(--text-secondary);
}

.process-box span {
  font-size: 13px;
  font-weight: 500;
  color: var(--text-primary);
}

.process-box .pid {
  font-family: var(--font-mono);
  font-size: 10px;
  color: var(--text-muted);
  background: var(--bg-void);
  padding: 2px 6px;
  border-radius: var(--radius-sm);
}

/* Syscall Arrows */
.syscall-arrows {
  display: flex;
  justify-content: center;
  gap: 80px;
}

.syscall-arrow {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
  color: var(--text-muted);
}

.syscall-arrow span {
  font-family: var(--font-mono);
  font-size: 11px;
}

/* Kernel Boundary */
.kernel-boundary {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 0 20px;
}

.boundary-line {
  flex: 1;
  height: 2px;
  background: linear-gradient(90deg, transparent, var(--accent-primary), transparent);
}

.boundary-label {
  font-size: 11px;
  font-weight: 500;
  color: var(--accent-primary);
  white-space: nowrap;
}

/* Probes Row */
.probes-row {
  display: flex;
  justify-content: center;
  gap: 24px;
  margin-bottom: 24px;
}

.probe-node {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  padding: 16px 20px;
  background: var(--bg-elevated);
  border-radius: var(--radius-md);
  border: 1px solid var(--border-default);
  cursor: pointer;
  transition: all var(--transition-fast);
  position: relative;
}

.probe-node:hover {
  transform: translateY(-2px);
  box-shadow: var(--shadow-md);
}

.probe-indicator {
  position: absolute;
  top: 8px;
  right: 8px;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  animation: pulse-indicator 2s ease-in-out infinite;
}

.probe-node.process .probe-indicator {
  background: var(--status-info);
  box-shadow: 0 0 8px var(--status-info);
}

.probe-node.file .probe-indicator {
  background: var(--status-safe);
  box-shadow: 0 0 8px var(--status-safe);
}

.probe-node.network .probe-indicator {
  background: var(--status-warning);
  box-shadow: 0 0 8px var(--status-warning);
}

@keyframes pulse-indicator {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.5; }
}

.probe-node.process { border-color: var(--status-info); }
.probe-node.file { border-color: var(--status-safe); }
.probe-node.network { border-color: var(--status-warning); }

.probe-node.process svg { color: var(--status-info); }
.probe-node.file svg { color: var(--status-safe); }
.probe-node.network svg { color: var(--status-warning); }

.probe-name {
  font-size: 12px;
  font-weight: 500;
  color: var(--text-primary);
  text-align: center;
}

.probe-tracepoint {
  font-family: var(--font-mono);
  font-size: 10px;
  color: var(--text-muted);
  background: var(--bg-void);
  padding: 2px 6px;
  border-radius: var(--radius-sm);
}

/* Buffer Arrows */
.buffer-arrows {
  display: flex;
  justify-content: center;
  gap: 100px;
  margin-bottom: 24px;
}

.arrow-line {
  width: 2px;
  height: 30px;
  background: linear-gradient(to bottom, var(--text-muted), var(--accent-primary));
  position: relative;
}

.arrow-line::after {
  content: '';
  position: absolute;
  bottom: -4px;
  left: -3px;
  border-left: 4px solid transparent;
  border-right: 4px solid transparent;
  border-top: 6px solid var(--accent-primary);
}

/* Ring Buffer */
.ring-buffer {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 16px 24px;
  background: var(--bg-overlay);
  border-radius: var(--radius-md);
  border: 1px solid var(--accent-primary);
  max-width: 400px;
  margin: 0 auto;
}

.ring-buffer svg {
  color: var(--accent-primary);
  flex-shrink: 0;
}

.buffer-info {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.buffer-title {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-primary);
}

.buffer-size {
  font-size: 10px;
  font-family: var(--font-mono);
  color: var(--text-muted);
}

.buffer-visual {
  flex: 1;
  height: 8px;
  background: var(--bg-void);
  border-radius: var(--radius-full);
  overflow: hidden;
}

.buffer-fill {
  height: 100%;
  background: linear-gradient(90deg, var(--status-safe), var(--accent-primary));
  border-radius: var(--radius-full);
}

.buffer-percent {
  font-size: 11px;
  font-family: var(--font-mono);
  color: var(--text-muted);
  flex-shrink: 0;
}

/* Legend */
.diagram-legend {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  padding-top: 16px;
  border-top: 1px solid var(--border-subtle);
}

.legend-title {
  font-size: 12px;
  color: var(--text-muted);
}

.legend-items {
  display: flex;
  gap: 24px;
}

.legend-item {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  color: var(--text-secondary);
}

.legend-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
}

.legend-dot.process { background: var(--status-info); }
.legend-dot.file { background: var(--status-safe); }
.legend-dot.network { background: var(--status-warning); }
</style>

