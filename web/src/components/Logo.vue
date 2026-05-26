<script setup lang="ts">
const props = defineProps<{ height?: number | string }>()

function px(v: number | string | undefined, fallback: number) {
  if (v === undefined || v === '') return `${fallback}px`
  if (typeof v === 'number') return `${v}px`
  return /^\d+$/.test(v) ? `${v}px` : v
}
</script>

<template>
  <img
    src="/logo.svg"
    alt="basepod"
    class="block w-auto max-w-full"
    :style="{ height: px(props.height, 28) }"
  />
</template>
