/**
 * Engine control composable — shared state for execution mode and run status.
 * Used by the navbar engine popover and dashboard engine activity section.
 */
export function useEngineControl() {
  const api = useApi()
  const { addToast } = useToast()

  const workerStats = useState<any>('engineWorkerStats', () => null)
  const runNowLoading = ref(false)
  const changingMode = ref(false)

  const executionMode = computed(() => workerStats.value?.executionMode || 'dry_run')
  const lastRunEpoch = computed(() => workerStats.value?.lastRunEpoch || 0)
  const lastRunEvaluated = computed(() => workerStats.value?.lastRunEvaluated || 0)
  const lastRunFlagged = computed(() => workerStats.value?.lastRunFlagged || 0)
  const queueDepth = computed(() => workerStats.value?.queueDepth || 0)

  function modeLabel(mode: string): string {
    switch (mode) {
      case 'auto': return 'Auto'
      case 'approval': return 'Approval'
      default: return 'Dry-Run'
    }
  }

  async function fetchStats() {
    try {
      const stats = await api('/api/v1/worker/stats')
      if (stats) workerStats.value = stats
    } catch {
      // Silent — stats are a nice-to-have
    }
  }

  async function setMode(mode: string) {
    changingMode.value = true
    try {
      const currentPrefs = await api('/api/v1/preferences') as any
      await api('/api/v1/preferences', {
        method: 'PUT',
        body: { ...currentPrefs, executionMode: mode }
      })
      // Refresh stats to pick up the new mode
      await fetchStats()
      addToast(`Execution mode set to ${modeLabel(mode)}`, 'success')
    } catch (e) {
      console.error('Failed to set execution mode:', e)
      addToast('Failed to change execution mode', 'error')
    } finally {
      changingMode.value = false
    }
  }

  async function triggerRunNow() {
    runNowLoading.value = true
    try {
      await api('/api/v1/engine/run', { method: 'POST' })
      addToast('Engine run triggered', 'info')
      // Give the engine a moment, then refresh stats
      await new Promise(r => setTimeout(r, 2000))
      await fetchStats()
    } catch (e) {
      console.error('Failed to trigger engine run:', e)
      addToast('Failed to trigger engine run', 'error')
    } finally {
      runNowLoading.value = false
    }
  }

  return {
    workerStats: readonly(workerStats),
    executionMode,
    lastRunEpoch,
    lastRunEvaluated,
    lastRunFlagged,
    queueDepth,
    runNowLoading: readonly(runNowLoading),
    changingMode: readonly(changingMode),
    modeLabel,
    fetchStats,
    setMode,
    triggerRunNow,
  }
}
