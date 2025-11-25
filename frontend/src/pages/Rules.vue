<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { FileCode, Terminal, Globe, FileText, RefreshCw, Search, Filter } from 'lucide-vue-next'
import RuleCard from '../components/rules/RuleCard.vue'
import { getRules, type DetectionRule } from '../lib/api'

const rules = ref<DetectionRule[]>([])
const loading = ref(true)
const searchQuery = ref('')
const filterType = ref<string>('all')
const filterSeverity = ref<string>('all')

// Fetch rules
const fetchRules = async () => {
  loading.value = true
  try {
    rules.value = await getRules()
  } catch (e) {
    console.error('Failed to fetch rules:', e)
    rules.value = []
  } finally {
    loading.value = false
  }
}

// Filtered rules
const filteredRules = computed(() => {
  let result = rules.value

  // Type filter
  if (filterType.value !== 'all') {
    result = result.filter(r => r.type === filterType.value)
  }

  // Severity filter
  if (filterSeverity.value !== 'all') {
    result = result.filter(r => r.severity === filterSeverity.value)
  }

  // Search filter
  if (searchQuery.value.trim()) {
    const query = searchQuery.value.toLowerCase()
    result = result.filter(r =>
      r.name.toLowerCase().includes(query) ||
      r.description.toLowerCase().includes(query)
    )
  }

  return result
})

// Grouped rules by type
const groupedRules = computed(() => {
  const groups: Record<string, DetectionRule[]> = {
    exec: [],
    file: [],
    connect: []
  }
  
  filteredRules.value.forEach(rule => {
    if (groups[rule.type]) {
      groups[rule.type].push(rule)
    }
  })

  return groups
})

// Stats
const stats = computed(() => ({
  total: rules.value.length,
  exec: rules.value.filter(r => r.type === 'exec').length,
  file: rules.value.filter(r => r.type === 'file').length,
  connect: rules.value.filter(r => r.type === 'connect').length,
}))

onMounted(fetchRules)
</script>

<template>
  <div class="rules-page">
    <!-- Header -->
    <div class="page-header">
      <div class="header-content">
        <h1 class="page-title">
          <FileCode :size="24" class="title-icon" />
          Detection Rules
        </h1>
        <span class="page-subtitle">Manage security detection rules</span>
      </div>
      <div class="header-stats">
        <div class="stat-item">
          <Terminal :size="14" class="stat-icon exec" />
          <span class="stat-value">{{ stats.exec }}</span>
          <span class="stat-label">Exec</span>
        </div>
        <div class="stat-item">
          <FileText :size="14" class="stat-icon file" />
          <span class="stat-value">{{ stats.file }}</span>
          <span class="stat-label">File</span>
        </div>
        <div class="stat-item">
          <Globe :size="14" class="stat-icon connect" />
          <span class="stat-value">{{ stats.connect }}</span>
          <span class="stat-label">Network</span>
        </div>
      </div>
    </div>

    <!-- Filters -->
    <div class="filters-bar">
      <div class="search-box">
        <Search :size="16" class="search-icon" />
        <input
          v-model="searchQuery"
          type="text"
          placeholder="Search rules..."
          class="search-input"
        />
      </div>

      <div class="filter-group">
        <Filter :size="14" class="filter-icon" />
        <select v-model="filterType" class="filter-select">
          <option value="all">All Types</option>
          <option value="exec">Exec</option>
          <option value="file">File</option>
          <option value="connect">Network</option>
        </select>

        <select v-model="filterSeverity" class="filter-select">
          <option value="all">All Severity</option>
          <option value="high">High</option>
          <option value="warning">Warning</option>
          <option value="info">Info</option>
        </select>
      </div>

      <button class="refresh-btn" @click="fetchRules" :disabled="loading">
        <RefreshCw :size="16" :class="{ spinning: loading }" />
        Refresh
      </button>
    </div>

    <!-- Content -->
    <div class="rules-content">
      <!-- Loading State -->
      <div v-if="loading" class="loading-state">
        <div class="loading-spinner"></div>
        <span>Loading rules...</span>
      </div>

      <!-- Empty State -->
      <div v-else-if="rules.length === 0" class="empty-state">
        <div class="empty-icon">📝</div>
        <div class="empty-title">No Rules Loaded</div>
        <div class="empty-description">
          No detection rules have been loaded. Add rules to your rules.yaml file and restart the application.
        </div>
      </div>

      <!-- No Matches -->
      <div v-else-if="filteredRules.length === 0" class="empty-state">
        <div class="empty-icon">🔍</div>
        <div class="empty-title">No Matching Rules</div>
        <div class="empty-description">
          Try adjusting your search or filters to find rules.
        </div>
      </div>

      <!-- Rule Groups -->
      <template v-else>
        <!-- Exec Rules -->
        <div v-if="groupedRules.exec.length > 0" class="rule-group">
          <div class="group-header">
            <div class="group-icon exec">
              <Terminal :size="16" />
            </div>
            <h2 class="group-title">Process Execution Rules</h2>
            <span class="group-count">{{ groupedRules.exec.length }}</span>
          </div>
          <div class="group-content">
            <RuleCard
              v-for="rule in groupedRules.exec"
              :key="rule.name"
              :rule="rule"
            />
          </div>
        </div>

        <!-- File Rules -->
        <div v-if="groupedRules.file.length > 0" class="rule-group">
          <div class="group-header">
            <div class="group-icon file">
              <FileText :size="16" />
            </div>
            <h2 class="group-title">File Access Rules</h2>
            <span class="group-count">{{ groupedRules.file.length }}</span>
          </div>
          <div class="group-content">
            <RuleCard
              v-for="rule in groupedRules.file"
              :key="rule.name"
              :rule="rule"
            />
          </div>
        </div>

        <!-- Network Rules -->
        <div v-if="groupedRules.connect.length > 0" class="rule-group">
          <div class="group-header">
            <div class="group-icon connect">
              <Globe :size="16" />
            </div>
            <h2 class="group-title">Network Connection Rules</h2>
            <span class="group-count">{{ groupedRules.connect.length }}</span>
          </div>
          <div class="group-content">
            <RuleCard
              v-for="rule in groupedRules.connect"
              :key="rule.name"
              :rule="rule"
            />
          </div>
        </div>
      </template>
    </div>
  </div>
