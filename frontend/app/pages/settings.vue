<template>
  <div>
    <div class="mb-8 flex flex-col md:flex-row md:items-center justify-between gap-4">
      <div>
        <h1 class="text-3xl font-bold tracking-tight text-zinc-900 dark:text-white">Settings</h1>
        <p class="text-zinc-500 dark:text-zinc-400 mt-2">Manage your service integrations and disk monitoring.</p>
      </div>
      <UButton icon="i-heroicons-plus" color="primary" @click="openAddModal">
        Add Integration
      </UButton>
    </div>

    <!-- Integration Cards -->
    <div v-if="loading" class="flex justify-center py-12">
      <UIcon name="i-heroicons-arrow-path" class="w-8 h-8 text-violet-500 animate-spin" />
    </div>

    <div v-else-if="integrations.length === 0" class="text-center py-16">
      <UIcon name="i-heroicons-server-stack" class="w-16 h-16 text-zinc-300 dark:text-zinc-600 mx-auto mb-4" />
      <h3 class="text-lg font-medium text-zinc-700 dark:text-zinc-300 mb-2">No integrations configured</h3>
      <p class="text-zinc-500 dark:text-zinc-400 mb-6">Connect your Plex, Sonarr, or Radarr instances to get started.</p>
      <UButton icon="i-heroicons-plus" color="primary" @click="openAddModal">
        Add Your First Integration
      </UButton>
    </div>

    <div v-else class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
      <div
        v-for="integration in integrations"
        :key="integration.id"
        class="rounded-xl border border-zinc-200 dark:border-zinc-800 bg-white dark:bg-zinc-900 shadow-sm overflow-hidden"
      >
        <!-- Card Header -->
        <div class="px-5 py-4 border-b border-zinc-100 dark:border-zinc-800 flex items-center justify-between">
          <div class="flex items-center gap-3">
            <div :class="['w-10 h-10 rounded-lg flex items-center justify-center', typeColor(integration.type)]">
              <UIcon :name="typeIcon(integration.type)" class="w-5 h-5 text-white" />
            </div>
            <div>
              <h3 class="font-semibold text-zinc-900 dark:text-white">{{ integration.name }}</h3>
              <span class="text-xs uppercase tracking-wider font-medium" :class="typeTextColor(integration.type)">
                {{ integration.type }}
              </span>
            </div>
          </div>
          <UBadge :color="integration.enabled ? 'success' : 'neutral'" variant="subtle">
            {{ integration.enabled ? 'Active' : 'Disabled' }}
          </UBadge>
        </div>

        <!-- Card Body -->
        <div class="px-5 py-4 space-y-3 text-sm">
          <div class="flex items-center gap-2 text-zinc-500 dark:text-zinc-400">
            <UIcon name="i-heroicons-link" class="w-4 h-4 shrink-0" />
            <span class="truncate">{{ integration.url }}</span>
          </div>
          <div class="flex items-center gap-2 text-zinc-500 dark:text-zinc-400">
            <UIcon name="i-heroicons-key" class="w-4 h-4 shrink-0" />
            <span class="font-mono text-xs">{{ integration.apiKey }}</span>
          </div>
          <div v-if="integration.lastSync" class="flex items-center gap-2 text-zinc-500 dark:text-zinc-400">
            <UIcon name="i-heroicons-clock" class="w-4 h-4 shrink-0" />
            <span>Last synced {{ formatTime(integration.lastSync) }}</span>
          </div>
          <div v-if="integration.lastError" class="flex items-center gap-2 text-red-500">
            <UIcon name="i-heroicons-exclamation-triangle" class="w-4 h-4 shrink-0" />
            <span class="text-xs">{{ integration.lastError }}</span>
          </div>
        </div>

        <!-- Card Footer -->
        <div class="px-5 py-3 border-t border-zinc-100 dark:border-zinc-800 flex items-center justify-between">
          <div class="flex gap-2">
            <UButton size="xs" variant="soft" icon="i-heroicons-arrow-path" :loading="testingId === integration.id" @click="testConnection(integration)">
              Test
            </UButton>
            <UButton size="xs" variant="soft" icon="i-heroicons-pencil" @click="openEditModal(integration)">
              Edit
            </UButton>
          </div>
          <UButton size="xs" variant="ghost" color="error" icon="i-heroicons-trash" @click="confirmDelete(integration)" />
        </div>
      </div>
    </div>

    <!-- Add/Edit Modal -->
    <UModal v-model:open="showModal" :title="editingIntegration ? 'Edit Integration' : 'Add Integration'">
      <template #body>
        <form class="space-y-4" @submit.prevent="onSubmit">
          <UFormField label="Type" name="type">
            <USelect
              v-model="formState.type"
              :items="typeSelectItems"
              :disabled="!!editingIntegration"
            />
          </UFormField>

          <UFormField label="Name" name="name">
            <UInput v-model="formState.name" :placeholder="namePlaceholder" />
          </UFormField>

          <UFormField label="URL" name="url">
            <UInput v-model="formState.url" placeholder="http://localhost:8989" />
          </UFormField>

          <UFormField :label="formState.type === 'plex' ? 'Plex Token' : 'API Key'" name="apiKey">
            <UInput v-model="formState.apiKey" type="password" placeholder="Enter API key or token" />
          </UFormField>

          <div v-if="testResult" class="mt-2">
            <UAlert
              :icon="testResult.success ? 'i-heroicons-check-circle' : 'i-heroicons-x-circle'"
              :color="testResult.success ? 'success' : 'error'"
              :title="testResult.success ? 'Connection successful' : 'Connection failed'"
              :description="testResult.error || ''"
            />
          </div>

          <div v-if="formError" class="mt-2">
            <UAlert icon="i-heroicons-x-circle" color="error" :title="formError" />
          </div>
        </form>
      </template>

      <template #footer>
        <div class="flex items-center justify-between w-full">
          <UButton
            variant="soft"
            icon="i-heroicons-signal"
            :loading="testingForm"
            @click="testFormConnection"
          >
            Test Connection
          </UButton>
          <div class="flex gap-2">
            <UButton variant="ghost" @click="showModal = false">Cancel</UButton>
            <UButton color="primary" :loading="saving" @click="onSubmit">
              {{ editingIntegration ? 'Save' : 'Add' }}
            </UButton>
          </div>
        </div>
      </template>
    </UModal>

    <!-- Delete Confirmation Modal -->
    <UModal v-model:open="showDeleteModal" title="Delete Integration" :description="`Are you sure you want to delete ${deletingIntegration?.name}? This cannot be undone.`">
      <template #footer>
        <div class="flex justify-end gap-2 w-full">
          <UButton variant="ghost" @click="showDeleteModal = false">Cancel</UButton>
          <UButton color="error" :loading="deleting" @click="deleteIntegration">Delete</UButton>
        </div>
      </template>
    </UModal>
  </div>
