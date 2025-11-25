<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { RecycleScroller } from 'vue-virtual-scroller'
import 'vue-virtual-scroller/dist/vue-virtual-scroller.css'
import { Radio } from 'lucide-vue-next'
import StreamControls from '../components/stream/StreamControls.vue'
import EventRow from '../components/stream/EventRow.vue'
import EventDetailsPanel from '../components/stream/EventDetailsPanel.vue'
import { subscribeToAllEvents, type StreamEvent } from '../lib/api'

// Event buffer with max size
const MAX_BUFFER_SIZE = 10000
const events = ref<StreamEvent[]>([])
const isPaused = ref(false)
const selectedEvent = ref<StreamEvent | null>(null)

// Filters
const filters = ref({
  exec: true,
  connect: true,
  file: true,
  containerOnly: false
})

// Filtered events
const filteredEvents = computed(() => {
  return events.value.filter(event => {
    // Type filter
    if (!filters.value[event.type]) return false
    // Container filter
    if (filters.value.containerOnly && !event.inContainer) return false
    return true
  })
})

// Event handling
let unsubscribe: (() => void) | null = null

const handleEvent = (event: StreamEvent) => {
  if (isPaused.value) return

  events.value.unshift(event)
  
  // Trim buffer if too large
  if (events.value.length > MAX_BUFFER_SIZE) {
    events.value = events.value.slice(0, MAX_BUFFER_SIZE)
  }
}

const togglePause = () => {
  isPaused.value = !isPaused.value
}

const clearEvents = () => {
  events.value = []
  selectedEvent.value = null
}

const selectEvent = (event: StreamEvent) => {
  selectedEvent.value = event
}

const closeDetails = () => {
  selectedEvent.value = null
}

const huntSimilar = (event: StreamEvent) => {
  // Filter to show similar events (same type + same process or destination)
  if (event.type === 'exec') {
    // Could implement search/filter logic here
    console.log('Hunt similar exec:', event.comm)
  }
}

onMounted(() => {
  unsubscribe = subscribeToAllEvents(handleEvent)
})

onUnmounted(() => {
  unsubscribe?.()
})
</script>

<template>
  <div class="live-stream">
    <!-- Header -->
    <div class="page-header">
      <div class="header-content">
        <h1 class="page-title">
          <Radio :size="24" class="title-icon" :class="{ pulsing: !isPaused }" />
          Live Stream
        </h1>
        <span class="page-subtitle">Real-time event monitoring</span>
      </div>
    </div>

    <!-- Main Content -->
    <div class="stream-container">
      <!-- Controls -->
      <StreamControls
        :is-paused="isPaused"
        :event-count="events.length"
        :filters="filters"
        @toggle-pause="togglePause"
        @clear="clearEvents"
        @update:filters="filters = $event"
      />

      <!-- Table Header -->
      <div class="table-header">
        <span class="col-time">Time</span>
        <span class="col-type">Type</span>
        <span class="col-process">Process</span>
        <span class="col-details">Details</span>
        <span class="col-container"></span>
      </div>

      <!-- Event List with Virtual Scrolling -->
      <div class="table-wrapper" :class="{ 'has-panel': selectedEvent }">
        <RecycleScroller
          v-if="filteredEvents.length > 0"
          class="scroller"
          :items="filteredEvents"
          :item-size="41"
          key-field="timestamp"
          v-slot="{ item }"
        >
          <EventRow
            :event="item"
            :is-selected="selectedEvent?.timestamp === item.timestamp && selectedEvent?.pid === item.pid"
            @select="selectEvent"
          />
        </RecycleScroller>

        <!-- Empty State -->
        <div v-else class="empty-state">
          <div class="empty-icon">📡</div>
          <div class="empty-title">
            {{ events.length === 0 ? 'Waiting for events...' : 'No matching events' }}
          </div>
          <div class="empty-description">
            {{ events.length === 0 
              ? 'Events will appear here as they are captured by eBPF probes'
              : 'Try adjusting your filters to see more events' 
            }}
          </div>
        </div>

        <!-- Details Panel -->
        <EventDetailsPanel
          :event="selectedEvent"
          @close="closeDetails"
          @hunt-similar="huntSimilar"
        />
      </div>
    </div>
  </div>
</template>

<style scoped>
.live-stream {
  height: 100%;
  display: flex;
  flex-direction: column;
  gap: 20px;
}

/* Header */
.page-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
}

.header-content {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.page-title {
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 24px;
  font-weight: 700;
  color: var(--text-primary);
  margin: 0;
}

.title-icon {
  color: var(--text-muted);
}

.title-icon.pulsing {
  color: var(--status-safe);
  animation: pulse 2s ease-in-out infinite;
}

@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.5; }
}

.page-subtitle {
  font-size: 14px;
  color: var(--text-muted);
}

/* Stream Container */
.stream-container {
  flex: 1;
  display: flex;
  flex-direction: column;
  background: var(--bg-surface);
  border-radius: var(--radius-lg);
  border: 1px solid var(--border-subtle);
  overflow: hidden;
  min-height: 0;
}

/* Table Header */
.table-header {
  display: grid;
  grid-template-columns: 100px 32px 120px 1fr 28px;
  align-items: center;
  gap: 12px;
  padding: 10px 16px;
  background: var(--bg-void);
  border-bottom: 1px solid var(--border-subtle);
  font-size: 11px;
  font-weight: 600;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

/* Table Wrapper */
.table-wrapper {
  flex: 1;
  position: relative;
  overflow: hidden;
  min-height: 0;
}

.table-wrapper.has-panel {
  margin-right: 380px;
}

.scroller {
  height: 100%;
}

/* Empty State */
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  padding: 60px 40px;
  text-align: center;
}

.empty-icon {
  font-size: 48px;
  margin-bottom: 16px;
  opacity: 0.5;
}

.empty-title {
  font-size: 16px;
  font-weight: 600;
  color: var(--text-primary);
  margin-bottom: 8px;
}

.empty-description {
  font-size: 14px;
  color: var(--text-muted);
  max-width: 300px;
}

/* Virtual scroller overrides */
:deep(.vue-recycle-scroller__item-wrapper) {
  overflow: visible;
}
</style>
