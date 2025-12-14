// AI Omnibox Composable - Phase 4
// Using singleton pattern to share state across all components
import { ref, computed, nextTick } from 'vue'
import { useAI, type Intent } from './useAI'
import { useRouter } from 'vue-router'
import { sendChatMessage } from '../lib/api'

// Shared state - singleton pattern
const isOpen = ref(false)
const input = ref('')
const intent = ref<Intent | null>(null)
const loading = ref(false)
const recentIntents = ref<Array<{ input: string; intent: Intent; timestamp: number }>>([])
const aiAnswer = ref<string | null>(null)
const answering = ref(false)

// Clean AI response to remove prompt content and internal markers
function cleanAIResponse(message: string): string {
  let cleaned = message

  // Remove context markers (these are definitely not user-facing)
  cleaned = cleaned.replace(/\[END CONTEXT[^\]]*\]/gi, '').trim()
  cleaned = cleaned.replace(/\[LIVE SYSTEM CONTEXT[^\]]*\]/gi, '').trim()
  cleaned = cleaned.replace(/\[END CONTEXT - The data above is for your reference only[^\]]*\]/gi, '').trim()
  cleaned = cleaned.replace(/\[INTERNAL NOTE[^\]]*\]/gi, '').trim()

  // Remove specific instruction patterns that might leak from system prompt
  cleaned = cleaned.replace(/IMPORTANT:.*?use the real data.*?\./gis, '').trim()
  cleaned = cleaned.replace(/CRITICAL:.*?Answer the User's Question Directly[^\n]*/gi, '').trim()
  cleaned = cleaned.replace(/NEVER include context markers.*?in your response/gi, '').trim()
  cleaned = cleaned.replace(/Your response should only contain.*?no internal markers/gi, '').trim()
  cleaned = cleaned.replace(/Do not include this marker or any instructions in your response/gi, '').trim()

  // Remove internal context notes about false positives (common pattern that leaks)
  cleaned = cleaned.replace(/NOTE:\s*Not all alerts indicate real threats[^\n]*/gi, '').trim()
  cleaned = cleaned.replace(/Not all alerts indicate real threats[^\n]*/gi, '').trim()
  cleaned = cleaned.replace(/Common system utilities \(ls, cat, grep, ps, etc\.\) doing normal operations may be false positives/gi, '').trim()
  cleaned = cleaned.replace(/Common utilities \(ls, cat, grep\) accessing normal files are typically NOT suspicious/gi, '').trim()

  // Remove system prompt introduction fragments (but preserve legitimate content)
  cleaned = cleaned.replace(/^You are Aegis AI, an intelligent assistant[^\n]*$/gm, '').trim()
  cleaned = cleaned.replace(/^Your capabilities:[^\n]*$/gm, '').trim()

  // Remove instruction sections that are clearly from the prompt
  cleaned = cleaned.replace(/Answer ONLY what the user asked - be contextual and concise/gi, '').trim()
  cleaned = cleaned.replace(/Response Guidelines:[^\n]*/gi, '').trim()
  cleaned = cleaned.replace(/NEVER provide UI navigation instructions/gi, '').trim()
  cleaned = cleaned.replace(/NEVER tell users how to navigate the UI/gi, '').trim()
  cleaned = cleaned.replace(/Focus on providing information, not teaching users how to use the interface/gi, '').trim()

  // Remove prompt-like instruction blocks (multi-line patterns that are clearly instructions)
  cleaned = cleaned.replace(/1\. \*\*Understand the Query Intent\*\*:.*?2\. \*\*Answer Directly/gs, '').trim()
  cleaned = cleaned.replace(/ALWAYS answer the question using the actual data/gi, '').trim()
  cleaned = cleaned.replace(/NEVER output template placeholders/gi, '').trim()

  // Clean up multiple consecutive newlines
  cleaned = cleaned.replace(/\n{3,}/g, '\n\n').trim()

  return cleaned
}

