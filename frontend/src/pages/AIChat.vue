<script setup lang="ts">
import { ref, watch, nextTick, onMounted, computed } from 'vue'
import { 
    Send, Trash2, Brain, Cloud, Info, Sparkles, 
    PanelRightOpen, PanelRightClose, Download, Copy, Check,
    Zap, Activity, AlertTriangle, Box, ChevronDown
} from 'lucide-vue-next'
import { useAIChat } from '../composables/useAIChat'
import { getAIStatus, getSystemStats, getWorkloads, type AIStatus, type SystemStats, type Workload } from '../lib/api'
import ChatMessage from '../components/ai/ChatMessage.vue'

const {
    messages,
    isLoading,
    error,
    lastContextSummary,
    hasMessages,
    sendMessage,
    clearChat
} = useAIChat()

// State
const aiStatus = ref<AIStatus | null>(null)
const systemStats = ref<SystemStats | null>(null)
const workloads = ref<Workload[]>([])
const inputText = ref('')
const messagesContainer = ref<HTMLElement | null>(null)
const showContextPanel = ref(true)
const selectedWorkload = ref<string>('all')
const copied = ref(false)

// Computed
const isAIReady = computed(() => aiStatus.value?.enabled && aiStatus.value?.status === 'ready')

onMounted(async () => {
    try {
        const [status, stats, wl] = await Promise.all([
            getAIStatus(),
            getSystemStats(),
            getWorkloads()
        ])
        aiStatus.value = status
        systemStats.value = stats
        workloads.value = wl
    } catch (e) {
        console.error('Failed to fetch initial data:', e)
    }
    
    // Refresh stats periodically
    setInterval(async () => {
        try {
            systemStats.value = await getSystemStats()
        } catch {}
    }, 5000)
})

// Auto-scroll to bottom when new messages arrive
watch(messages, async () => {
    await nextTick()
    if (messagesContainer.value) {
        messagesContainer.value.scrollTop = messagesContainer.value.scrollHeight
    }
}, { deep: true })

async function handleSend() {
    if (!inputText.value.trim() || isLoading.value) return
    
    const message = inputText.value
    inputText.value = ''
    await sendMessage(message)
}

function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter' && !e.shiftKey) {
        e.preventDefault()
        handleSend()
    }
}

