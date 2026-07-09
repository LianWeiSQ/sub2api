<template>
  <div class="card p-4">
    <div class="mb-4 flex flex-wrap items-start justify-between gap-3">
      <div>
        <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
          {{ t('admin.usage.litellmBenchmarkTitle') }}
        </h3>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.usage.litellmBenchmarkDescription') }}
        </p>
      </div>
      <div class="text-right text-xs text-gray-500 dark:text-gray-400">
        <div>{{ t('admin.usage.sampleSize') }}: {{ summary.sample_size }}</div>
        <div v-if="data?.generated_at">{{ formatGeneratedAt(data.generated_at) }}</div>
      </div>
    </div>

    <div v-if="loading" class="flex h-48 items-center justify-center">
      <LoadingSpinner />
    </div>

    <div v-else-if="rows.length === 0" class="flex h-48 items-center justify-center text-sm text-gray-500 dark:text-gray-400">
      {{ t('admin.usage.noBenchmarkData') }}
    </div>

    <div v-else class="space-y-4">
      <div class="grid grid-cols-2 gap-3 lg:grid-cols-4">
        <div>
          <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.usage.benchmarkSavedTokens') }}</p>
          <p class="text-lg font-semibold text-gray-900 dark:text-white">{{ formatTokens(summary.saved_tokens) }}</p>
        </div>
        <div>
          <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.usage.benchmarkHitRate') }}</p>
          <p class="text-lg font-semibold text-gray-900 dark:text-white">{{ formatPercent(summary.cache_hit_rate || 0) }}</p>
        </div>
        <div>
          <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.usage.benchmarkSavedCost') }}</p>
          <p class="text-lg font-semibold text-emerald-600 dark:text-emerald-400">${{ formatCost(summary.saved_cost || 0) }}</p>
        </div>
        <div>
          <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.usage.benchmarkLatencyDelta') }}</p>
          <p class="text-lg font-semibold text-gray-900 dark:text-white">{{ formatLatencyDelta(summary.average_latency_delta_ms || 0) }}</p>
        </div>
      </div>

      <div class="overflow-x-auto">
        <table class="w-full min-w-[760px] divide-y divide-gray-200 text-sm dark:divide-dark-700">
          <thead class="bg-gray-50 dark:bg-dark-800">
            <tr>
              <th class="px-3 py-2 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">{{ t('admin.usage.sample') }}</th>
              <th class="px-3 py-2 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">{{ t('admin.usage.scenario') }}</th>
              <th class="px-3 py-2 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">{{ t('admin.usage.baselineTokens') }}</th>
              <th class="px-3 py-2 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">{{ t('admin.usage.litellmTokens') }}</th>
              <th class="px-3 py-2 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">{{ t('admin.usage.savedTokens') }}</th>
              <th class="px-3 py-2 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">{{ t('admin.usage.cacheStatus') }}</th>
              <th class="px-3 py-2 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">{{ t('admin.usage.latencyDelta') }}</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-100 bg-white dark:divide-dark-700 dark:bg-dark-900">
            <tr v-for="row in rows" :key="row.id" class="hover:bg-gray-50 dark:hover:bg-dark-800">
              <td class="px-3 py-2 text-gray-900 dark:text-white">
                <div class="max-w-[180px] truncate font-medium" :title="row.name">{{ row.name }}</div>
                <div class="text-xs text-gray-500 dark:text-gray-400">{{ row.id }}</div>
              </td>
              <td class="px-3 py-2 text-gray-600 dark:text-gray-300">
                <div>{{ row.category }}</div>
                <div v-if="row.scenario" class="text-xs text-gray-500 dark:text-gray-400">{{ row.scenario }}</div>
              </td>
              <td class="px-3 py-2 text-right text-gray-700 dark:text-gray-300">{{ formatTokens(row.baseline.total_tokens) }}</td>
              <td class="px-3 py-2 text-right text-gray-700 dark:text-gray-300">{{ formatTokens(row.litellm.total_tokens) }}</td>
              <td class="px-3 py-2 text-right font-medium text-emerald-600 dark:text-emerald-400">{{ formatTokens(resolveSavedTokens(row)) }}</td>
              <td class="px-3 py-2">
                <span class="inline-flex items-center rounded px-2 py-0.5 text-xs font-medium" :class="getStatusClass(row.gateway_cache_status)">
                  {{ getStatusLabel(row.gateway_cache_status) }}
                </span>
              </td>
              <td class="px-3 py-2 text-right text-gray-700 dark:text-gray-300">{{ formatLatencyDelta(row.latency_delta_ms || 0) }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import type {
  BaselineVsLiteLLMBenchmarkResponse,
  LiteLLMBenchmarkSampleResult,
  LiteLLMBenchmarkSummary
} from '@/api/admin/usage'

const props = defineProps<{
  data: BaselineVsLiteLLMBenchmarkResponse | null
  loading?: boolean
}>()

const { t } = useI18n()

const rows = computed(() => props.data?.baseline_vs_litellm || [])

const normalizeRateToPercent = (value: number | null | undefined): number => {
  if (!Number.isFinite(value ?? NaN)) return 0
  const n = Number(value)
  return n <= 1 ? n * 100 : n
}

const resolveSavedTokens = (row: LiteLLMBenchmarkSampleResult): number => {
  if (typeof row.saved_tokens === 'number') return row.saved_tokens
  return Math.max(0, (row.baseline?.total_tokens || 0) - (row.litellm?.total_tokens || 0))
}

const summary = computed<LiteLLMBenchmarkSummary>(() => {
  const apiSummary = props.data?.summary
  const sampleSize = apiSummary?.sample_size ?? rows.value.length
  const baselineTokens = apiSummary?.baseline_tokens ?? rows.value.reduce((sum, row) => sum + (row.baseline?.total_tokens || 0), 0)
  const litellmTokens = apiSummary?.litellm_tokens ?? rows.value.reduce((sum, row) => sum + (row.litellm?.total_tokens || 0), 0)
  const savedTokens = apiSummary?.saved_tokens ?? rows.value.reduce((sum, row) => sum + resolveSavedTokens(row), 0)
  const savedCost = apiSummary?.saved_cost ?? rows.value.reduce((sum, row) => sum + (row.saved_cost || 0), 0)
  const hitRate = apiSummary?.cache_hit_rate ?? (
    rows.value.length > 0
      ? rows.value.filter((row) => row.gateway_cache_status === 'hit').length / rows.value.length
      : 0
  )
  const averageLatencyDelta = apiSummary?.average_latency_delta_ms ?? (
    rows.value.length > 0
      ? rows.value.reduce((sum, row) => sum + (row.latency_delta_ms || 0), 0) / rows.value.length
      : 0
  )

  return {
    sample_size: sampleSize,
    baseline_tokens: baselineTokens,
    litellm_tokens: litellmTokens,
    saved_tokens: savedTokens,
    saved_cost: savedCost,
    cache_hit_rate: hitRate,
    average_latency_delta_ms: averageLatencyDelta,
    upstream_call_reduction: apiSummary?.upstream_call_reduction || 0
  }
})

const getStatusLabel = (status?: string | null): string => {
  if (status === 'hit') return t('admin.usage.gatewayCacheStatus.hit')
  if (status === 'miss') return t('admin.usage.gatewayCacheStatus.miss')
  if (status === 'bypass') return t('admin.usage.gatewayCacheStatus.bypass')
  if (status === 'store') return t('admin.usage.gatewayCacheStatus.store')
  if (status === 'disabled') return t('admin.usage.gatewayCacheStatus.disabled')
  return t('admin.usage.gatewayCacheStatus.unknown')
}

const getStatusClass = (status?: string | null): string => {
  if (status === 'hit') return 'bg-emerald-100 text-emerald-800 dark:bg-emerald-900/40 dark:text-emerald-200'
  if (status === 'miss') return 'bg-amber-100 text-amber-800 dark:bg-amber-900/40 dark:text-amber-200'
  if (status === 'bypass') return 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-200'
  if (status === 'store') return 'bg-sky-100 text-sky-800 dark:bg-sky-900/40 dark:text-sky-200'
  return 'bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-200'
}

const formatTokens = (value: number): string => {
  const n = Math.max(0, value)
  if (n >= 1_000_000_000) return `${(n / 1_000_000_000).toFixed(2)}B`
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(2)}M`
  if (n >= 1_000) return `${(n / 1_000).toFixed(2)}K`
  return n.toLocaleString()
}

const formatCost = (value: number): string => {
  if (value >= 1000) return `${(value / 1000).toFixed(2)}K`
  if (value >= 1) return value.toFixed(2)
  if (value >= 0.01) return value.toFixed(3)
  return value.toFixed(4)
}

const formatPercent = (value: number): string => `${normalizeRateToPercent(value).toFixed(1)}%`

const formatLatencyDelta = (value: number): string => {
  const sign = value > 0 ? '+' : ''
  return `${sign}${value.toFixed(0)}ms`
}

const formatGeneratedAt = (value: string): string => {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString()
}
</script>
