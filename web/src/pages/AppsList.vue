<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAppsStore } from '@/stores/apps'

const apps = useAppsStore()
const router = useRouter()
const showCreate = ref(false)
const newName = ref('')
const newImage = ref('')

onMounted(() => apps.load())

async function create() {
  await apps.create({ name: newName.value, image_repo: newImage.value || undefined })
  showCreate.value = false
  newName.value = ''
  newImage.value = ''
}
</script>

<template>
  <div class="page">
    <header class="head">
      <h1>Apps</h1>
      <UButton icon="i-lucide-plus" @click="showCreate = true">New App</UButton>
    </header>
    <UTable
      :data="apps.items"
      :columns="[
        { accessorKey: 'name', header: 'Name' },
        { accessorKey: 'image_repo', header: 'Image' },
        { accessorKey: 'current_version', header: 'Version' },
        { accessorKey: 'deploy_strategy', header: 'Strategy' },
      ]"
      @select="(row: any) => router.push(`/apps/${row.original.name}`)"
    />

    <UModal v-model:open="showCreate" title="Create app">
      <template #body>
        <UFormField label="Name">
          <UInput v-model="newName" placeholder="my-app" />
        </UFormField>
        <UFormField label="Image (optional)">
          <UInput v-model="newImage" placeholder="ghcr.io/me/app:latest" />
        </UFormField>
      </template>
      <template #footer>
        <UButton color="neutral" variant="ghost" @click="showCreate = false">Cancel</UButton>
        <UButton @click="create" :disabled="!newName">Create</UButton>
      </template>
    </UModal>
  </div>
</template>

<style scoped>
.head { display: flex; align-items: center; justify-content: space-between; margin-bottom: 1.5rem; }
.head h1 { margin: 0; }
</style>