</template>

<style scoped>
.rules-page {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

/* Header */
.page-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 16px;
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
  color: var(--accent-primary);
}

.page-subtitle {
  font-size: 14px;
  color: var(--text-muted);
}

.header-stats {
  display: flex;
  gap: 16px;
}

.stat-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 16px;
  background: var(--bg-elevated);
  border-radius: var(--radius-md);
  border: 1px solid var(--border-subtle);
}

.stat-icon.exec { color: var(--status-info); }
.stat-icon.file { color: var(--status-safe); }
.stat-icon.connect { color: var(--status-warning); }

.stat-value {
  font-size: 16px;
  font-weight: 700;
  font-family: var(--font-mono);
  color: var(--text-primary);
}

.stat-label {
  font-size: 12px;
  color: var(--text-muted);
}

/* Filters */
.filters-bar {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 16px;
  background: var(--bg-surface);
  border-radius: var(--radius-lg);
  border: 1px solid var(--border-subtle);
  flex-wrap: wrap;
}

.search-box {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  background: var(--bg-elevated);
  border-radius: var(--radius-md);
  border: 1px solid var(--border-subtle);
  flex: 1;
  min-width: 200px;
}

.search-box:focus-within {
  border-color: var(--border-focus);
}

.search-icon {
  color: var(--text-muted);
  flex-shrink: 0;
}

.search-input {
  flex: 1;
  background: transparent;
  border: none;
  color: var(--text-primary);
  font-size: 13px;
  outline: none;
}

.search-input::placeholder {
  color: var(--text-muted);
}

.filter-group {
  display: flex;
  align-items: center;
  gap: 8px;
}

.filter-icon {
  color: var(--text-muted);
}

.filter-select {
  padding: 8px 12px;
  background: var(--bg-elevated);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  color: var(--text-primary);
  font-size: 12px;
  cursor: pointer;
}

.filter-select:focus {
  border-color: var(--border-focus);
  outline: none;
}

.refresh-btn {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 16px;
  background: var(--bg-elevated);
  border-radius: var(--radius-md);
  font-size: 12px;
  font-weight: 500;
  color: var(--text-secondary);
  transition: all var(--transition-fast);
}

.refresh-btn:hover:not(:disabled) {
  background: var(--bg-overlay);
  color: var(--text-primary);
}

.refresh-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.refresh-btn .spinning {
  animation: spin 1s linear infinite;
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

/* Content */
.rules-content {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

/* Loading & Empty States */
.loading-state,
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 80px 40px;
  text-align: center;
}

.loading-spinner {
  width: 32px;
  height: 32px;
  border: 3px solid var(--border-subtle);
  border-top-color: var(--accent-primary);
  border-radius: 50%;
  animation: spin 1s linear infinite;
  margin-bottom: 16px;
}

.loading-state span {
  color: var(--text-muted);
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
  max-width: 400px;
}

/* Rule Groups */
.rule-group {
  background: var(--bg-surface);
  border-radius: var(--radius-lg);
  border: 1px solid var(--border-subtle);
  overflow: hidden;
}

.group-header {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 16px 20px;
  background: var(--bg-elevated);
  border-bottom: 1px solid var(--border-subtle);
}

.group-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border-radius: var(--radius-md);
}

.group-icon.exec {
  background: var(--status-info-dim);
  color: var(--status-info);
}

.group-icon.file {
  background: var(--status-safe-dim);
  color: var(--status-safe);
}

.group-icon.connect {
  background: var(--status-warning-dim);
  color: var(--status-warning);
}

.group-title {
  flex: 1;
  font-size: 14px;
  font-weight: 600;
  color: var(--text-primary);
  margin: 0;
}

.group-count {
  padding: 4px 12px;
  background: var(--bg-overlay);
  border-radius: var(--radius-full);
  font-size: 12px;
  font-weight: 600;
  font-family: var(--font-mono);
  color: var(--text-secondary);
}

.group-content {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 16px;
}
</style>