async function exportChat() {
    const content = messages.value.map(m => 
        `[${m.role.toUpperCase()}] ${new Date(m.timestamp).toLocaleString()}\n${m.content}`
    ).join('\n\n---\n\n')
    
    const blob = new Blob([content], { type: 'text/markdown' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `eulerguard-chat-${Date.now()}.md`
    a.click()
    URL.revokeObjectURL(url)
}

async function copyLastResponse() {
    const lastAssistant = [...messages.value].reverse().find(m => m.role === 'assistant')
    if (lastAssistant) {
        await navigator.clipboard.writeText(lastAssistant.content)
        copied.value = true
        setTimeout(() => copied.value = false, 2000)
    }
}

const quickActions = [
    { text: "System health check", icon: Activity, color: 'green' },
    { text: "Security assessment", icon: AlertTriangle, color: 'orange' },
    { text: "Analyze recent alerts", icon: Zap, color: 'red' },
    { text: "Top resource consumers", icon: Box, color: 'blue' },
]

const suggestionQuestions = [
    { text: "What's causing the high CPU load?", category: 'performance' },
    { text: "Are there any blocked security events?", category: 'security' },
    { text: "Explain the reverse shell alerts", category: 'security' },
    { text: "Which containers are most active?", category: 'workloads' },
    { text: "Summarize network connections", category: 'network' },
    { text: "Any suspicious file access?", category: 'security' },
]
</script>

<template>
    <div class="ai-chat-page">
        <!-- Main Chat Area -->
        <div class="chat-main">
            <!-- Chat Header -->
            <div class="chat-header">
                <div class="header-left">
                    <div class="title-section">
                        <div class="ai-avatar">
                            <Sparkles :size="20" />
                        </div>
                        <div>
                            <h1>EulerGuard AI</h1>
                            <div class="provider-info" v-if="aiStatus?.enabled">
                                <component :is="aiStatus.isLocal ? Brain : Cloud" :size="12" />
                                <span>{{ aiStatus.provider }}</span>
                                <span class="status-indicator" :class="aiStatus.status">
                                    {{ aiStatus.status === 'ready' ? 'Online' : 'Offline' }}
                                </span>
                            </div>
                        </div>
                    </div>
                </div>
                
                <div class="header-actions">
                    <!-- Workload Filter -->
                    <div class="workload-filter" v-if="workloads.length > 0">
                        <Box :size="14" />
                        <select v-model="selectedWorkload">
                            <option value="all">All Workloads</option>
                            <option v-for="w in workloads" :key="w.id" :value="w.id">
                                {{ w.cgroupPath.split('/').pop() || w.id }}
                            </option>
                        </select>
                        <ChevronDown :size="14" class="select-arrow" />
                    </div>
                    
                    <!-- Action Buttons -->
                    <button 
                        v-if="hasMessages"
                        class="header-btn" 
                        @click="copyLastResponse"
                        title="Copy last response"
                    >
                        <component :is="copied ? Check : Copy" :size="16" />
                    </button>
                    <button 
                        v-if="hasMessages"
                        class="header-btn" 
                        @click="exportChat"
                        title="Export conversation"
                    >
                        <Download :size="16" />
                    </button>
                    <button 
                        v-if="hasMessages" 
                        class="header-btn danger"
                        @click="clearChat"
                        title="New conversation"
                    >
                        <Trash2 :size="16" />
                    </button>
                    <button 
                        class="header-btn"
                        @click="showContextPanel = !showContextPanel"
                        :title="showContextPanel ? 'Hide context panel' : 'Show context panel'"
                    >
                        <component :is="showContextPanel ? PanelRightClose : PanelRightOpen" :size="16" />
                    </button>
                </div>
            </div>
            
            <!-- Messages Container -->
            <div class="messages-wrapper">
                <div ref="messagesContainer" class="messages-container">
                    <!-- Empty State -->
                    <div v-if="!hasMessages && !isLoading" class="empty-state">
                        <div class="welcome-section">
                            <div class="welcome-icon">
                                <Sparkles :size="32" />
                            </div>
                            <h2>How can I help you today?</h2>
                            <p>I have real-time access to your kernel telemetry via eBPF</p>
                        </div>
                        
                        <!-- Quick Actions -->
                        <div class="quick-actions">
                            <button 
                                v-for="action in quickActions"
                                :key="action.text"
                                class="quick-action-btn"
                                :class="action.color"
                                @click="sendMessage(action.text)"
                            >
                                <component :is="action.icon" :size="18" />
                                <span>{{ action.text }}</span>
                            </button>
                        </div>
                        
                        <!-- Suggestions Grid -->
                        <div class="suggestions-section">
                            <div class="suggestions-grid">
                                <button 
                                    v-for="q in suggestionQuestions" 
                                    :key="q.text"
                                    class="suggestion-btn"
                                    @click="sendMessage(q.text)"
                                >
                                    <span>{{ q.text }}</span>
                                    <span class="suggestion-arrow">→</span>
                                </button>
                            </div>
                        </div>
                    </div>
                    
                    <!-- Messages -->
                    <template v-if="hasMessages">
                        <ChatMessage 
                            v-for="(msg, i) in messages" 
                            :key="i" 
                            :message="msg"
                        />
                    </template>
                    
                    <!-- Loading -->
                    <div v-if="isLoading" class="typing-indicator">
                        <div class="typing-avatar">
                            <Brain :size="16" />
                        </div>
                        <div class="typing-dots">
                            <span></span>
                            <span></span>
                            <span></span>
                        </div>
                    </div>
                    
                    <!-- Error -->
                    <div v-if="error" class="error-toast">
                        <AlertTriangle :size="16" />
                        <span>{{ error }}</span>
                        <button @click="error = null">×</button>
                    </div>
                </div>
            </div>
            
            <!-- Input Area -->
            <div class="input-section">
                <div class="input-container">
                    <textarea
                        v-model="inputText"
                        placeholder="Ask about system security, performance, alerts..."
                        rows="1"
                        @keydown="handleKeydown"
                        :disabled="isLoading || !isAIReady"
                    />
                    <button 
                        class="send-btn" 
                        @click="handleSend"
                        :disabled="!inputText.trim() || isLoading || !isAIReady"
                    >
                        <Send :size="18" />
                    </button>
                </div>
                <div class="input-footer">
                    <span class="context-note">
                        <Zap :size="10" />
                        Live eBPF context auto-injected
                    </span>
                    <span class="shortcut-hint">Enter to send</span>
                </div>
            </div>
        </div>
        
        <!-- Context Sidebar -->
        <Transition name="slide-panel">
            <aside v-if="showContextPanel" class="context-panel">
                <div class="panel-header">
                    <h3>Live Telemetry</h3>
                    <span class="live-badge">
                        <span class="live-dot"></span>
                        LIVE
                    </span>
                </div>
                
                <!-- System Stats -->
                <div class="stats-grid" v-if="systemStats">
                    <div class="stat-card">
                        <span class="stat-value">{{ systemStats.eventsPerSec.toFixed(0) }}</span>
                        <span class="stat-label">Events/sec</span>
                    </div>
                    <div class="stat-card">
                        <span class="stat-value">{{ systemStats.processCount }}</span>
                        <span class="stat-label">Processes</span>
                    </div>
                    <div class="stat-card">
                        <span class="stat-value">{{ systemStats.workloadCount }}</span>
                        <span class="stat-label">Workloads</span>
                    </div>
                    <div class="stat-card" :class="{ alert: systemStats.alertCount > 0 }">
                        <span class="stat-value">{{ systemStats.alertCount }}</span>
                        <span class="stat-label">Alerts</span>
                    </div>
                </div>
                
                <!-- Active Workloads -->
                <div class="panel-section" v-if="workloads.length > 0">
                    <h4>Active Workloads</h4>
                    <div class="workload-list">
                        <div 
                            v-for="w in workloads.slice(0, 5)" 
                            :key="w.id" 
                            class="workload-item"
                            :class="{ selected: selectedWorkload === w.id }"
                            @click="selectedWorkload = w.id"
                        >
                            <Box :size="14" />
                            <span class="workload-name">{{ w.cgroupPath.split('/').pop() || 'unknown' }}</span>
                            <span class="workload-events">{{ w.execCount + w.fileCount + w.connectCount }}</span>
                        </div>
                    </div>
                </div>
                
                <!-- Context Summary -->
                <div class="panel-section" v-if="lastContextSummary">
                    <h4>AI Context</h4>
                    <div class="context-summary">
                        {{ lastContextSummary }}
                    </div>
                </div>
            </aside>
        </Transition>
    </div>
</template>

<style scoped>
.ai-chat-page {
    display: flex;
    height: calc(100vh - var(--topbar-height) - 48px);
    gap: 0;
}

/* Main Chat Area */
.chat-main {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-width: 0;
}

.chat-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 16px 24px;
    border-bottom: 1px solid var(--border-subtle);
    background: var(--bg-surface);
}

.header-left {
    display: flex;
    align-items: center;
    gap: 16px;
}

.title-section {
    display: flex;
    align-items: center;
    gap: 12px;
}

.ai-avatar {
    width: 40px;
    height: 40px;
    border-radius: 12px;
    background: linear-gradient(135deg, var(--accent-primary), #a855f7);
    display: flex;
    align-items: center;
    justify-content: center;
    color: white;
}

.title-section h1 {
    font-size: 18px;
    font-weight: 600;
    color: var(--text-primary);
    margin: 0;
}

.provider-info {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 12px;
    color: var(--text-muted);
    margin-top: 2px;
}

.status-indicator {
    padding: 2px 6px;
    border-radius: var(--radius-full);
    font-size: 10px;
    font-weight: 500;
}

.status-indicator.ready {
    background: rgba(34, 197, 94, 0.15);
    color: var(--status-safe);
}

.status-indicator.unavailable {
    background: rgba(239, 68, 68, 0.15);
    color: var(--status-critical);
}

.header-actions {
    display: flex;
    align-items: center;
    gap: 8px;
}

.workload-filter {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 6px 10px;
    background: var(--bg-elevated);
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-sm);
    font-size: 12px;
    color: var(--text-secondary);
    position: relative;
}

.workload-filter select {
    background: none;
    border: none;
    color: inherit;
    font-size: inherit;
    padding-right: 16px;
    cursor: pointer;
    appearance: none;
}

.select-arrow {
    position: absolute;
    right: 8px;
    pointer-events: none;
}

.header-btn {
    width: 36px;
    height: 36px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--bg-elevated);
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-sm);
    color: var(--text-secondary);
    cursor: pointer;
    transition: all 0.15s;
}