export function useOmnibox() {
  const router = useRouter()
  const { parseIntent, generateRule } = useAI()

  const toggle = () => {
    isOpen.value = !isOpen.value
    if (isOpen.value) {
      input.value = ''
      intent.value = null
    }
  }

  const openWithQuery = (query: string) => {
    input.value = query
    isOpen.value = true
    intent.value = null
    aiAnswer.value = null
    // Don't auto-parse - wait for user to press Enter
    // This gives them a chance to edit the query if needed
  }

  const askQuestion = async (question: string) => {
    // Set the input and open omnibox
    input.value = question
    isOpen.value = true
    intent.value = null
    aiAnswer.value = null

    // Wait for next tick to ensure omnibox is rendered
    await nextTick()

    // Directly send to AI without intent parsing - we know these are questions
    answering.value = true
    try {
      const chatResponse = await sendChatMessage(question, 'omnibox-session')

      // Clean up AI response - remove any internal context markers and prompt content
      aiAnswer.value = cleanAIResponse(chatResponse.message)
    } catch (err) {
      console.error('Failed to get AI answer:', err)
      // If AI fails, fall back to intent parsing
      await parseInput()
    } finally {
      answering.value = false
    }
  }

  const close = () => {
    isOpen.value = false
    input.value = ''
    intent.value = null
  }

  // Quick heuristic to detect if input is likely a question (skip intent parsing)
  const isLikelyQuestion = (text: string): boolean => {
    const trimmed = text.trim().toLowerCase()
    const questionWords = ['what', 'how', 'why', 'when', 'where', 'who', 'which', 'show me', 'tell me', 'explain', 'describe', 'list']
    return questionWords.some(word => trimmed.startsWith(word)) || trimmed.endsWith('?')
  }

  // Quick heuristic to detect if input is likely an action (needs intent parsing)
  const isLikelyAction = (text: string): boolean => {
    const trimmed = text.trim().toLowerCase()
    const actionWords = ['create', 'block', 'allow', 'promote', 'delete', 'go to', 'navigate', 'open', 'enable', 'disable']
    return actionWords.some(word => trimmed.startsWith(word))
  }

  const parseInput = async () => {
    if (!input.value.trim()) return

    aiAnswer.value = null
    intent.value = null

    const inputText = input.value.trim()

    // For obvious questions, skip intent parsing and go straight to AI chat
    if (isLikelyQuestion(inputText) && !isLikelyAction(inputText)) {
      answering.value = true
      try {
        const chatResponse = await sendChatMessage(input.value, 'omnibox-session')

        // Clean up AI response - remove any internal context markers and prompt content
        aiAnswer.value = cleanAIResponse(chatResponse.message)
      } catch (err) {
        console.error('Failed to get AI answer:', err)
      } finally {
        answering.value = false
      }
      return
    }

    // For potential actions, show "AI is thinking" immediately and do both in parallel
    answering.value = true // Show "AI is thinking" right away (skip "Processing...")

    // Parse intent in parallel with chat - both are AI calls so do them together
    const intentPromise = parseIntent(input.value, {
      currentPage: router.currentRoute.value.path,
      selectedItem: '',
      recentActions: recentIntents.value.slice(-5).map(r => r.input)
    })

    // Start chat immediately (don't wait for intent parsing)
    const chatPromise = sendChatMessage(input.value, 'omnibox-session')

    try {
      // Wait for both, but chat is the priority
      const [parsed, chatResponse] = await Promise.all([intentPromise, chatPromise])

      // Clean up AI response - remove any internal context markers and prompt content
      aiAnswer.value = cleanAIResponse(chatResponse.message)

      // If it's a clear navigation action, execute it instead of showing answer
      if (parsed && (parsed.type === 'create_rule' || parsed.type === 'navigation') && parsed.confidence > 0.7) {
        intent.value = parsed
        answering.value = false
        await executeIntent()
        return
      }

      // Only store intent for recent history if it's a clear action (not a question)
      if (parsed && parsed.type !== 'explain_event' && parsed.confidence > 0.7) {
        intent.value = parsed
        recentIntents.value.push({
          input: input.value,
          intent: parsed,
          timestamp: Date.now()
        })
        if (recentIntents.value.length > 10) {
          recentIntents.value = recentIntents.value.slice(-10)
        }
      } else {
        intent.value = null
      }
    } catch (err) {
      console.error('Failed to get AI answer:', err)
      // If chat fails, try to use intent if available
      try {
        const parsed = await intentPromise
        if (parsed) {
          intent.value = parsed
        }
      } catch (intentErr) {
        console.error('Intent parsing also failed:', intentErr)
      }
    } finally {
      answering.value = false
      loading.value = false
    }
  }

  const executeIntent = async () => {
    if (!intent.value) return

    const { type, params } = intent.value

    switch (type) {
      case 'create_rule':
        // Navigate to Policy Studio with pre-filled rule generation
        router.push({
          path: '/policy-studio',
          query: { generate: input.value }
        })
        close()
        break

      case 'query_events':
        // Navigate to Investigation with query
        router.push({
          path: '/investigation',
          query: { q: input.value }
        })
        close()
        break

      case 'explain_event':
        // Show explanation modal or navigate
        if (params.eventId) {
          router.push({
            path: '/investigation',
            query: { explain: params.eventId }
          })
        }
        close()
        break

      case 'analyze_process':
        // Navigate to Investigation - if PID is provided, analyze that process
        // Otherwise, just open Investigation page where user can search/analyze
        if (params.pid) {
          router.push({
            path: '/investigation',
            query: { analyze: 'process', id: params.pid }
          })
        } else {
          // For questions like "Which workloads have alerts?", navigate with query
          router.push({
            path: '/investigation',
            query: { q: input.value }
          })
        }
        close()
        break

      case 'promote_rule':
        if (params.ruleName) {
          // Call promote API
          // This would be handled by the policy studio page
          router.push({
            path: '/policy-studio',
            query: { promote: params.ruleName }
          })
        }
        close()
        break

      case 'navigation':
        if (params.page) {
          router.push(params.page as string)
        }
        close()
        break

      default:
        console.warn('Unknown intent type:', type)
    }
  }

  // Keyboard shortcuts
  const handleKeydown = (e: KeyboardEvent) => {
    // Cmd/Ctrl + K to toggle
    if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
      e.preventDefault()
      toggle()
    }
    // Escape to close
    if (e.key === 'Escape' && isOpen.value) {
      e.preventDefault()
      close()
    }
    // Note: Enter key handling is done in AIOmnibox component
    // to allow parsing first, then executing
  }

  return {
    isOpen,
    input,
    intent,
    loading,
    aiAnswer,
    answering,
    recentIntents: computed(() => recentIntents.value.slice().reverse()),
    toggle,
    openWithQuery,
    askQuestion,
    close,
    parseInput,
    executeIntent,
    handleKeydown
  }
}

