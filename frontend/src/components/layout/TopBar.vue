<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { Brain, Cloud, Sparkles } from 'lucide-vue-next'
import { getAIStatus, type AIStatus } from '../../lib/api'
import DiagnosisModal from '../ai/DiagnosisModal.vue'

const probeStatus = ref<'active' | 'error' | 'starting'>('starting')

// AI state
const aiStatus = ref<AIStatus | null>(null)
const showDiagnosisModal = ref(false)

onMounted(async () => {
  setTimeout(() => {
    if (probeStatus.value === 'starting') {
      probeStatus.value = 'active'
    }
  }, 2000)
  
  // Fetch AI status
  try {
    aiStatus.value = await getAIStatus()
  } catch (e) {
    console.error('Failed to fetch AI status:', e)
  }
})
</script>

<template>
  <header class="topbar">
    <div class="topbar-left">
      <div class="status-indicator" :class="probeStatus">
        <span class="pulse-ring"></span>
        <span class="status-dot"></span>
        <span class="status-text">
          {{ probeStatus === 'active' ? 'Probes Active' : probeStatus === 'error' ? 'Probe Error' : 'Starting...' }}
        </span>
      </div>
    </div>

    <div class="topbar-center">
      <!-- Rate display removed -->
    </div>

    <div class="topbar-right">
      <!-- AI Status & Quick Diagnose -->
      <div v-if="aiStatus?.enabled" class="ai-section">
        <!-- AI Provider Status Badge -->
        <div class="ai-status" :class="aiStatus.status">
          <component 
            :is="aiStatus.isLocal ? Brain : Cloud" 
            :size="14" 
            class="ai-icon"
          />
          <span class="ai-label">{{ aiStatus.isLocal ? 'Local AI' : 'Cloud AI' }}</span>
        </div>
        
        <!-- Quick Diagnose Button -->
        <button 
          class="diagnose-btn" 
          @click="showDiagnosisModal = true"
          :disabled="aiStatus.status !== 'ready'"
          title="Quick one-click system diagnosis"
        >
          <Sparkles :size="16" />
          <span>Quick Diagnose</span>
        </button>
      </div>
    </div>
  </header>
  
  <!-- Quick Diagnosis Modal -->
  <DiagnosisModal 
    :visible="showDiagnosisModal" 
    @close="showDiagnosisModal = false"
  />
</template>

<style scoped>
.topbar {
  height: var(--topbar-height);
  background: var(--bg-surface);
  border-bottom: 1px solid var(--border-subtle);
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 24px;
}

.topbar-left,
.topbar-right {
  display: flex;
  align-items: center;
  gap: 16px;
}

.topbar-center {
  display: flex;
  align-items: center;
}

.status-indicator {
  display: flex;
  align-items: center;
  gap: 10px;
  position: relative;
}

.pulse-ring {
  position: absolute;
  width: 12px;
  height: 12px;
  border-radius: 50%;
  opacity: 0;
}

.status-indicator.active .pulse-ring {
  background: var(--status-safe);
  animation: pulse-ring 2s cubic-bezier(0.215, 0.61, 0.355, 1) infinite;
}

.status-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  background: var(--text-muted);
  position: relative;
  z-index: 1;
}

.status-indicator.active .status-dot {
  background: var(--status-safe);
  box-shadow: var(--glow-safe);
}

.status-indicator.error .status-dot {
  background: var(--status-critical);
  box-shadow: var(--glow-critical);
}

.status-indicator.starting .status-dot {
  background: var(--status-warning);
  animation: blink 1s ease-in-out infinite;
}

.status-text {
  font-size: 13px;
  font-weight: 500;
  color: var(--text-secondary);
}

.status-indicator.active .status-text {
  color: var(--status-safe);
}

/* AI Section Styles */
.ai-section {
  display: flex;
  align-items: center;
  gap: 12px;
}

.ai-status {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 4px 12px;
  background: var(--bg-elevated);
  border-radius: var(--radius-full);
  font-size: 12px;
}

.ai-status.ready .ai-icon {
  color: var(--status-safe);
}

.ai-status.unavailable .ai-icon {
  color: var(--status-warning);
}

.ai-label {
  color: var(--text-secondary);
}

.diagnose-btn {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 14px;
  background: linear-gradient(135deg, var(--accent-primary), #a855f7);
  border: none;
  border-radius: var(--radius-sm);
  color: white;
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s ease;
}

.diagnose-btn:hover:not(:disabled) {
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(168, 85, 247, 0.3);
}

.diagnose-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.diagnose-btn svg {
  animation: sparkle 2s ease-in-out infinite;
}

@keyframes sparkle {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.6; }
}

@keyframes pulse-ring {
  0% {
    transform: scale(0.5);
    opacity: 0.8;
  }

  100% {
    transform: scale(2);
    opacity: 0;
  }
}

@keyframes blink {

  0%,
  100% {
    opacity: 1;
  }

  50% {
    opacity: 0.4;
  }
}
</style>
