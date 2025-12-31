<!-- Sentinel Preview Component - Updated to use global card styles -->
<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { Sparkles, ArrowRight } from 'lucide-vue-next'
import { useRouter } from 'vue-router'
import { useSentinel } from '../../composables/useSentinel'
import InsightCard from '../sentinel/InsightCard.vue'

const router = useRouter()
const { insights, fetchInsights } = useSentinel()

const recentInsights = ref(insights.value.slice(0, 3))

onMounted(async () => {
  await fetchInsights()
  recentInsights.value = insights.value.slice(0, 3)
})

const goToSentinel = () => {
  router.push('/sentinel')
}
</script>

<template>
  <div class="card-base">
    <div class="card-header-base">
      <div class="header-left">
        <Sparkles :size="20" />
        <h3>AI Sentinel</h3>
      </div>
      <button class="btn btn-secondary" @click="goToSentinel">
        <span>View All</span>
        <ArrowRight :size="14" />
      </button>
    </div>

    <div class="card-content-base">
      <div v-if="recentInsights.length === 0" class="empty-state">
        <Sparkles :size="32" />
        <p>No insights yet</p>
        <p class="subtitle">AI Sentinel is monitoring your system</p>
      </div>

      <div v-else class="insights-preview">
        <InsightCard
          v-for="insight in recentInsights"
          :key="insight.id"
          :insight="insight"
          @action="() => {}"
          @askAI="() => {}"
        />
      </div>
    </div>
  </div>
</template>

<style scoped>
.header-left {
  display: flex;
  align-items: center;
  gap: 12px;
}

.card-header-base h3 {
  font-size: 18px;
  font-weight: 600;
  color: var(--text-primary);
  margin: 0;
}

.empty-state {
  padding: 40px 20px; /* Adjusted padding */
  text-align: center;
  color: var(--text-muted);
}

.empty-state p {
  margin-top: 16px;
  font-size: 14px;
}

.subtitle {
  font-size: 12px;
  color: var(--text-muted);
  margin-top: 4px;
}

.insights-preview {
  display: flex;
  flex-direction: column;
  gap: 16px;
}
</style>