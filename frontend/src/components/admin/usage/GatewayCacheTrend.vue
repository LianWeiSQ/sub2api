<template>
  <div class="card p-4">
    <div class="mb-4 flex items-center justify-between gap-3">
      <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
        {{ t('admin.usage.gatewayCacheTrend') }}
      </h3>
      <span class="text-xs text-gray-500 dark:text-gray-400">
        {{ t('admin.usage.gatewayCacheTrendHint') }}
      </span>
    </div>
    <div v-if="loading" class="flex h-48 items-center justify-center">
      <LoadingSpinner />
    </div>
    <div v-else-if="hasCacheData && chartData" class="h-48">
      <Line :data="chartData" :options="lineOptions" />
    </div>
    <div
      v-else
      class="flex h-48 items-center justify-center text-sm text-gray-500 dark:text-gray-400"
    >
      {{ t('admin.usage.noGatewayCacheData') }}
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  CategoryScale,
  Chart as ChartJS,
  Filler,
  Legend,
  LinearScale,
  LineElement,
  PointElement,
  Title,
  Tooltip
} from 'chart.js'
import { Line } from 'vue-chartjs'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import type { TrendDataPoint } from '@/types'

ChartJS.register(
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Title,
  Tooltip,
  Legend,
  Filler
)

const props = defineProps<{
  trendData: TrendDataPoint[]
  loading?: boolean
}>()

const { t } = useI18n()

const isDarkMode = computed(() => document.documentElement.classList.contains('dark'))

const chartColors = computed(() => ({
  text: isDarkMode.value ? '#e5e7eb' : '#374151',
  grid: isDarkMode.value ? '#374151' : '#e5e7eb',
  savedTokens: '#10b981',
  upstreamReduction: '#3b82f6',
  hitRate: '#8b5cf6'
}))

const normalizeRateToPercent = (value: number | null | undefined): number => {
  if (!Number.isFinite(value ?? NaN)) return 0
  const n = Number(value)
  return n <= 1 ? n * 100 : n
}

const hasCacheData = computed(() =>
  props.trendData.some((point) =>
    (point.gateway_saved_tokens || 0) > 0 ||
    (point.gateway_cache_hits || 0) > 0 ||
    (point.gateway_cache_misses || 0) > 0 ||
    (point.gateway_cache_bypasses || 0) > 0 ||
    (point.gateway_cache_stores || 0) > 0
  )
)

const chartData = computed(() => {
  if (!props.trendData?.length) return null

  return {
    labels: props.trendData.map((d) => d.date),
    datasets: [
      {
        label: t('admin.usage.gatewaySavedTokens'),
        data: props.trendData.map((d) => d.gateway_saved_tokens || 0),
        borderColor: chartColors.value.savedTokens,
        backgroundColor: `${chartColors.value.savedTokens}20`,
        fill: true,
        tension: 0.3
      },
      {
        label: t('admin.usage.upstreamCallsAvoided'),
        data: props.trendData.map((d) => d.upstream_call_reduction || 0),
        borderColor: chartColors.value.upstreamReduction,
        backgroundColor: `${chartColors.value.upstreamReduction}20`,
        fill: false,
        tension: 0.3
      },
      {
        label: t('admin.usage.gatewayCacheHitRate'),
        data: props.trendData.map((d) => normalizeRateToPercent(d.gateway_cache_hit_rate)),
        borderColor: chartColors.value.hitRate,
        backgroundColor: `${chartColors.value.hitRate}20`,
        borderDash: [5, 5],
        fill: false,
        tension: 0.3,
        yAxisID: 'yPercent'
      }
    ]
  }
})

const lineOptions = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  interaction: {
    intersect: false,
    mode: 'index' as const
  },
  plugins: {
    legend: {
      position: 'top' as const,
      labels: {
        color: chartColors.value.text,
        usePointStyle: true,
        pointStyle: 'circle',
        padding: 15,
        font: { size: 11 }
      }
    },
    tooltip: {
      callbacks: {
        label: (context: any) => {
          if (context.dataset.yAxisID === 'yPercent') {
            return `${context.dataset.label}: ${context.raw.toFixed(1)}%`
          }
          return `${context.dataset.label}: ${formatCompact(context.raw)}`
        },
        footer: (tooltipItems: any) => {
          const dataIndex = tooltipItems[0]?.dataIndex
          const point = dataIndex != null ? props.trendData[dataIndex] : null
          if (!point) return ''
          const hit = point.gateway_cache_hits || 0
          const miss = point.gateway_cache_misses || 0
          const bypass = point.gateway_cache_bypasses || 0
          const store = point.gateway_cache_stores || 0
          return `${t('admin.usage.gatewayCacheHitShort')} ${hit} | ${t('admin.usage.gatewayCacheMissShort')} ${miss} | ${t('admin.usage.gatewayCacheBypassShort')} ${bypass} | ${t('admin.usage.gatewayCacheStoreShort')} ${store}`
        }
      }
    }
  },
  scales: {
    x: {
      grid: { color: chartColors.value.grid },
      ticks: {
        color: chartColors.value.text,
        font: { size: 10 }
      }
    },
    y: {
      grid: { color: chartColors.value.grid },
      ticks: {
        color: chartColors.value.text,
        font: { size: 10 },
        callback: (value: string | number) => formatCompact(Number(value))
      }
    },
    yPercent: {
      position: 'right' as const,
      min: 0,
      max: 100,
      grid: { drawOnChartArea: false },
      ticks: {
        color: chartColors.value.hitRate,
        font: { size: 10 },
        callback: (value: string | number) => `${value}%`
      }
    }
  }
}))

const formatCompact = (value: number): string => {
  if (value >= 1_000_000_000) return `${(value / 1_000_000_000).toFixed(2)}B`
  if (value >= 1_000_000) return `${(value / 1_000_000).toFixed(2)}M`
  if (value >= 1_000) return `${(value / 1_000).toFixed(2)}K`
  return value.toLocaleString()
}
</script>
