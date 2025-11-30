<script setup lang="ts">
import { ref, onMounted } from 'vue'

const probeStatus = ref<'active' | 'error' | 'starting'>('starting')

onMounted(() => {
  setTimeout(() => {
    if (probeStatus.value === 'starting') {
      probeStatus.value = 'active'
    }
  }, 2000)
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
      <!-- Placeholder for future actions -->
    </div>
  </header>
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
