<template>
  <div class="rounded-xl border border-zinc-200 dark:border-zinc-800 bg-white dark:bg-zinc-900 shadow-sm p-5">
    <div class="flex items-center justify-between mb-4">
      <div class="flex items-center gap-3">
        <div class="w-10 h-10 rounded-lg flex items-center justify-center" :class="statusColor">
          <UIcon name="i-heroicons-server-stack" class="w-5 h-5 text-white" />
        </div>
        <div>
          <h3 class="font-semibold text-zinc-900 dark:text-white text-sm">{{ group.mountPath }}</h3>
          <span class="text-xs text-zinc-500 dark:text-zinc-400">
            {{ formatBytes(group.usedBytes) }} / {{ formatBytes(group.totalBytes) }}
          </span>
        </div>
      </div>
      <span class="text-2xl font-bold" :class="percentColor">{{ usagePercent }}%</span>
    </div>

    <!-- Progress Bar -->
    <div class="w-full h-3 bg-zinc-100 dark:bg-zinc-800 rounded-full overflow-hidden">
      <div
        class="h-full rounded-full transition-all duration-500 ease-out"
        :class="barColor"
        :style="{ width: usagePercent + '%' }"
      />
    </div>

    <!-- Threshold markers -->
    <div class="flex items-center justify-between mt-2 text-xs text-zinc-400 dark:text-zinc-500">
      <span>{{ formatBytes(freeBytes) }} free</span>
      <div class="flex items-center gap-3">
        <span v-if="group.thresholdPct" class="flex items-center gap-1">
          <span class="w-2 h-2 rounded-full bg-amber-400" />
          Target: {{ group.targetPct }}%
        </span>
        <span v-if="group.thresholdPct" class="flex items-center gap-1">
          <span class="w-2 h-2 rounded-full bg-red-400" />
          Threshold: {{ group.thresholdPct }}%
        </span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
interface DiskGroup {
  id: number
  mountPath: string
  totalBytes: number
  usedBytes: number
  thresholdPct: number
  targetPct: number
}

const props = defineProps<{
  group: DiskGroup
}>()

const usagePercent = computed(() => {
  if (props.group.totalBytes === 0) return 0
  return Math.round((props.group.usedBytes / props.group.totalBytes) * 100)
})

const freeBytes = computed(() => props.group.totalBytes - props.group.usedBytes)

const statusColor = computed(() => {
  const pct = usagePercent.value
  if (pct >= (props.group.thresholdPct || 85)) return 'bg-red-500'
  if (pct >= (props.group.targetPct || 75)) return 'bg-amber-500'
  return 'bg-violet-500'
})

const percentColor = computed(() => {
  const pct = usagePercent.value
  if (pct >= (props.group.thresholdPct || 85)) return 'text-red-500'
  if (pct >= (props.group.targetPct || 75)) return 'text-amber-500'
  return 'text-violet-500'
})

const barColor = computed(() => {
  const pct = usagePercent.value
  if (pct >= (props.group.thresholdPct || 85)) return 'bg-gradient-to-r from-red-400 to-red-500'
  if (pct >= (props.group.targetPct || 75)) return 'bg-gradient-to-r from-amber-400 to-amber-500'
  return 'bg-gradient-to-r from-violet-400 to-violet-500'
})

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  const val = bytes / Math.pow(1024, i)
  return `${val.toFixed(val >= 100 ? 0 : 1)} ${units[i]}`
}
</script>
