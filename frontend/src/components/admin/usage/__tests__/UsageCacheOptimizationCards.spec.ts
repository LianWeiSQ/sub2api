import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import UsageCacheOptimizationCards from '../UsageCacheOptimizationCards.vue'

const messages: Record<string, string> = {
  'admin.usage.gatewayCacheHitRate': 'Gateway cache hit rate',
  'admin.usage.gatewayCacheTarget': 'Target {target}',
  'admin.usage.gatewaySavedTokens': 'Saved tokens',
  'admin.usage.gatewaySavedTokenBreakdown': 'Input {input} / output {output}',
  'admin.usage.gatewaySavedCost': 'Saved cost',
  'admin.usage.upstreamCallReduction': 'Avoided {count} upstream calls',
  'admin.usage.gatewayCacheEvents': 'Cache events',
  'admin.usage.gatewayCacheHitShort': 'Hit',
  'admin.usage.gatewayCacheMissShort': 'Miss',
  'admin.usage.gatewayCacheBypassShort': 'Bypass',
  'admin.usage.gatewayCacheStoreShort': 'Store',
}

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, string>) => {
        let value = messages[key] ?? key
        if (params) {
          Object.entries(params).forEach(([name, paramValue]) => {
            value = value.replace(`{${name}}`, paramValue)
          })
        }
        return value
      },
    }),
  }
})

describe('UsageCacheOptimizationCards', () => {
  it('renders gateway cache savings and event counters', () => {
    const wrapper = mount(UsageCacheOptimizationCards, {
      props: {
        stats: {
          total_requests: 100,
          total_input_tokens: 0,
          total_output_tokens: 0,
          total_cache_tokens: 0,
          total_tokens: 0,
          total_cost: 0,
          total_actual_cost: 0,
          total_account_cost: 0,
          average_duration_ms: 0,
          gateway_cache_hits: 92,
          gateway_cache_misses: 6,
          gateway_cache_bypasses: 1,
          gateway_cache_stores: 1,
          gateway_cache_hit_rate: 0.92,
          gateway_saved_input_tokens: 15_000_000,
          gateway_saved_output_tokens: 270_000,
          gateway_saved_tokens: 15_270_000,
          gateway_saved_cost: 123.45,
          upstream_call_reduction: 92,
        },
      },
      global: {
        stubs: {
          Icon: true,
        },
      },
    })

    const text = wrapper.text()
    expect(text).toContain('92.0%')
    expect(text).toContain('15.27M')
    expect(text).toContain('$123.45')
    expect(text).toContain('Avoided 92 upstream calls')
    expect(text).toContain('Hit 92')
    expect(text).toContain('Miss 6')
  })
})
