<template>
  <div class="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-4">
    <div class="card p-4">
      <div class="flex items-start justify-between gap-3">
        <div>
          <p class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.usage.gatewayCacheHitRate') }}</p>
          <p class="mt-1 text-2xl font-bold text-gray-900 dark:text-white">{{ formatPercent(hitRatePercent) }}</p>
        </div>
        <div class="rounded-lg bg-sky-100 p-2 text-sky-600 dark:bg-sky-900/30 dark:text-sky-400">
          <Icon name="database" size="md" />
        </div>
      </div>
      <div class="mt-3 h-1.5 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-700">
        <div
          class="h-full rounded-full bg-sky-500 transition-all"
          :style="{ width: `${Math.min(hitRatePercent, 100)}%` }"
        ></div>
      </div>
      <p class="mt-2 text-xs text-gray-500 dark:text-gray-400">
        {{ t('admin.usage.gatewayCacheTarget', { target: formatPercent(targetHitRatePercent) }) }}
      </p>
    </div>

    <div class="card p-4">
      <div class="flex items-start justify-between gap-3">
        <div>
          <p class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.usage.gatewaySavedTokens') }}</p>
          <p class="mt-1 text-2xl font-bold text-gray-900 dark:text-white">{{ formatTokens(savedTokens) }}</p>
        </div>
        <div class="rounded-lg bg-emerald-100 p-2 text-emerald-600 dark:bg-emerald-900/30 dark:text-emerald-400">
          <Icon name="bolt" size="md" />
        </div>
      </div>
      <p class="mt-2 text-xs text-gray-500 dark:text-gray-400">
        {{ t('admin.usage.gatewaySavedTokenBreakdown', {
          input: formatTokens(savedInputTokens),
          output: formatTokens(savedOutputTokens),
        }) }}
      </p>
    </div>

    <div class="card p-4">
      <div class="flex items-start justify-between gap-3">
        <div>
          <p class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.usage.gatewaySavedCost') }}</p>
          <p class="mt-1 text-2xl font-bold text-emerald-600 dark:text-emerald-400">${{ formatCost(savedCost) }}</p>
        </div>
        <div class="rounded-lg bg-green-100 p-2 text-green-600 dark:bg-green-900/30 dark:text-green-400">
          <Icon name="dollar" size="md" />
        </div>
      </div>
      <p class="mt-2 text-xs text-gray-500 dark:text-gray-400">
        {{ t('admin.usage.upstreamCallReduction', { count: formatInteger(upstreamCallReduction) }) }}
      </p>
    </div>

    <div class="card p-4">
      <div class="flex items-start justify-between gap-3">
        <div>
          <p class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.usage.gatewayCacheEvents') }}</p>
          <p class="mt-1 text-2xl font-bold text-gray-900 dark:text-white">{{ formatInteger(totalEvents) }}</p>
        </div>
        <div class="rounded-lg bg-violet-100 p-2 text-violet-600 dark:bg-violet-900/30 dark:text-violet-400">
          <Icon name="chartBar" size="md" />
        </div>
      </div>
      <div class="mt-2 flex flex-wrap gap-x-3 gap-y-1 text-xs text-gray-500 dark:text-gray-400">
        <span>{{ t('admin.usage.gatewayCacheHitShort') }} {{ formatInteger(stats?.gateway_cache_hits || 0) }}</span>
        <span>{{ t('admin.usage.gatewayCacheMissShort') }} {{ formatInteger(stats?.gateway_cache_misses || 0) }}</span>
        <span>{{ t('admin.usage.gatewayCacheBypassShort') }} {{ formatInteger(stats?.gateway_cache_bypasses || 0) }}</span>
        <span>{{ t('admin.usage.gatewayCacheStoreShort') }} {{ formatInteger(stats?.gateway_cache_stores || 0) }}</span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import type { AdminUsageStatsResponse } from '@/api/admin/usage'

const props = withDefaults(defineProps<{
  stats: AdminUsageStatsResponse | null
  targetHitRate?: number
}>(), {
  targetHitRate: 0.9
})

const { t } = useI18n()

const normalizeRateToPercent = (value: number | null | undefined): number => {
  if (!Number.isFinite(value ?? NaN)) return 0
  const n = Number(value)
  return n <= 1 ? n * 100 : n
}

const savedInputTokens = computed(() => props.stats?.gateway_saved_input_tokens || 0)
const savedOutputTokens = computed(() => props.stats?.gateway_saved_output_tokens || 0)
const savedTokens = computed(() =>
  props.stats?.gateway_saved_tokens ?? savedInputTokens.value + savedOutputTokens.value
)
const savedCost = computed(() => props.stats?.gateway_saved_cost || 0)
const upstreamCallReduction = computed(() => props.stats?.upstream_call_reduction || 0)
const hitRatePercent = computed(() => normalizeRateToPercent(props.stats?.gateway_cache_hit_rate))
const targetHitRatePercent = computed(() => normalizeRateToPercent(props.targetHitRate))
const totalEvents = computed(() =>
  (props.stats?.gateway_cache_hits || 0) +
  (props.stats?.gateway_cache_misses || 0) +
  (props.stats?.gateway_cache_bypasses || 0) +
  (props.stats?.gateway_cache_stores || 0)
)

const formatInteger = (value: number): string => Math.max(0, value).toLocaleString()

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

const formatPercent = (value: number): string => `${Math.max(0, value).toFixed(1)}%`
</script>
