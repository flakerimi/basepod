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
      h(RouterLink, {
        to: `/apps/${row.original.name}`,
        class: 'font-medium text-(--ui-primary) no-underline hover:underline',
      }, () => row.original.name),
  },
  { accessorKey: 'image_repo', header: 'Image' },
  { accessorKey: 'current_version', header: 'Version' },
  { accessorKey: 'deploy_strategy', header: 'Strategy' },
  {
    id: 'actions',
    header: '',
    cell: ({ row }: { row: { original: App } }) =>
      h('button', {
        class: 'cursor-pointer border-0 bg-transparent px-2 py-1 text-sm text-(--ui-text-muted) hover:text-(--ui-primary)',
        onClick: () => router.push(`/apps/${row.original.name}`),
      }, 'View →'),
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
  <div>
    <header class="mb-6 flex items-center justify-between">
      <h1 class="m-0 text-2xl font-semibold">Apps</h1>
      <UButton icon="i-lucide-plus" @click="showCreate = true">New App</UButton>
    </header>

    <div
      v-if="apps.items.length === 0"
      class="flex flex-col items-center gap-4 rounded-2xl border border-dashed border-(--ui-border) bg-(--ui-bg-elevated) p-12 text-center"
    >
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
        <UButton :disabled="!newName" @click="create">Create</UButton>
      </template>
    </UModal>
  </div>
</template>
