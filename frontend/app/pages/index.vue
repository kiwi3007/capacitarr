<template>
  <div>
    <div class="mb-8 flex flex-col md:flex-row md:items-center justify-between gap-4">
      <div>
        <h1 class="text-3xl font-bold tracking-tight text-zinc-900 dark:text-white">Dashboard</h1>
        <p class="text-zinc-500 dark:text-zinc-400 mt-2">Capacity overview across your media storage.</p>
      </div>

      <div class="flex items-center gap-2">
        <USelect
          v-model="dateRange"
          :items="dateRangeOptions"
          class="w-36"
        />
        <USelect
          v-model="chartMode"
          :items="chartModeOptions"
          class="w-40"
        />
      </div>
    </div>

    <!-- Summary Cards -->
    <div class="grid grid-cols-1 md:grid-cols-3 gap-6 mb-8">
      <div class="rounded-xl border border-zinc-200 dark:border-zinc-800 bg-white dark:bg-zinc-900 shadow-sm p-5">
        <div class="flex items-center gap-2 text-violet-500 font-medium text-sm mb-3">
          <UIcon name="i-heroicons-server" />
          Total Storage
        </div>
        <div class="text-3xl font-bold text-zinc-900 dark:text-white">{{ formatBytes(totalCapacity) }}</div>
        <p class="text-sm text-zinc-500 dark:text-zinc-400 mt-1">
          {{ diskGroups.length }} disk group{{ diskGroups.length !== 1 ? 's' : '' }} mapped
        </p>
      </div>

      <div class="rounded-xl border border-zinc-200 dark:border-zinc-800 bg-white dark:bg-zinc-900 shadow-sm p-5">
        <div class="flex items-center gap-2 text-purple-500 font-medium text-sm mb-3">
          <UIcon name="i-heroicons-chart-pie" />
          Used Capacity
        </div>
        <div class="text-3xl font-bold text-zinc-900 dark:text-white">{{ formatBytes(totalUsed) }}</div>
        <p class="text-sm text-zinc-500 dark:text-zinc-400 mt-1">
          {{ totalCapacity > 0 ? Math.round((totalUsed / totalCapacity) * 100) : 0 }}% utilization
        </p>
      </div>

      <div class="rounded-xl border border-zinc-200 dark:border-zinc-800 bg-white dark:bg-zinc-900 shadow-sm p-5">
        <div class="flex items-center gap-2 text-fuchsia-500 font-medium text-sm mb-3">
          <UIcon name="i-heroicons-server-stack" />
          Integrations
        </div>
        <div class="text-3xl font-bold text-zinc-900 dark:text-white">{{ integrationCount }}</div>
        <p class="text-sm text-zinc-500 dark:text-zinc-400 mt-1">
          Active services connected
        </p>
      </div>
    </div>

    <!-- Per-Disk-Group Sections -->
    <div v-if="diskGroups.length > 0" class="space-y-6">
      <div
        v-for="group in diskGroups"
        :key="group.id"
        class="rounded-xl border border-zinc-200 dark:border-zinc-800 bg-white dark:bg-zinc-900 shadow-sm overflow-hidden"
      >
        <!-- Disk group header with progress -->
        <div class="p-5 border-b border-zinc-100 dark:border-zinc-800">
          <DiskGroupCard :group="group" />
        </div>
        <!-- Chart for this disk group -->
        <div class="h-64 p-4">
          <CapacityChart :mode="chartMode" :disk-group-id="group.id" :since="dateRange" />
        </div>
      </div>
    </div>

    <!-- Empty state -->
    <div v-else-if="!loading" class="mb-8">
      <div class="rounded-xl border border-dashed border-zinc-300 dark:border-zinc-700 p-8 text-center">
        <UIcon name="i-heroicons-server-stack" class="w-12 h-12 text-zinc-300 dark:text-zinc-600 mx-auto mb-3" />
        <h3 class="text-zinc-600 dark:text-zinc-400 font-medium mb-1">No disk groups yet</h3>
        <p class="text-sm text-zinc-400 dark:text-zinc-500 mb-4">
          Add integrations in <NuxtLink to="/settings" class="text-violet-500 hover:underline">Settings</NuxtLink> and data will appear on the next poll cycle.
        </p>
      </div>
    </div>

    <!-- Integration Usage Breakdown -->
    <div v-if="enabledIntegrations.length > 0" class="mt-6">
      <IntegrationUsage :integrations="enabledIntegrations" />
    </div>
  </div>
</template>

<script setup lang="ts">
const api = useApi()
const token = useCookie('jwt')
const router = useRouter()

const chartModeOptions = [
  { label: 'Percentage', value: 'percentage' },
  { label: 'Raw (GB)', value: 'raw' }
]

const dateRangeOptions = [
  { label: 'Last Hour', value: '1h' },
  { label: 'Last 24h', value: '24h' },
  { label: 'Last 7 Days', value: '7d' },
  { label: 'Last 30 Days', value: '30d' },
  { label: 'All Time', value: 'all' }
]

const chartMode = ref('percentage')
const dateRange = ref('24h')
const diskGroups = ref<any[]>([])
const allIntegrations = ref<any[]>([])
const loading = ref(true)

const enabledIntegrations = computed(() =>
  allIntegrations.value.filter((i: any) => i.enabled)
)

const integrationCount = computed(() => enabledIntegrations.value.length)

const totalCapacity = computed(() =>
  diskGroups.value.reduce((sum, g) => sum + (g.totalBytes || 0), 0)
)

const totalUsed = computed(() =>
  diskGroups.value.reduce((sum, g) => sum + (g.usedBytes || 0), 0)
)

onMounted(async () => {
  if (!token.value) {
    router.push('/login')
    return
  }
  await fetchDashboardData()
})

async function fetchDashboardData() {
  loading.value = true
  try {
    const [groups, integrations] = await Promise.all([
      api('/api/v1/disk-groups'),
      api('/api/v1/integrations')
    ])
    diskGroups.value = groups
    allIntegrations.value = integrations
  } catch (e) {
    console.error('Failed to fetch dashboard data:', e)
  } finally {
    loading.value = false
  }
}

function formatBytes(bytes: number): string {
  if (!bytes || bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  const val = bytes / Math.pow(1024, i)
  return `${val.toFixed(val >= 100 ? 0 : 1)} ${units[i]}`
}
</script>