.header-btn:hover {
    background: var(--bg-hover);
    color: var(--text-primary);
}

.header-btn.danger:hover {
    color: var(--status-critical);
    border-color: var(--status-critical);
}

/* Messages */
.messages-wrapper {
    flex: 1;
    overflow: hidden;
    background: var(--bg-void);
}

.messages-container {
    height: 100%;
    overflow-y: auto;
    padding: 24px;
    display: flex;
    flex-direction: column;
    gap: 24px;
}

/* Empty State */
.empty-state {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    max-width: 700px;
    margin: 0 auto;
    padding: 40px 20px;
}

.welcome-section {
    text-align: center;
    margin-bottom: 32px;
}

.welcome-icon {
    width: 64px;
    height: 64px;
    border-radius: 16px;
    background: linear-gradient(135deg, var(--accent-primary), #a855f7);
    display: flex;
    align-items: center;
    justify-content: center;
    color: white;
    margin: 0 auto 16px;
    animation: float 3s ease-in-out infinite;
}

@keyframes float {
    0%, 100% { transform: translateY(0); }
    50% { transform: translateY(-8px); }
}

.welcome-section h2 {
    font-size: 24px;
    font-weight: 600;
    color: var(--text-primary);
    margin: 0 0 8px;
}

.welcome-section p {
    color: var(--text-muted);
    margin: 0;
}

/* Quick Actions */
.quick-actions {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 12px;
    width: 100%;
    max-width: 500px;
    margin-bottom: 24px;
}

.quick-action-btn {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 14px 16px;
    background: var(--bg-surface);
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-lg);
    color: var(--text-secondary);
    font-size: 13px;
    font-weight: 500;
    cursor: pointer;
    transition: all 0.2s;
}

