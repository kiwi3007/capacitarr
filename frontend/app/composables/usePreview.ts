import type { EvaluatedItem, DiskContext, PreviewResponse } from '~/types/api';

/**
 * usePreview — SSE-reactive preview data composable.
 *
 * Fetches scored media items from `/api/v1/preview` and listens for
 * `preview_updated` and `preview_invalidated` SSE events to auto-refresh.
 *
 * Shared by library.vue (Library Management page) and rules.vue (Scoring
 * Engine live preview).
 */
export function usePreview() {
  const api = useApi();
  const { on, off } = useEventStream();

  const items = ref<EvaluatedItem[]>([]);
  const diskContext = ref<DiskContext | null>(null);
  const loading = ref(false);
  const stale = ref(false);

  // ---------------------------------------------------------------------------
  // Data fetching
  // ---------------------------------------------------------------------------

  async function refresh(force = false): Promise<void> {
    loading.value = true;
    try {
      const url = force ? '/api/v1/preview?force=true' : '/api/v1/preview';
      const data = (await api(url)) as PreviewResponse;
      items.value = data?.items ?? [];
      diskContext.value = data?.diskContext ?? null;
      stale.value = false;
    } catch (err) {
      console.warn('[usePreview] fetch failed:', err);
      items.value = [];
      diskContext.value = null;
    } finally {
      loading.value = false;
    }
  }

  // ---------------------------------------------------------------------------
  // SSE handlers
  // ---------------------------------------------------------------------------

  function handlePreviewUpdated() {
    // Cache has fresh data — fetch it (should be instant from cache)
    refresh();
  }

  function handlePreviewInvalidated() {
    // Cache was cleared due to config change — mark stale and request fresh data
    stale.value = true;
    refresh(true);
  }

  // ---------------------------------------------------------------------------
  // Lifecycle
  // ---------------------------------------------------------------------------

  onMounted(() => {
    on('preview_updated', handlePreviewUpdated);
    on('preview_invalidated', handlePreviewInvalidated);
  });

  onUnmounted(() => {
    off('preview_updated', handlePreviewUpdated);
    off('preview_invalidated', handlePreviewInvalidated);
  });

  return {
    /** Evaluated items from the preview cache. Mutable ref for component compatibility. */
    items,
    /** Disk context from the preview cache. */
    diskContext: readonly(diskContext),
    /** Whether a fetch is in progress. */
    loading: readonly(loading),
    /** Whether the cached data is stale (invalidated, awaiting refresh). */
    stale: readonly(stale),
    refresh,
  };
}
