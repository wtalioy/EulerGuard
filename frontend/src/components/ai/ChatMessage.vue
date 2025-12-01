<script setup lang="ts">
import { computed } from 'vue'
import { User, Brain } from 'lucide-vue-next'
import type { ChatMessage } from '../../lib/api'

const props = defineProps<{
    message: ChatMessage
}>()

const isUser = computed(() => props.message.role === 'user')
const formattedTime = computed(() => {
    const date = new Date(props.message.timestamp)
    return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
})

function escapeHtml(text: string): string {
    return text
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;')
        .replace(/'/g, '&#39;')
}

// Simple markdown rendering with HTML escaping
function renderMarkdown(text: string): string {
    const escaped = escapeHtml(text)
    return escaped
        .replace(/^### (.+)$/gm, '<h4>$1</h4>')
        .replace(/^## (.+)$/gm, '<h3>$1</h3>')
        .replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>')
        .replace(/\*(.+?)\*/g, '<em>$1</em>')
        .replace(/`([^`]+)`/g, '<code>$1</code>')
        .replace(/^- (.+)$/gm, '<li>$1</li>')
        .replace(/\n\n/g, '<br><br>')
}
</script>

<template>
    <div class="message" :class="{ user: isUser, assistant: !isUser }">
        <div class="avatar">
            <component :is="isUser ? User : Brain" :size="16" />
        </div>
        <div class="content">
            <div class="meta">
                <span class="role">{{ isUser ? 'You' : 'EulerGuard AI' }}</span>
                <span class="time">{{ formattedTime }}</span>
            </div>
            <div 
                v-if="isUser" 
                class="text"
            >{{ message.content }}</div>
            <div 
                v-else 
                class="text markdown" 
                v-html="renderMarkdown(message.content)"
            />
        </div>
    </div>
</template>

<style scoped>
.message {
    display: flex;
    gap: 12px;
}

.message.user {
    flex-direction: row-reverse;
}

.avatar {
    width: 32px;
    height: 32px;
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
}

.message.user .avatar {
    background: var(--bg-elevated);
    color: var(--text-secondary);
}

.message.assistant .avatar {
    background: linear-gradient(135deg, var(--accent-primary), #a855f7);
    color: white;
}

.content {
    max-width: 85%;
}

.meta {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-bottom: 4px;
}

.role {
    font-size: 12px;
    font-weight: 500;
    color: var(--text-secondary);
}

.time {
    font-size: 11px;
    color: var(--text-muted);
}

.text {
    padding: 12px 16px;
    border-radius: var(--radius-lg);
    font-size: 14px;
    line-height: 1.6;
}

.message.user .text {
    background: var(--accent-primary);
    color: white;
    border-bottom-right-radius: 4px;
}

.message.assistant .text {
    background: var(--bg-elevated);
    color: var(--text-primary);
    border-bottom-left-radius: 4px;
}

.text.markdown :deep(h3),
.text.markdown :deep(h4) {
    margin: 12px 0 6px;
    color: var(--text-primary);
}

.text.markdown :deep(h3) { font-size: 14px; }
.text.markdown :deep(h4) { font-size: 13px; }

.text.markdown :deep(code) {
    background: var(--bg-void);
    padding: 2px 6px;
    border-radius: 4px;
    font-family: var(--font-mono);
    font-size: 12px;
}

.text.markdown :deep(strong) {
    color: var(--accent-primary);
}

.text.markdown :deep(li) {
    margin: 4px 0;
    padding-left: 8px;
}
</style>

