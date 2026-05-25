<script setup lang="ts">
import { onMounted, ref, h, resolveComponent } from 'vue'
import { useRouter } from 'vue-router'
import { useAppsStore, type App } from '@/stores/apps'

const apps = useAppsStore()
const router = useRouter()
const showCreate = ref(false)
const newName = ref('')
const newImage = ref('')

const RouterLink = resolveComponent('RouterLink')

const columns = [
  {
    accessorKey: 'name',
    header: 'Name',
    cell: ({ row }: { row: { original: App } }) =>
      h(RouterLink, { to: `/apps/${row.original.name}`, class: 'app-link' }, () => row.original.name),
  },
  { accessorKey: 'image_repo', header: 'Image' },
  { accessorKey: 'current_version', header: 'Version' },
  { accessorKey: 'deploy_strategy', header: 'Strategy' },
  {
    id: 'actions',
    header: '',
    cell: ({ row }: { row: { original: App } }) =>
      h(
        'button',
        {
          class: 'view-btn',
          onClick: () => router.push(`/apps/${row.original.name}`),
        },
        'View →',
      ),
  },
]

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

    <div v-if="apps.items.length === 0" class="empty">
      <p>No apps yet.</p>
      <UButton variant="outline" icon="i-lucide-plus" @click="showCreate = true">Create your first app</UButton>
    </div>

    <UTable v-else :data="apps.items" :columns="columns" />

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
.empty {
  background: var(--ui-bg-elevated);
  border: 1px dashed var(--ui-border);
  border-radius: 1rem;
  padding: 3rem;
  text-align: center;
  display: flex; flex-direction: column; align-items: center; gap: 1rem;
}
:deep(.app-link) {
  color: var(--color-primary-500);
  text-decoration: none;
  font-weight: 500;
}
:deep(.app-link:hover) { text-decoration: underline; }
:deep(.view-btn) {
  background: none;
  border: 0;
  color: var(--ui-text-muted);
  cursor: pointer;
  font-size: 0.85rem;
  padding: 0.25rem 0.5rem;
}
:deep(.view-btn:hover) { color: var(--color-primary-500); }
</style>
