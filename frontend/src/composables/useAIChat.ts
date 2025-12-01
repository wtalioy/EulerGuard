import { ref, computed } from 'vue'
import { 
    sendChatMessage, 
    getChatHistory, 
    clearChatHistory,
    type ChatMessage, 
    type ChatResponse 
} from '../lib/api'

// Generate a unique session ID
function generateSessionId(): string {
    return `chat-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`
}

// Singleton state (persists across component instances)
const sessionId = ref<string>(generateSessionId())
const messages = ref<ChatMessage[]>([])
const isLoading = ref(false)
const error = ref<string | null>(null)
const lastContextSummary = ref<string>('')

export function useAIChat() {
    const hasMessages = computed(() => messages.value.length > 0)
    
    async function sendMessage(content: string): Promise<void> {
        if (!content.trim() || isLoading.value) return
        
        // Add user message immediately (optimistic)
        const userMessage: ChatMessage = {
            role: 'user',
            content: content.trim(),
            timestamp: Date.now()
        }
        messages.value.push(userMessage)
        
        isLoading.value = true
        error.value = null
        
        try {
            const response: ChatResponse = await sendChatMessage(
                content.trim(),
                sessionId.value
            )
            
            // Update session ID (in case server generated one)
            sessionId.value = response.sessionId
            lastContextSummary.value = response.contextSummary
            
            // Add assistant response
            const assistantMessage: ChatMessage = {
                role: 'assistant',
                content: response.message,
                timestamp: response.timestamp
            }
            messages.value.push(assistantMessage)
            
        } catch (e) {
            error.value = e instanceof Error ? e.message : 'Failed to send message'
            // Remove the optimistic user message on error
            messages.value.pop()
        } finally {
            isLoading.value = false
        }
    }
    
    async function loadHistory(): Promise<void> {
        if (!sessionId.value) return
        
        try {
            const history = await getChatHistory(sessionId.value)
            if (history.length > 0) {
                messages.value = history
            }
        } catch (e) {
            console.error('Failed to load chat history:', e)
        }
    }
    
    async function clearChat(): Promise<void> {
        try {
            await clearChatHistory(sessionId.value)
        } catch (e) {
            console.error('Failed to clear chat:', e)
        }
        
        // Reset local state
        messages.value = []
        sessionId.value = generateSessionId()
        lastContextSummary.value = ''
        error.value = null
    }
    
    return {
        // State
        sessionId,
        messages,
        isLoading,
        error,
        lastContextSummary,
        hasMessages,
        
        // Actions
        sendMessage,
        loadHistory,
        clearChat
    }
}

