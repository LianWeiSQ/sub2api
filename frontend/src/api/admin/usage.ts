/**
 * Admin Usage API endpoints
 * Handles admin-level usage logs and statistics retrieval
 */

import { apiClient } from '../client'
import type { AdminUsageLog, UsageQueryParams, PaginatedResponse, UsageRequestType } from '@/types'
import type { EndpointStat } from '@/types'

// ==================== Types ====================

export interface AdminUsageStatsResponse {
  total_requests: number
  total_input_tokens: number
  total_output_tokens: number
  total_cache_tokens: number
  total_cache_creation_tokens: number
  total_cache_read_tokens: number
  total_tokens: number
  total_cost: number
  total_actual_cost: number
  total_account_cost: number
  average_duration_ms: number
  gateway_cache_hits?: number
  gateway_cache_misses?: number
  gateway_cache_bypasses?: number
  gateway_cache_stores?: number
  gateway_cache_hit_rate?: number
  gateway_saved_input_tokens?: number
  gateway_saved_output_tokens?: number
  gateway_saved_tokens?: number
  gateway_saved_cost?: number
  upstream_call_reduction?: number
  endpoints?: EndpointStat[]
  upstream_endpoints?: EndpointStat[]
  endpoint_paths?: EndpointStat[]
}

export interface LiteLLMBenchmarkRunMetrics {
  input_tokens: number
  output_tokens: number
  total_tokens: number
  cost?: number
  duration_ms?: number
  first_token_ms?: number | null
  upstream_calls?: number
}

export interface LiteLLMBenchmarkSampleResult {
  id: string
  name: string
  category: string
  scenario?: string
  baseline: LiteLLMBenchmarkRunMetrics
  litellm: LiteLLMBenchmarkRunMetrics
  gateway_cache_status?: string | null
  gateway_cache_hit_rate?: number
  saved_input_tokens?: number
  saved_output_tokens?: number
  saved_tokens?: number
  saved_cost?: number
  latency_delta_ms?: number
  notes?: string | null
}

export interface LiteLLMBenchmarkSummary {
  sample_size: number
  baseline_tokens: number
  litellm_tokens: number
  saved_tokens: number
  saved_cost?: number
  cache_hit_rate?: number
  average_latency_delta_ms?: number
  upstream_call_reduction?: number
}

export interface BaselineVsLiteLLMBenchmarkResponse {
  generated_at?: string
  target_cache_hit_rate?: number
  summary?: Partial<LiteLLMBenchmarkSummary>
  baseline_vs_litellm: LiteLLMBenchmarkSampleResult[]
}

export interface SimpleUser {
  id: number
  email: string
  deleted: boolean
}

export interface SimpleApiKey {
  id: number
  name: string
  user_id: number
}

export interface UsageCleanupFilters {
  start_time: string
  end_time: string
  user_id?: number
  api_key_id?: number
  account_id?: number
  group_id?: number
  model?: string | null
  request_type?: UsageRequestType | null
  stream?: boolean | null
  billing_type?: number | null
}

export interface UsageCleanupTask {
  id: number
  status: string
  filters: UsageCleanupFilters
  created_by: number
  deleted_rows: number
  error_message?: string | null
  canceled_by?: number | null
  canceled_at?: string | null
  started_at?: string | null
  finished_at?: string | null
  created_at: string
  updated_at: string
}

export interface CreateUsageCleanupTaskRequest {
  start_date: string
  end_date: string
  user_id?: number
  api_key_id?: number
  account_id?: number
  group_id?: number
  model?: string | null
  request_type?: UsageRequestType | null
  stream?: boolean | null
  billing_type?: number | null
  timezone?: string
}

export interface AdminUsageQueryParams extends UsageQueryParams {
  user_id?: number
  exact_total?: boolean
  billing_mode?: string
  sort_by?: string
  sort_order?: 'asc' | 'desc'
}

// ==================== API Functions ====================

/**
 * List all usage logs with optional filters (admin only)
 * @param params - Query parameters for filtering and pagination
 * @returns Paginated list of usage logs
 */
