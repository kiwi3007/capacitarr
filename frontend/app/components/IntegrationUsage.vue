<template>
  <div class="rounded-xl border border-zinc-200 dark:border-zinc-800 bg-white dark:bg-zinc-900 shadow-sm p-5">
    <h3 class="text-lg font-semibold text-zinc-900 dark:text-white mb-4">Usage by Integration</h3>

    <div v-if="diskIntegrations.length === 0" class="text-center py-6 text-zinc-400">
      No integrations configured
    </div>

    <div v-else class="space-y-4">
      <div v-for="int_ in diskIntegrations" :key="int_.id" class="space-y-2">
        <div class="flex items-center justify-between text-sm">
          <div class="flex items-center gap-2">
            <span 
              class="w-3 h-3 rounded-full" 
              :style="{ backgroundColor: typeColor(int_.type) }"
            />
            <span class="font-medium text-zinc-700 dark:text-zinc-300">
              {{ int_.name }}
            </span>
            <span class="text-xs text-zinc-400 dark:text-zinc-500 uppercase">
              {{ int_.type }}
            </span>
          </div>
          <span class="text-zinc-500 dark:text-zinc-400 tabular-nums">
            {{ formatBytes(int_.mediaSizeBytes || 0) }}
            <span class="text-xs text-zinc-400 dark:text-zinc-500 ml-1">
              ({{ int_.mediaCount || 0 }} items)
            </span>
          </span>
        </div>

        <!-- Usage bar relative to total media -->
        <div class="h-2 rounded-full bg-zinc-100 dark:bg-zinc-800 overflow-hidden">
          <div 
            class="h-full rounded-full transition-all duration-500"
            :style="{
              width: barWidth(int_.mediaSizeBytes || 0) + '%',
              backgroundColor: typeColor(int_.type)
            }"
          />
        </div>
      </div>

      <!-- Total -->
      <div class="pt-3 border-t border-zinc-100 dark:border-zinc-800 flex justify-between text-sm font-medium">
        <span class="text-zinc-600 dark:text-zinc-400">Total Media</span>
        <span class="text-zinc-900 dark:text-white tabular-nums">{{ formatBytes(totalMedia) }}</span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
const props = defineProps<{
  integrations: any[]
}>()

// Plex is only used for protection rules, not disk usage tracking
const diskIntegrations = computed(() =>
  props.integrations.filter(i => i.type !== 'plex')
)

const typeColor = (type: string): string => {
  switch (type) {
    case 'sonarr': return '#8b5cf6'  // violet
    case 'radarr': return '#f59e0b'  // amber
    case 'plex':   return '#e9a211'  // gold
    default:       return '#6b7280'  // gray
  }
}

const totalMedia = computed(() =>
  diskIntegrations.value.reduce((sum, i) => sum + (i.mediaSizeBytes || 0), 0)
)

const barWidth = (bytes: number): number => {
  if (totalMedia.value === 0) return 0
  return Math.max(1, (bytes / totalMedia.value) * 100)
}

function formatBytes(bytes: number): string {
  if (!bytes || bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  const i = Math.floor(Math.log(Math.abs(bytes)) / Math.log(1024))
  const val = bytes / Math.pow(1024, i)
  return `${val.toFixed(val >= 100 ? 0 : 1)} ${units[i]}`
}
</script>