.quick-action-btn:hover {
    transform: translateY(-2px);
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
}

.quick-action-btn.green:hover { border-color: var(--status-safe); color: var(--status-safe); }
.quick-action-btn.orange:hover { border-color: var(--status-warning); color: var(--status-warning); }
.quick-action-btn.red:hover { border-color: var(--status-critical); color: var(--status-critical); }
.quick-action-btn.blue:hover { border-color: var(--accent-primary); color: var(--accent-primary); }

/* Suggestions */
.suggestions-section {
    width: 100%;
    max-width: 600px;
}

.suggestions-grid {
    display: flex;
    flex-direction: column;
    gap: 8px;
}

.suggestion-btn {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 12px 16px;
    background: var(--bg-surface);
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-md);
    color: var(--text-secondary);
    font-size: 13px;
    text-align: left;
    cursor: pointer;
    transition: all 0.15s;
}

.suggestion-btn:hover {
    background: var(--bg-elevated);
    color: var(--text-primary);
    border-color: var(--accent-primary);
}

.suggestion-arrow {
    opacity: 0;
    transform: translateX(-4px);
    transition: all 0.15s;
}

.suggestion-btn:hover .suggestion-arrow {
    opacity: 1;
    transform: translateX(0);
}

/* Typing Indicator */
.typing-indicator {
    display: flex;
    align-items: flex-start;
    gap: 12px;
}

.typing-avatar {
    width: 32px;
    height: 32px;
    border-radius: 8px;
    background: linear-gradient(135deg, var(--accent-primary), #a855f7);
    display: flex;
    align-items: center;
    justify-content: center;
    color: white;
    flex-shrink: 0;
}

.typing-dots {
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 16px;
    background: var(--bg-surface);
    border-radius: var(--radius-lg);
}

.typing-dots span {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--text-muted);
    animation: typing 1.4s infinite;
}

.typing-dots span:nth-child(2) { animation-delay: 0.2s; }
.typing-dots span:nth-child(3) { animation-delay: 0.4s; }

@keyframes typing {
    0%, 60%, 100% { transform: translateY(0); opacity: 0.4; }
    30% { transform: translateY(-4px); opacity: 1; }
}

/* Error Toast */
.error-toast {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 12px 16px;
    background: rgba(239, 68, 68, 0.1);
    border: 1px solid rgba(239, 68, 68, 0.2);
    border-radius: var(--radius-md);
    color: var(--status-critical);
    font-size: 13px;
}

.error-toast button {
    margin-left: auto;
    background: none;
    border: none;
    color: inherit;
    font-size: 18px;
    cursor: pointer;
    line-height: 1;
}

