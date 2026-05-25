<script setup lang="ts">
defineProps<{ height?: string }>()
</script>

<template>
  <img src="/logo.svg" alt="basepod" :style="{ height: height ?? '32px' }" />
</template>
