<template>
  <div>
    <!-- Integration error banner -->
    <IntegrationErrorBanner :integrations="integrations" />

    <!-- Stale data indicator -->
    <div
      v-if="stale"
      data-slot="stale-indicator"
      class="bg-muted text-muted-foreground mb-4 flex items-center gap-2 rounded-md px-4 py-2 text-sm"
    >
      <svg
        class="size-4 animate-spin"
        xmlns="http://www.w3.org/2000/svg"
        fill="none"
        viewBox="0 0 24 24"
      >
        <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
        <path
          class="opacity-75"
          fill="currentColor"
          d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
        />
      </svg>
      {{ $t('preview.stale') }}
    </div>

    <!-- Library Table -->
    <LibraryTable
      ref="libraryTableRef"
      :items="items"
      :integrations="enabledIntegrations"
      :loading="loading"
      @refresh="refresh(true)"
      @force-delete="handleForceDelete"
    />
  </div>
</template>

<script setup lang="ts">
import type { IntegrationConfig, EvaluatedItem } from '~/types/api';

const api = useApi();
const { addToast } = useToast();
const { t } = useI18n();
const { items, loading, stale, refresh } = usePreview();

// ---------------------------------------------------------------------------
// Integrations
// ---------------------------------------------------------------------------
const integrations = ref<IntegrationConfig[]>([]);

const enabledIntegrations = computed(() => integrations.value.filter((i) => i.enabled));

async function fetchIntegrations() {
  try {
    integrations.value = (await api('/api/v1/integrations')) as IntegrationConfig[];
  } catch (err) {
    console.warn('[Library] fetchIntegrations failed:', err);
  }
}

// ---------------------------------------------------------------------------
// Force Delete
// ---------------------------------------------------------------------------
const libraryTableRef = ref<InstanceType<
  typeof import('~/components/LibraryTable.vue').default
> | null>(null);

async function handleForceDelete(selectedItems: EvaluatedItem[]) {
  try {
    const body = selectedItems.map((e) => ({
      mediaName: e.item.title,
      mediaType: e.item.type,
      integrationId: e.item.integrationId,
      externalId: e.item.externalId,
      sizeBytes: e.item.sizeBytes,
      reason: e.reason || `Score: ${e.score.toFixed(2)}`,
      scoreDetails: JSON.stringify(e.factors),
      posterUrl: e.item.posterUrl ?? '',
    }));

    const result = (await api('/api/v1/force-delete', {
      method: 'POST',
      body,
    })) as { queued: number; total: number };

    addToast(t('library.forceDeleteSuccess', { count: result.queued }), 'success');
    libraryTableRef.value?.onDeleteComplete();

    // Refresh to reflect changes
    await refresh(true);
  } catch (err: unknown) {
    const message = err instanceof Error ? err.message : String(err);
    addToast(`${t('library.forceDeleteError')}: ${message}`, 'error');
    libraryTableRef.value?.onDeleteComplete();
  }
}

// ---------------------------------------------------------------------------
// Init
// ---------------------------------------------------------------------------
onMounted(async () => {
  await Promise.all([fetchIntegrations(), refresh()]);
});
</script>
