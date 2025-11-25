<script setup lang="ts">
import { X, Terminal, Globe, FileText, Code, Database } from 'lucide-vue-next'
import type { ProbeInfo } from '../../data/probes'

defineProps<{
  probe: ProbeInfo
}>()

defineEmits<{
  close: []
}>()

const getCategoryIcon = (category: string) => {
  switch (category) {
    case 'process': return Terminal
    case 'network': return Globe
    case 'file': return FileText
    default: return Terminal
  }
}
</script>

<template>
  <div class="probe-card-overlay" @click.self="$emit('close')">
    <div class="probe-card">
      <div class="card-header">
        <div class="header-info">
          <div class="probe-icon" :class="probe.category">
            <component :is="getCategoryIcon(probe.category)" :size="20" />
          </div>
          <div class="probe-title">
            <h2 class="title">{{ probe.name }}</h2>
            <code class="tracepoint">{{ probe.tracepoint }}</code>
          </div>
        </div>
        <button class="close-btn" @click="$emit('close')">
          <X :size="20" />
        </button>
      </div>

      <div class="card-content">
        <!-- Description -->
        <section class="card-section">
          <h3 class="section-title">
            <span class="section-icon">📋</span>
            Description
          </h3>
          <p class="description">{{ probe.description }}</p>
        </section>

        <!-- BPF Source Code -->
        <section class="card-section">
          <h3 class="section-title">
            <Code :size="16" class="section-icon" />
            eBPF Source Code
          </h3>
          <div class="code-block">
            <pre><code>{{ probe.sourceCode }}</code></pre>
          </div>
        </section>

        <!-- Kernel Structures -->
        <section class="card-section">
          <h3 class="section-title">
            <Database :size="16" class="section-icon" />
            Kernel Structures Accessed
          </h3>
          <div class="struct-list">
            <div v-for="struct in probe.kernelStructs" :key="struct" class="struct-item">
              <span class="struct-arrow">→</span>
              <code>{{ struct }}</code>
            </div>
          </div>
        </section>
      </div>
    </div>
  </div>
</template>

<style scoped>
.probe-card-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.7);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 100;
  padding: 40px;
  backdrop-filter: blur(4px);
}

.probe-card {
  width: 100%;
  max-width: 700px;
  max-height: calc(100vh - 80px);
  background: var(--bg-surface);
  border-radius: var(--radius-lg);
  border: 1px solid var(--border-default);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  box-shadow: var(--shadow-lg);
}

.card-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  padding: 20px 24px;
  background: var(--bg-elevated);
  border-bottom: 1px solid var(--border-subtle);
}

.header-info {
  display: flex;
  align-items: flex-start;
  gap: 16px;
}

.probe-icon {
  width: 44px;
  height: 44px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: var(--radius-md);
  flex-shrink: 0;
}

.probe-icon.process {
  background: var(--status-info-dim);
  color: var(--status-info);
}

.probe-icon.network {
  background: var(--status-warning-dim);
  color: var(--status-warning);
}

.probe-icon.file {
  background: var(--status-safe-dim);
  color: var(--status-safe);
}

.probe-title {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.title {
  font-size: 18px;
  font-weight: 600;
  color: var(--text-primary);
  margin: 0;
}

.tracepoint {
  font-family: var(--font-mono);
  font-size: 12px;
  color: var(--accent-primary);
  background: var(--bg-void);
  padding: 4px 8px;
  border-radius: var(--radius-sm);
}

.close-btn {
  width: 36px;
  height: 36px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: var(--radius-md);
  color: var(--text-secondary);
  transition: all var(--transition-fast);
  flex-shrink: 0;
}

.close-btn:hover {
  background: var(--bg-overlay);
  color: var(--text-primary);
}

.card-content {
  flex: 1;
  overflow-y: auto;
  padding: 24px;
}

.card-section {
  margin-bottom: 24px;
}

.card-section:last-child {
  margin-bottom: 0;
}

.section-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  font-weight: 600;
  color: var(--text-secondary);
  margin: 0 0 12px 0;
}

.section-icon {
  color: var(--text-muted);
}

.description {
  font-size: 14px;
  line-height: 1.7;
  color: var(--text-secondary);
  margin: 0;
}

.code-block {
  background: var(--bg-void);
  border-radius: var(--radius-md);
  overflow-x: auto;
}

.code-block pre {
  margin: 0;
  padding: 16px;
}

.code-block code {
  font-family: var(--font-mono);
  font-size: 12px;
  line-height: 1.6;
  color: var(--text-secondary);
  white-space: pre;
}

.struct-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.struct-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 14px;
  background: var(--bg-elevated);
  border-radius: var(--radius-md);
}

.struct-arrow {
  color: var(--accent-primary);
  font-weight: bold;
}

.struct-item code {
  font-family: var(--font-mono);
  font-size: 13px;
  color: var(--text-primary);
}
</style>

