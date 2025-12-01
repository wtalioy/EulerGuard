<script setup lang="ts">
import { Database } from 'lucide-vue-next'

defineProps<{
    summary: string
}>()
</script>

<template>
    <div class="context-badge">
        <Database :size="12" />
        <span>Context: {{ summary }}</span>
    </div>
</template>

<style scoped>
.context-badge {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 6px 12px;
    background: rgba(59, 130, 246, 0.1);
    border-bottom: 1px solid var(--border-subtle);
    font-size: 11px;
    color: var(--chart-network);
}
</style>

