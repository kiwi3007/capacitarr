<template>
  <div data-slot="cleanup-sparkline">
    <!-- No data -->
    <div
      v-if="!loading && noData"
      class="flex items-center justify-center h-[44px] text-[11px] text-muted-foreground/50"
    >
      No cleanup data
    </div>

    <!-- Sparkline -->
    <ClientOnly v-else-if="!loading && chartSeries.length > 0">
      <apexchart
        type="area"
        height="44"
        :options="chartOptions"
        :series="chartSeries"
      />
    </ClientOnly>
  </div>
</template>

<script setup lang="ts">
import type { CleanupHistoryItem, SparklineTooltipOpts } from '~/types/api'

type CleanupRange = '24h' | '7d' | '30d' | '90d'

const props = defineProps<{
  range: CleanupRange
}>()

const api = useApi()
const { primaryColor } = useThemeColors()

const loading = ref(false)
const noData = ref(false)
const series = ref<{ x: string; y: number }[]>([])

const chartSeries = computed(() => {
  if (series.value.length === 0) return []
  return [{ name: 'Items Deleted', data: series.value }]
})

const chartOptions = computed(() => ({
  chart: {
    type: 'area' as const,
    sparkline: { enabled: true },
    animations: { enabled: true, easing: 'easeinout', speed: 400 },
  },
  stroke: { curve: 'smooth' as const, width: 1.5 },
  colors: [primaryColor.value],
  fill: {
    type: 'gradient',
    gradient: {
      shadeIntensity: 1,
      opacityFrom: 0.4,
      opacityTo: 0.05,
      stops: [0, 100],
    },
  },
  tooltip: {
    enabled: true,
    x: { show: true },
    y: {
      formatter: (val: number, _opts: SparklineTooltipOpts) =>
        `${val} deleted`,
    },
    theme: 'dark',
  },
  xaxis: { type: 'category' as const },
}))

async function fetchCleanupHistory() {
  loading.value = true
  noData.value = false
  try {
    const data = (await api(`/api/v1/cleanup-history`, {
      query: { range: props.range },
    })) as CleanupHistoryItem[]

    if (!data || data.length === 0) {
      noData.value = true
      series.value = []
    } else {
      series.value = data.map((item) => ({
        x: item.timestamp,
        y: item.itemsDeleted,
      }))
    }
  } catch {
    noData.value = true
    series.value = []
  } finally {
    loading.value = false
  }
}

onMounted(() => fetchCleanupHistory())

watch(
  () => props.range,
  () => fetchCleanupHistory(),
)
</script>
