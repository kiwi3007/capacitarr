<template>
  <div class="h-full w-full">
    <div v-if="loading" class="h-full flex items-center justify-center">
      <UIcon name="i-heroicons-arrow-path" class="w-8 h-8 text-violet-500 animate-spin" />
    </div>
    <div v-else-if="error" class="h-full flex flex-col items-center justify-center text-red-500">
      <UIcon name="i-heroicons-exclamation-triangle" class="w-8 h-8 mb-2" />
      <span>Error loading metrics</span>
    </div>
    <div v-else-if="noData" class="h-full flex flex-col items-center justify-center text-zinc-400 dark:text-zinc-500">
      <UIcon name="i-heroicons-chart-bar" class="w-8 h-8 mb-2" />
      <span class="text-sm">Waiting for data…</span>
    </div>
    <ClientOnly v-else>
      <apexchart 
        type="area" 
        height="100%" 
        :options="chartOptions" 
        :series="series" 
      />
    </ClientOnly>
  </div>
</template>

<script setup lang="ts">
const props = defineProps<{
  mode: string       // 'percentage' or 'raw'
  diskGroupId?: number
  since?: string     // '1h', '24h', '7d', '30d', or '' for all
}>()

const api = useApi()
const loading = ref(true)
const error = ref(false)
const noData = ref(false)

const colorMode = useColorMode()

const series = ref<any[]>([])

const chartOptions = computed(() => {
  const isDark = colorMode.value === 'dark'
  const textColor = isDark ? '#a1a1aa' : '#71717a'
  const gridColor = isDark ? '#3f3f46' : '#e4e4e7'
  const isPercent = props.mode === 'percentage'

  return {
    chart: {
      type: 'area',
      height: '100%',
      toolbar: { show: false },
      zoom: { enabled: false },
      background: 'transparent',
      fontFamily: 'inherit',
      animations: {
        enabled: true,
        easing: 'easeinout',
        speed: 400
      }
    },
    colors: props.mode === 'raw' ? ['#8b5cf6', '#10b981'] : ['#8b5cf6'], // violet + emerald
    fill: {
      type: 'gradient',
      gradient: {
        shadeIntensity: 1,
        opacityFrom: 0.4,
        opacityTo: 0.05,
        stops: [0, 90, 100]
      }
    },
    dataLabels: { enabled: false },
    stroke: {
      curve: 'smooth',
      width: 2
    },
    xaxis: {
      type: 'datetime',
      labels: {
        style: { colors: textColor },
        datetimeUTC: false,
        format: 'MMM dd HH:mm'
      },
      axisBorder: { show: false },
      axisTicks: { show: false },
      tooltip: {
        enabled: true,
        formatter: (_val: string, opts: any) => {
          const ts = opts.w.globals.seriesX[opts.seriesIndex][opts.dataPointIndex]
          return new Date(ts).toLocaleString()
        }
      }
    },
    yaxis: {
      min: isPercent ? 0 : undefined,
      max: isPercent ? 100 : undefined,
      labels: {
        style: { colors: textColor },
        formatter: (value: number) => {
          if (isPercent) {
            return `${value.toFixed(0)}%`
          }
          return formatBytes(value)
        }
      }
    },
    grid: {
      borderColor: gridColor,
      strokeDashArray: 4,
      xaxis: { lines: { show: true } },
      yaxis: { lines: { show: true } }
    },
    theme: {
      mode: isDark ? 'dark' : 'light'
    },
    tooltip: {
      theme: isDark ? 'dark' : 'light',
      x: {
        format: 'MMM dd, yyyy HH:mm:ss'
      },
      y: {
        formatter: (value: number) => {
          if (isPercent) {
            return `${value.toFixed(1)}%`
          }
          return formatBytes(value)
        }
      }
    },
    legend: {
      show: !isPercent,
      position: 'top',
      horizontalAlign: 'right',
      labels: {
        colors: textColor
      }
    }
  }
})

function formatBytes(bytes: number): string {
  if (!bytes || bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  const i = Math.floor(Math.log(Math.abs(bytes)) / Math.log(1024))
  const val = bytes / Math.pow(1024, i)
  return `${val.toFixed(val >= 100 ? 0 : 1)} ${units[i]}`
}

async function fetchMetrics() {
  loading.value = true
  error.value = false
  noData.value = false
  try {
    const query: Record<string, any> = { resolution: 'raw' }
    if (props.diskGroupId) {
      query.disk_group_id = props.diskGroupId
    }
    if (props.since && props.since !== 'all') {
      query.since = props.since
    }

    const res = await api('/api/v1/metrics/history', { query })
    
    if (res.status === 'success' && res.data && res.data.length > 0) {
      const isPercent = props.mode === 'percentage'

      const usedData = res.data.map((row: any) => {
        const ts = new Date(row.Timestamp).getTime()
        if (isPercent) {
          const pct = row.TotalCapacity > 0
            ? (row.UsedCapacity / row.TotalCapacity) * 100
            : 0
          return [ts, Math.round(pct * 10) / 10]
        }
        return [ts, row.UsedCapacity]
      })

      if (isPercent) {
        series.value = [
          { name: 'Used %', data: usedData }
        ]
      } else {
        // Raw mode: show both Used (violet) and Total (emerald)
        const totalData = res.data.map((row: any) => [
          new Date(row.Timestamp).getTime(),
          row.TotalCapacity
        ])
        series.value = [
          { name: 'Used', data: usedData },
          { name: 'Total', data: totalData }
        ]
      }
    } else {
      noData.value = true
    }
  } catch (err) {
    console.error('Failed to grab history data:', err)
    error.value = true
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  fetchMetrics()
})

watch(() => [props.mode, props.diskGroupId, props.since], () => {
  fetchMetrics()
})
</script>