</template>

<script setup lang="ts">
const api = useApi()
const token = useCookie('jwt')
const router = useRouter()
const toast = useToast()

// Auth guard
onMounted(() => {
  if (!token.value) {
    router.push('/login')
    return
  }
  fetchIntegrations()
})

// State
const loading = ref(true)
const integrations = ref<any[]>([])
const showModal = ref(false)
const showDeleteModal = ref(false)
const editingIntegration = ref<any>(null)
const deletingIntegration = ref<any>(null)
const saving = ref(false)
const deleting = ref(false)
const testingId = ref<number | null>(null)
const testingForm = ref(false)
const testResult = ref<any>(null)
const formError = ref('')

const formState = reactive({
  type: 'sonarr',
  name: '',
  url: '',
  apiKey: ''
})

const typeSelectItems = [
  { label: 'Sonarr', value: 'sonarr' },
  { label: 'Radarr', value: 'radarr' },
  { label: 'Plex', value: 'plex' }
]

const namePlaceholder = computed(() => {
  const defaults: Record<string, string> = { sonarr: 'My Sonarr', radarr: 'My Radarr', plex: 'My Plex' }
  return defaults[formState.type] || 'Integration Name'
})

// Type styling
function typeIcon(type: string) {
  const icons: Record<string, string> = {
    sonarr: 'i-heroicons-tv',
    radarr: 'i-heroicons-film',
    plex: 'i-heroicons-play-circle'
  }
  return icons[type] || 'i-heroicons-server'
}