/* Input Section */
.input-section {
    padding: 16px 24px 20px;
    background: var(--bg-surface);
    border-top: 1px solid var(--border-subtle);
}

.input-container {
    display: flex;
    gap: 12px;
    align-items: flex-end;
    background: var(--bg-elevated);
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-xl);
    padding: 8px 8px 8px 16px;
    transition: border-color 0.15s, box-shadow 0.15s;
}

.input-container:focus-within {
    border-color: var(--accent-primary);
    box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.1);
}

.input-container textarea {
    flex: 1;
    background: none;
    border: none;
    color: var(--text-primary);
    font-size: 14px;
    resize: none;
    min-height: 24px;
    max-height: 120px;
    line-height: 1.5;
    padding: 8px 0;
}

.input-container textarea:focus {
    outline: none;
}

.input-container textarea::placeholder {
    color: var(--text-muted);
}

.send-btn {
    width: 40px;
    height: 40px;
    background: linear-gradient(135deg, var(--accent-primary), #a855f7);
    border: none;
    border-radius: var(--radius-lg);
    color: white;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: all 0.15s;
    flex-shrink: 0;
}

.send-btn:hover:not(:disabled) {
    transform: scale(1.05);
}

.send-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
}

.input-footer {
    display: flex;
    justify-content: space-between;
    margin-top: 8px;
    font-size: 11px;
    color: var(--text-muted);
}

.context-note {
    display: flex;
    align-items: center;
    gap: 4px;
    color: var(--accent-primary);
}

/* Context Panel */
.context-panel {
    width: 280px;
    background: var(--bg-surface);
    border-left: 1px solid var(--border-subtle);
    display: flex;
    flex-direction: column;
    overflow: hidden;
}

.panel-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 16px;
    border-bottom: 1px solid var(--border-subtle);
}

.panel-header h3 {
    font-size: 14px;
    font-weight: 600;
    color: var(--text-primary);
    margin: 0;
}

.live-badge {
    display: flex;
    align-items: center;
    gap: 4px;
    font-size: 10px;
    font-weight: 600;
    color: var(--status-safe);
    padding: 2px 6px;
    background: rgba(34, 197, 94, 0.15);
    border-radius: var(--radius-full);
}

.live-dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--status-safe);
    animation: pulse 2s infinite;
}

@keyframes pulse {
    0%, 100% { opacity: 1; }
    50% { opacity: 0.5; }
}

.stats-grid {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 8px;
    padding: 16px;
}

.stat-card {
    padding: 12px;
    background: var(--bg-elevated);
    border-radius: var(--radius-md);
    text-align: center;
}

.stat-card.alert {
    background: rgba(239, 68, 68, 0.1);
}

.stat-value {
    display: block;
    font-size: 20px;
    font-weight: 600;
    color: var(--text-primary);
}

.stat-card.alert .stat-value {
    color: var(--status-critical);
}

.stat-label {
    font-size: 11px;
    color: var(--text-muted);
}

.panel-section {
    padding: 16px;
    border-top: 1px solid var(--border-subtle);
}

.panel-section h4 {
    font-size: 12px;
    font-weight: 600;
    color: var(--text-muted);
    margin: 0 0 12px;
    text-transform: uppercase;
    letter-spacing: 0.05em;
}

.workload-list {
    display: flex;
    flex-direction: column;
    gap: 4px;
}

.workload-item {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 8px 10px;
    border-radius: var(--radius-sm);
    font-size: 12px;
    color: var(--text-secondary);
    cursor: pointer;
    transition: all 0.15s;
}

.workload-item:hover {
    background: var(--bg-hover);
}

.workload-item.selected {
    background: var(--accent-glow);
    color: var(--accent-primary);
}

.workload-name {
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
}

.workload-events {
    font-size: 11px;
    padding: 2px 6px;
    background: var(--bg-elevated);
    border-radius: var(--radius-full);
}

.context-summary {
    font-size: 12px;
    color: var(--text-secondary);
    line-height: 1.5;
    padding: 10px;
    background: var(--bg-elevated);
    border-radius: var(--radius-md);
}

/* Panel Transition */
.slide-panel-enter-active,
.slide-panel-leave-active {
    transition: all 0.3s ease;
}

.slide-panel-enter-from,
.slide-panel-leave-to {
    width: 0;
    opacity: 0;
}
</style>

