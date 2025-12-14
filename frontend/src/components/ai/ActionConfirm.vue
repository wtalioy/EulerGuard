<!-- Action Confirm Component - Phase 4 -->
<script setup lang="ts">
import { computed } from 'vue'
import { AlertTriangle, CheckCircle2, X } from 'lucide-vue-next'
import type { Intent } from '../../composables/useAI'

const props = defineProps<{
  intent: Intent
  visible: boolean
}>()

const emit = defineEmits<{
  confirm: []
  cancel: []
}>()

const isHighConfidence = computed(() => props.intent.confidence > 0.8)
const hasWarnings = computed(() => props.intent.warnings && props.intent.warnings.length > 0)
</script>

<template>
  <Teleport to="body">
    <Transition name="modal">
      <div v-if="visible" class="modal-overlay" @click.self="$emit('cancel')">
        <div class="modal-container">
          <div class="modal-header">
            <h3 class="modal-title">Confirm Action</h3>
            <button class="close-btn" @click="$emit('cancel')">
              <X :size="18" />
            </button>
          </div>

          <div class="modal-body">
            <div class="intent-summary">
              <div class="intent-type">
                <CheckCircle2 v-if="isHighConfidence" :size="20" class="icon-success" />
                <AlertTriangle v-else :size="20" class="icon-warning" />
                <span class="type-label">{{ intent.type }}</span>
                <span class="confidence" :class="{ high: isHighConfidence, low: !isHighConfidence }">
                  {{ Math.round(intent.confidence * 100) }}% confidence
                </span>
              </div>
            </div>

            <div v-if="hasWarnings" class="warnings-section">
              <div class="section-title">
                <AlertTriangle :size="16" />
                <span>Warnings</span>
              </div>
              <ul class="warnings-list">
                <li v-for="(warning, idx) in intent.warnings" :key="idx">{{ warning }}</li>
              </ul>
            </div>

            <div v-if="intent.preview" class="preview-section">
              <div class="section-title">Preview</div>
              <pre class="preview-content">{{ intent.preview.content }}</pre>
            </div>
          </div>

          <div class="modal-footer">
            <button class="btn-secondary" @click="$emit('cancel')">Cancel</button>
            <button class="btn-primary" @click="$emit('confirm')">Confirm</button>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.6);
  backdrop-filter: blur(4px);
  z-index: 10000;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 20px;
}

.modal-container {
  background: var(--bg-surface);
  border: 1px solid var(--border-default);
  border-radius: var(--radius-lg);
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3);
  width: 100%;
  max-width: 600px;
  max-height: 90vh;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.modal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 20px;
  border-bottom: 1px solid var(--border-subtle);
}

.modal-title {
  font-size: 18px;
  font-weight: 600;
  color: var(--text-primary);
  margin: 0;
}

.close-btn {
  padding: 4px;
  background: transparent;
  border: none;
  color: var(--text-secondary);
  cursor: pointer;
  border-radius: var(--radius-sm);
  transition: all 0.15s;
}

.close-btn:hover {
  background: var(--bg-hover);
  color: var(--text-primary);
}

.modal-body {
  flex: 1;
  padding: 20px;
  overflow-y: auto;
}

.intent-summary {
  margin-bottom: 20px;
}

.intent-type {
  display: flex;
  align-items: center;
  gap: 10px;
}

.type-label {
  text-transform: capitalize;
  font-weight: 600;
  color: var(--text-primary);
}

.confidence {
  padding: 4px 10px;
  border-radius: var(--radius-sm);
  font-size: 12px;
  font-weight: 600;
}

.confidence.high {
  background: rgba(34, 197, 94, 0.1);
  color: rgb(34, 197, 94);
}

.confidence.low {
  background: rgba(251, 191, 36, 0.1);
  color: rgb(251, 191, 36);
}

.icon-success {
  color: rgb(34, 197, 94);
}

.icon-warning {
  color: rgb(251, 191, 36);
}

.warnings-section,
.preview-section {
  margin-top: 20px;
  padding-top: 20px;
  border-top: 1px solid var(--border-subtle);
}

.section-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
  font-weight: 600;
  color: var(--text-primary);
  margin-bottom: 12px;
}

.warnings-list {
  list-style: none;
  padding: 0;
  margin: 0;
}

.warnings-list li {
  padding: 10px;
  background: rgba(251, 191, 36, 0.1);
  border: 1px solid rgba(251, 191, 36, 0.2);
  border-radius: var(--radius-md);
  margin-top: 8px;
  font-size: 13px;
  color: var(--text-secondary);
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

.modal-footer {
  display: flex;
  gap: 12px;
  padding: 20px;
  border-top: 1px solid var(--border-subtle);
  justify-content: flex-end;
}

.btn-secondary,
.btn-primary {
  padding: 10px 20px;
  border-radius: var(--radius-md);
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s ease;
  border: none;
}

.btn-secondary {
  background: var(--bg-elevated);
  color: var(--text-secondary);
  border: 1px solid var(--border-subtle);
}

.btn-secondary:hover:not(:disabled) {
  background: var(--bg-hover);
  color: var(--text-primary);
  transform: translateY(-1px);
}

.btn-secondary:active:not(:disabled) {
  transform: translateY(0);
}

.btn-primary {
  background: var(--chart-network);
  color: white;
  box-shadow: 0 2px 8px rgba(59, 130, 246, 0.25);
}

.btn-primary:hover:not(:disabled) {
  background: var(--accent-hover);
  box-shadow: 0 4px 12px rgba(59, 130, 246, 0.35);
  transform: translateY(-1px);
}

.btn-primary:active:not(:disabled) {
  transform: translateY(0);
  box-shadow: 0 2px 6px rgba(59, 130, 246, 0.3);
}

.modal-enter-active,
.modal-leave-active {
  transition: opacity 0.2s;
}

.modal-enter-from,
.modal-leave-to {
  opacity: 0;
}
</style>