function typeColor(type: string) {
  const colors: Record<string, string> = {
    sonarr: 'bg-sky-500',
    radarr: 'bg-amber-500',
    plex: 'bg-orange-500'
  }
  return colors[type] || 'bg-zinc-500'
}

function typeTextColor(type: string) {
  const colors: Record<string, string> = {
    sonarr: 'text-sky-500',
    radarr: 'text-amber-500',
    plex: 'text-orange-500'
  }
  return colors[type] || 'text-zinc-500'
}

function formatTime(dateStr: string) {
  if (!dateStr) return 'Never'
  return new Date(dateStr).toLocaleString()
}

// API calls
async function fetchIntegrations() {
  loading.value = true
  try {
    const data = await api('/api/v1/integrations')
    integrations.value = data
  } catch (e) {
    console.error('Failed to fetch integrations:', e)
  } finally {
    loading.value = false
  }
}

function openAddModal() {
  editingIntegration.value = null
  formState.type = 'sonarr'
  formState.name = ''
  formState.url = ''
  formState.apiKey = ''
  testResult.value = null
  formError.value = ''
  showModal.value = true
}

function openEditModal(integration: any) {
  editingIntegration.value = integration
  formState.type = integration.type
  formState.name = integration.name
  formState.url = integration.url
  formState.apiKey = ''
  testResult.value = null
  formError.value = ''
  showModal.value = true
}

async function onSubmit() {
  saving.value = true
  formError.value = ''
  try {
    if (editingIntegration.value) {
      await api(`/api/v1/integrations/${editingIntegration.value.id}`, {
        method: 'PUT',
        body: formState
      })
      toast.add({ title: 'Integration updated', icon: 'i-heroicons-check-circle', color: 'success' })
    } else {
      await api('/api/v1/integrations', {
        method: 'POST',
        body: formState
      })
      toast.add({ title: 'Integration added', icon: 'i-heroicons-check-circle', color: 'success' })
    }
    showModal.value = false
    await fetchIntegrations()
  } catch (e: any) {
    formError.value = e.data?.error || 'Failed to save integration'
  } finally {
    saving.value = false
  }
}

function confirmDelete(integration: any) {
  deletingIntegration.value = integration
  showDeleteModal.value = true
}

async function deleteIntegration() {
  if (!deletingIntegration.value) return
  deleting.value = true
  try {
    await api(`/api/v1/integrations/${deletingIntegration.value.id}`, {
      method: 'DELETE'
    })
    toast.add({ title: 'Integration deleted', icon: 'i-heroicons-trash', color: 'error' })
    showDeleteModal.value = false
    await fetchIntegrations()
  } catch (e) {
    console.error('Failed to delete:', e)
  } finally {
    deleting.value = false
  }
}

async function testConnection(integration: any) {
  testingId.value = integration.id
  try {
    const result = await api('/api/v1/integrations/test', {
      method: 'POST',
      body: { type: integration.type, url: integration.url, apiKey: integration.apiKey }
    })
    if (result.success) {
      toast.add({ title: `${integration.name}: Connected!`, icon: 'i-heroicons-check-circle', color: 'success' })
    } else {
      toast.add({ title: `${integration.name}: Failed`, description: result.error, icon: 'i-heroicons-x-circle', color: 'error' })
    }
  } catch (e) {
    toast.add({ title: `${integration.name}: Error`, icon: 'i-heroicons-x-circle', color: 'error' })
  } finally {
    testingId.value = null
  }
}

async function testFormConnection() {
  testingForm.value = true
  testResult.value = null
  try {
    testResult.value = await api('/api/v1/integrations/test', {
      method: 'POST',
      body: { type: formState.type, url: formState.url, apiKey: formState.apiKey }
    })
  } catch (e) {
    testResult.value = { success: false, error: 'Network error' }
  } finally {
    testingForm.value = false
  }
}
</script>
