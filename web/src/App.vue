<script setup lang="ts">
import { useRoute } from 'vue-router'
import AppShell from '@/components/AppShell.vue'

const route = useRoute()
</script>

<template>
  <UApp>
    <component :is="route.meta.layout === 'blank' ? 'div' : AppShell">
      <router-view />
    </component>
  </UApp>
</template>