export async function list(
  params: AdminUsageQueryParams,
  options?: { signal?: AbortSignal }
): Promise<PaginatedResponse<AdminUsageLog>> {
  const { data } = await apiClient.get<PaginatedResponse<AdminUsageLog>>('/admin/usage', {
    params,
    signal: options?.signal
  })
  return data
}

/**
 * Get usage statistics with optional filters (admin only)
 * @param params - Query parameters for filtering
 * @returns Usage statistics
 */
export async function getStats(params: {
  user_id?: number
  api_key_id?: number
  account_id?: number
  group_id?: number
  model?: string
  request_type?: UsageRequestType
  stream?: boolean
  period?: string
  start_date?: string
  end_date?: string
  timezone?: string
  nocache?: number
}): Promise<AdminUsageStatsResponse> {
  const { data } = await apiClient.get<AdminUsageStatsResponse>('/admin/usage/stats', {
    params
  })
  return data
}

/**
 * Get optional baseline-vs-LiteLLM benchmark results for cache optimization.
 * Backends that have not enabled the benchmark endpoint may return 404; callers
 * should treat that as "no benchmark data" rather than a page-level failure.
 */
export async function getBaselineVsLiteLLMBenchmark(params?: {
  start_date?: string
  end_date?: string
  model?: string
  group_id?: number
}): Promise<BaselineVsLiteLLMBenchmarkResponse> {
  const { data } = await apiClient.get<BaselineVsLiteLLMBenchmarkResponse>(
    '/admin/usage/benchmark/baseline-vs-litellm',
    { params }
  )
  return data
}

/**
 * Search users by email keyword (admin only)
 * @param keyword - Email keyword to search
 * @returns List of matching users (max 30)
 */
export async function searchUsers(keyword: string): Promise<SimpleUser[]> {
  const { data } = await apiClient.get<SimpleUser[]>('/admin/usage/search-users', {
    params: { q: keyword }
  })
  return data
}

/**
 * Search API keys by user ID and/or keyword (admin only)
 * @param userId - Optional user ID to filter by
 * @param keyword - Optional keyword to search in key name
 * @returns List of matching API keys (max 30)
 */
export async function searchApiKeys(userId?: number, keyword?: string): Promise<SimpleApiKey[]> {
  const params: Record<string, unknown> = {}
  if (userId !== undefined) {
    params.user_id = userId
  }
  if (keyword) {
    params.q = keyword
  }
  const { data } = await apiClient.get<SimpleApiKey[]>('/admin/usage/search-api-keys', {
    params
  })
  return data
}

/**
 * List usage cleanup tasks (admin only)
 * @param params - Query parameters for pagination
 * @returns Paginated list of cleanup tasks
 */
export async function listCleanupTasks(
  params: { page?: number; page_size?: number },
  options?: { signal?: AbortSignal }
): Promise<PaginatedResponse<UsageCleanupTask>> {
  const { data } = await apiClient.get<PaginatedResponse<UsageCleanupTask>>('/admin/usage/cleanup-tasks', {
    params,
    signal: options?.signal
  })
  return data
}

/**
 * Create a usage cleanup task (admin only)
 * @param payload - Cleanup task parameters
 * @returns Created cleanup task
 */
export async function createCleanupTask(payload: CreateUsageCleanupTaskRequest): Promise<UsageCleanupTask> {
  const { data } = await apiClient.post<UsageCleanupTask>('/admin/usage/cleanup-tasks', payload)
  return data
}

/**
 * Cancel a usage cleanup task (admin only)
 * @param taskId - Task ID to cancel
 */
export async function cancelCleanupTask(taskId: number): Promise<{ id: number; status: string }> {
  const { data } = await apiClient.post<{ id: number; status: string }>(
    `/admin/usage/cleanup-tasks/${taskId}/cancel`
  )
  return data
}

export const adminUsageAPI = {
  list,
  getStats,
  getBaselineVsLiteLLMBenchmark,
  searchUsers,
  searchApiKeys,
  listCleanupTasks,
  createCleanupTask,
  cancelCleanupTask
}

export default adminUsageAPI
