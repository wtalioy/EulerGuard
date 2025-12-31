<script setup lang="ts">
defineProps<{
  title?: string
  padding?: boolean
}>()
</script>

<template>
  <div class="card-base" :class="{ 'card-content-base': padding !== false }">
    <template v-if="title">
      <div class="card-header-base">
        <h3 class="card-title-base">{{ title }}</h3>
        <slot name="header-actions"></slot>
      </div>
    </template>
    <div :class="padding !== false ? 'card-content-base' : ''">
      <slot></slot>
    </div>
  </div>
</template>
