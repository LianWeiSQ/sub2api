package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// UsageHandler handles admin usage-related requests
type UsageHandler struct {
	usageService         *service.UsageService
	apiKeyService        *service.APIKeyService
	adminService         service.AdminService
	cleanupService       *service.UsageCleanupService
	openAIGatewayService *service.OpenAIGatewayService
}

// NewUsageHandler creates a new admin usage handler
func NewUsageHandler(
	usageService *service.UsageService,
	apiKeyService *service.APIKeyService,
	adminService service.AdminService,
	cleanupService *service.UsageCleanupService,
	openAIGatewayServices ...*service.OpenAIGatewayService,
) *UsageHandler {
	var openAIGatewayService *service.OpenAIGatewayService
	if len(openAIGatewayServices) > 0 {
		openAIGatewayService = openAIGatewayServices[0]
	}
	return &UsageHandler{
		usageService:         usageService,
		apiKeyService:        apiKeyService,
		adminService:         adminService,
		cleanupService:       cleanupService,
		openAIGatewayService: openAIGatewayService,
	}
}

// CreateUsageCleanupTaskRequest represents cleanup task creation request
type CreateUsageCleanupTaskRequest struct {
	StartDate   string  `json:"start_date"`
	EndDate     string  `json:"end_date"`
	UserID      *int64  `json:"user_id"`
	APIKeyID    *int64  `json:"api_key_id"`
	AccountID   *int64  `json:"account_id"`
	GroupID     *int64  `json:"group_id"`
	Model       *string `json:"model"`
	RequestType *string `json:"request_type"`
	Stream      *bool   `json:"stream"`
	BillingType *int8   `json:"billing_type"`
	Timezone    string  `json:"timezone"`
}

// List handles listing all usage records with filters
// GET /api/v1/admin/usage
func (h *UsageHandler) List(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	exactTotal := false
	if exactTotalRaw := strings.TrimSpace(c.Query("exact_total")); exactTotalRaw != "" {
		parsed, err := strconv.ParseBool(exactTotalRaw)
		if err != nil {
			response.BadRequest(c, "Invalid exact_total value, use true or false")
			return
		}
		exactTotal = parsed
	}

	// Parse filters
	var userID, apiKeyID, accountID, groupID int64
	if userIDStr := c.Query("user_id"); userIDStr != "" {
		id, err := strconv.ParseInt(userIDStr, 10, 64)
		if err != nil {
			response.BadRequest(c, "Invalid user_id")
			return
		}
		userID = id
	}

	if apiKeyIDStr := c.Query("api_key_id"); apiKeyIDStr != "" {
		id, err := strconv.ParseInt(apiKeyIDStr, 10, 64)
		if err != nil {
			response.BadRequest(c, "Invalid api_key_id")
			return
		}
		apiKeyID = id
	}

	if accountIDStr := c.Query("account_id"); accountIDStr != "" {
		id, err := strconv.ParseInt(accountIDStr, 10, 64)
		if err != nil {
			response.BadRequest(c, "Invalid account_id")
			return
		}
		accountID = id
	}

	if groupIDStr := c.Query("group_id"); groupIDStr != "" {
		id, err := strconv.ParseInt(groupIDStr, 10, 64)
		if err != nil {
			response.BadRequest(c, "Invalid group_id")
			return
		}
		groupID = id
	}

	model := c.Query("model")
	billingMode := strings.TrimSpace(c.Query("billing_mode"))

	var requestType *int16
	var stream *bool
	if requestTypeStr := strings.TrimSpace(c.Query("request_type")); requestTypeStr != "" {
		parsed, err := service.ParseUsageRequestType(requestTypeStr)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		value := int16(parsed)
		requestType = &value
	} else if streamStr := c.Query("stream"); streamStr != "" {
		val, err := strconv.ParseBool(streamStr)
		if err != nil {
			response.BadRequest(c, "Invalid stream value, use true or false")
			return
		}
		stream = &val
	}

	var billingType *int8
	if billingTypeStr := c.Query("billing_type"); billingTypeStr != "" {
		val, err := strconv.ParseInt(billingTypeStr, 10, 8)
		if err != nil {
			response.BadRequest(c, "Invalid billing_type")
			return
		}
		bt := int8(val)
		billingType = &bt
	}

	// Parse date range
	var startTime, endTime *time.Time
	userTZ := c.Query("timezone") // Get user's timezone from request
	if startDateStr := c.Query("start_date"); startDateStr != "" {
		t, err := timezone.ParseInUserLocation("2006-01-02", startDateStr, userTZ)
		if err != nil {
			response.BadRequest(c, "Invalid start_date format, use YYYY-MM-DD")
			return
		}
		startTime = &t
	}

	if endDateStr := c.Query("end_date"); endDateStr != "" {
		t, err := timezone.ParseInUserLocation("2006-01-02", endDateStr, userTZ)
		if err != nil {
			response.BadRequest(c, "Invalid end_date format, use YYYY-MM-DD")
			return
		}
		// Use half-open range [start, end), move to next calendar day start (DST-safe).
		t = t.AddDate(0, 0, 1)
		endTime = &t
	}

	params := pagination.PaginationParams{
		Page:      page,
		PageSize:  pageSize,
		SortBy:    c.DefaultQuery("sort_by", "created_at"),
		SortOrder: c.DefaultQuery("sort_order", "desc"),
	}
	filters := usagestats.UsageLogFilters{
		UserID:      userID,
		APIKeyID:    apiKeyID,
		AccountID:   accountID,
		GroupID:     groupID,
		Model:       model,
		RequestType: requestType,
		Stream:      stream,
		BillingType: billingType,
		BillingMode: billingMode,
		StartTime:   startTime,
		EndTime:     endTime,
		ExactTotal:  exactTotal,
	}

	records, result, err := h.usageService.ListWithFilters(c.Request.Context(), params, filters)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	out := make([]dto.AdminUsageLog, 0, len(records))
	for i := range records {
		out = append(out, *dto.UsageLogFromServiceAdmin(&records[i]))
	}
	response.Paginated(c, out, result.Total, page, pageSize)
}

// Stats handles getting usage statistics with filters
// GET /api/v1/admin/usage/stats
func (h *UsageHandler) Stats(c *gin.Context) {
	// Parse filters - same as List endpoint
	var userID, apiKeyID, accountID, groupID int64
	if userIDStr := c.Query("user_id"); userIDStr != "" {
		id, err := strconv.ParseInt(userIDStr, 10, 64)
		if err != nil {
			response.BadRequest(c, "Invalid user_id")
			return
		}
		userID = id
	}

	if apiKeyIDStr := c.Query("api_key_id"); apiKeyIDStr != "" {
		id, err := strconv.ParseInt(apiKeyIDStr, 10, 64)
		if err != nil {
			response.BadRequest(c, "Invalid api_key_id")
			return
		}
		apiKeyID = id
	}

	if accountIDStr := c.Query("account_id"); accountIDStr != "" {
		id, err := strconv.ParseInt(accountIDStr, 10, 64)
		if err != nil {
			response.BadRequest(c, "Invalid account_id")
			return
		}
		accountID = id
	}

	if groupIDStr := c.Query("group_id"); groupIDStr != "" {
		id, err := strconv.ParseInt(groupIDStr, 10, 64)
		if err != nil {
			response.BadRequest(c, "Invalid group_id")
			return
		}
		groupID = id
	}

	model := c.Query("model")
	billingMode := strings.TrimSpace(c.Query("billing_mode"))

	var requestType *int16
	var stream *bool
	if requestTypeStr := strings.TrimSpace(c.Query("request_type")); requestTypeStr != "" {
		parsed, err := service.ParseUsageRequestType(requestTypeStr)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		value := int16(parsed)
		requestType = &value
	} else if streamStr := c.Query("stream"); streamStr != "" {
		val, err := strconv.ParseBool(streamStr)
		if err != nil {
			response.BadRequest(c, "Invalid stream value, use true or false")
			return
		}
		stream = &val
	}

	var billingType *int8
	if billingTypeStr := c.Query("billing_type"); billingTypeStr != "" {
		val, err := strconv.ParseInt(billingTypeStr, 10, 8)
		if err != nil {
			response.BadRequest(c, "Invalid billing_type")
			return
		}
		bt := int8(val)
		billingType = &bt
	}

	// Parse date range
	userTZ := c.Query("timezone")
	now := timezone.NowInUserLocation(userTZ)
	var startTime, endTime time.Time

	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")

	if startDateStr != "" && endDateStr != "" {
		var err error
		startTime, err = timezone.ParseInUserLocation("2006-01-02", startDateStr, userTZ)
		if err != nil {
			response.BadRequest(c, "Invalid start_date format, use YYYY-MM-DD")
			return
		}
		endTime, err = timezone.ParseInUserLocation("2006-01-02", endDateStr, userTZ)
		if err != nil {
			response.BadRequest(c, "Invalid end_date format, use YYYY-MM-DD")
			return
		}
		// 与 SQL 条件 created_at < end 对齐，使用次日 00:00 作为上边界（DST-safe）。
		endTime = endTime.AddDate(0, 0, 1)
	} else {
		period := c.DefaultQuery("period", "today")
		switch period {
		case "today":
			startTime = timezone.StartOfDayInUserLocation(now, userTZ)
		case "week":
			startTime = now.AddDate(0, 0, -7)
		case "month":
			startTime = now.AddDate(0, -1, 0)
		default:
			startTime = timezone.StartOfDayInUserLocation(now, userTZ)
		}
		endTime = now
	}

	// Build filters and call GetStatsWithFilters
	filters := usagestats.UsageLogFilters{
		UserID:      userID,
		APIKeyID:    apiKeyID,
		AccountID:   accountID,
		GroupID:     groupID,
		Model:       model,
		RequestType: requestType,
		Stream:      stream,
		BillingType: billingType,
		BillingMode: billingMode,
		StartTime:   &startTime,
		EndTime:     &endTime,
	}

	var stats *usagestats.UsageStats
	// nocache: 绕过缓存直接回源,刷新者本人拿最新;不回写缓存(管理台"我刷新我自己拿最新"语义,非全局失效)。
	if parseBoolQueryWithDefault(c.Query("nocache"), false) {
		s, err := h.usageService.GetStatsWithFilters(c.Request.Context(), filters)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		stats = s
		c.Header("X-Usage-Stats-Cache", "bypass")
	} else {
		s, hit, err := h.getStatsCached(c.Request.Context(), filters)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		stats = s
		c.Header("X-Usage-Stats-Cache", cacheStatusValue(hit))
	}
	if h.openAIGatewayService != nil && stats != nil {
		cacheStats := h.openAIGatewayService.GetGatewayResponseCacheStats()
		stats.GatewayCacheHits = cacheStats.Hits
		stats.GatewayCacheMisses = cacheStats.Misses
		stats.GatewayCacheBypasses = cacheStats.Bypasses
		stats.GatewayCacheStores = cacheStats.Stores
		stats.GatewayCacheHitRate = cacheStats.HitRate
		stats.GatewaySavedInputTokens = cacheStats.SavedInputTokens
		stats.GatewaySavedOutputTokens = cacheStats.SavedOutputTokens
		stats.GatewaySavedTokens = cacheStats.SavedTokens
		stats.GatewaySavedCost = cacheStats.SavedCost
		stats.UpstreamCallReduction = cacheStats.UpstreamCallReduction
	}

	response.Success(c, stats)
}

type benchmarkRunMetrics struct {
	InputTokens   int     `json:"input_tokens"`
	OutputTokens  int     `json:"output_tokens"`
	TotalTokens   int     `json:"total_tokens"`
	DurationMs    float64 `json:"duration_ms,omitempty"`
	FirstTokenMs  float64 `json:"first_token_ms,omitempty"`
	UpstreamCalls int     `json:"upstream_calls,omitempty"`
}

type benchmarkSampleResult struct {
	ID                  string              `json:"id"`
	Name                string              `json:"name"`
	Category            string              `json:"category"`
	Scenario            string              `json:"scenario,omitempty"`
	Baseline            benchmarkRunMetrics `json:"baseline"`
	LiteLLM             benchmarkRunMetrics `json:"litellm"`
	GatewayCacheStatus  string              `json:"gateway_cache_status,omitempty"`
	GatewayCacheHitRate float64             `json:"gateway_cache_hit_rate,omitempty"`
	SavedInputTokens    int                 `json:"saved_input_tokens,omitempty"`
	SavedOutputTokens   int                 `json:"saved_output_tokens,omitempty"`
	SavedTokens         int                 `json:"saved_tokens,omitempty"`
	SavedCost           float64             `json:"saved_cost,omitempty"`
	LatencyDeltaMs      float64             `json:"latency_delta_ms,omitempty"`
	Notes               string              `json:"notes,omitempty"`
}

type benchmarkSummary struct {
	SampleSize            int     `json:"sample_size"`
	BaselineTokens        int     `json:"baseline_tokens"`
	LiteLLMTokens         int     `json:"litellm_tokens"`
	SavedTokens           int     `json:"saved_tokens"`
	SavedCost             float64 `json:"saved_cost,omitempty"`
	CacheHitRate          float64 `json:"cache_hit_rate,omitempty"`
	AverageLatencyDeltaMs float64 `json:"average_latency_delta_ms,omitempty"`
	UpstreamCallReduction float64 `json:"upstream_call_reduction,omitempty"`
}

type benchmarkResponse struct {
	GeneratedAt        string                  `json:"generated_at"`
	TargetCacheHitRate float64                 `json:"target_cache_hit_rate"`
	Summary            benchmarkSummary        `json:"summary"`
	BaselineVsLiteLLM  []benchmarkSampleResult `json:"baseline_vs_litellm"`
}

type cacheBenchmarkRow struct {
	Route                    string  `json:"route"`
	SampleID                 string  `json:"sample_id"`
	Category                 string  `json:"category"`
	Cacheable                bool    `json:"cacheable"`
	PassIndex                int     `json:"pass_index"`
	CacheStatus              string  `json:"cache_status"`
	InputTokens              int     `json:"input_tokens"`
	OutputTokens             int     `json:"output_tokens"`
	TotalTokens              int     `json:"total_tokens"`
	GatewaySavedInputTokens  int     `json:"gateway_saved_input_tokens"`
	GatewaySavedOutputTokens int     `json:"gateway_saved_output_tokens"`
	GatewaySavedTokens       int     `json:"gateway_saved_tokens"`
	DurationMs               float64 `json:"duration_ms"`
	FirstTokenMs             float64 `json:"first_token_ms"`
	UpstreamDelta            int     `json:"upstream_delta"`
	Error                    string  `json:"error"`
}

type cacheBenchmarkFixtureFile struct {
	Samples []struct {
		ID          string `json:"id"`
		Category    string `json:"category"`
		Description string `json:"description"`
	} `json:"samples"`
}

// BenchmarkBaselineVsLiteLLM returns the latest local cache benchmark artifact.
// GET /api/v1/admin/usage/benchmark/baseline-vs-litellm
func (h *UsageHandler) BenchmarkBaselineVsLiteLLM(c *gin.Context) {
	payload, err := loadBenchmarkBaselineVsLiteLLM()
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, payload)
}

func emptyBenchmarkBaselineVsLiteLLM() benchmarkResponse {
	return benchmarkResponse{
		GeneratedAt:        time.Now().UTC().Format(time.RFC3339),
		TargetCacheHitRate: 0.90,
		Summary:            benchmarkSummary{},
		BaselineVsLiteLLM:  []benchmarkSampleResult{},
	}
}

func loadBenchmarkBaselineVsLiteLLM() (benchmarkResponse, error) {
	payload := emptyBenchmarkBaselineVsLiteLLM()
	root, ok := findCacheBenchmarkRoot()
	if !ok {
		return payload, nil
	}

	artifactPath := filepath.Join(root, "tools", "cache_benchmark", "results", "cache_benchmark_results.json")
	raw, err := os.ReadFile(artifactPath)
	if err != nil {
		if os.IsNotExist(err) {
			return payload, nil
		}
		return payload, err
	}

	var rows []cacheBenchmarkRow
	if err := json.Unmarshal(raw, &rows); err != nil {
		return payload, err
	}
	if len(rows) == 0 {
		return payload, nil
	}

	metadata := loadCacheBenchmarkFixtureMetadata(root)
	results, summary := buildBenchmarkBaselineVsLiteLLM(rows, metadata)
	payload.BaselineVsLiteLLM = results
	payload.Summary = summary
	if info, err := os.Stat(artifactPath); err == nil {
		payload.GeneratedAt = info.ModTime().UTC().Format(time.RFC3339)
	}
	return payload, nil
}

func findCacheBenchmarkRoot() (string, bool) {
	wd, err := os.Getwd()
	if err != nil {
		return "", false
	}
	dir := wd
	for i := 0; i < 6; i++ {
		if _, err := os.Stat(filepath.Join(dir, "tools", "cache_benchmark", "fixtures.json")); err == nil {
			return dir, true
		}
		next := filepath.Dir(dir)
		if next == dir {
			break
		}
		dir = next
	}
	return "", false
}

func loadCacheBenchmarkFixtureMetadata(root string) map[string]cacheBenchmarkFixtureSampleMetadata {
	out := make(map[string]cacheBenchmarkFixtureSampleMetadata)
	raw, err := os.ReadFile(filepath.Join(root, "tools", "cache_benchmark", "fixtures.json"))
	if err != nil {
		return out
	}
	var fixtures cacheBenchmarkFixtureFile
	if err := json.Unmarshal(raw, &fixtures); err != nil {
		return out
	}
	for _, sample := range fixtures.Samples {
		out[sample.ID] = cacheBenchmarkFixtureSampleMetadata{
			Category:    sample.Category,
			Description: sample.Description,
		}
	}
	return out
}

type cacheBenchmarkFixtureSampleMetadata struct {
	Category    string
	Description string
}

func buildBenchmarkBaselineVsLiteLLM(rows []cacheBenchmarkRow, metadata map[string]cacheBenchmarkFixtureSampleMetadata) ([]benchmarkSampleResult, benchmarkSummary) {
	bySample := make(map[string]map[string][]cacheBenchmarkRow)
	for _, row := range rows {
		if row.Error != "" || row.SampleID == "" {
			continue
		}
		if bySample[row.SampleID] == nil {
			bySample[row.SampleID] = make(map[string][]cacheBenchmarkRow)
		}
		bySample[row.SampleID][row.Route] = append(bySample[row.SampleID][row.Route], row)
	}

	ids := make([]string, 0, len(bySample))
	for id := range bySample {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	results := make([]benchmarkSampleResult, 0, len(ids))
	summary := benchmarkSummary{}
	var latencyDeltaTotal float64
	var latencyDeltaCount int
	var hitRows, cacheableRows int
	var upstreamCallsBaseline, upstreamCallsLiteLLM int

	for _, id := range ids {
		routes := bySample[id]
		baselineRows := routes["baseline"]
		liteLLMRows := routes["litellm"]
		if len(liteLLMRows) == 0 {
			continue
		}

		baseline := aggregateBenchmarkRunMetrics(baselineRows)
		liteLLMObserved := aggregateBenchmarkRunMetrics(liteLLMRows)
		savedInput, savedOutput, savedTokens, hits, cacheable := savedBenchmarkTokens(liteLLMRows)
		hitRate := 0.0
		if cacheable > 0 {
			hitRate = float64(hits) / float64(cacheable)
		}
		liteLLM := benchmarkRunMetrics{
			InputTokens:   maxInt(0, baseline.InputTokens-savedInput),
			OutputTokens:  maxInt(0, baseline.OutputTokens-savedOutput),
			TotalTokens:   maxInt(0, baseline.TotalTokens-savedTokens),
			DurationMs:    liteLLMObserved.DurationMs,
			FirstTokenMs:  liteLLMObserved.FirstTokenMs,
			UpstreamCalls: liteLLMObserved.UpstreamCalls,
		}
		if baseline.TotalTokens == 0 {
			liteLLM.TotalTokens = liteLLMObserved.TotalTokens
		}
		if baseline.InputTokens == 0 {
			liteLLM.InputTokens = liteLLMObserved.InputTokens
		}
		if baseline.OutputTokens == 0 {
			liteLLM.OutputTokens = liteLLMObserved.OutputTokens
		}

		status := "miss"
		if hits > 0 {
			status = "hit"
		} else if cacheable == 0 {
			status = "bypass"
		}
		meta := metadata[id]
		category := meta.Category
		if category == "" && len(liteLLMRows) > 0 {
			category = liteLLMRows[0].Category
		}
		latencyDelta := liteLLM.DurationMs - baseline.DurationMs
		result := benchmarkSampleResult{
			ID:                  id,
			Name:                id,
			Category:            category,
			Scenario:            meta.Description,
			Baseline:            baseline,
			LiteLLM:             liteLLM,
			GatewayCacheStatus:  status,
			GatewayCacheHitRate: hitRate,
			SavedInputTokens:    savedInput,
			SavedOutputTokens:   savedOutput,
			SavedTokens:         savedTokens,
			LatencyDeltaMs:      latencyDelta,
		}
		results = append(results, result)

		summary.SampleSize++
		summary.BaselineTokens += baseline.TotalTokens
		summary.LiteLLMTokens += liteLLM.TotalTokens
		summary.SavedTokens += savedTokens
		upstreamCallsBaseline += baseline.UpstreamCalls
		upstreamCallsLiteLLM += liteLLM.UpstreamCalls
		hitRows += hits
		cacheableRows += cacheable
		latencyDeltaTotal += latencyDelta
		latencyDeltaCount++
	}

	if cacheableRows > 0 {
		summary.CacheHitRate = float64(hitRows) / float64(cacheableRows)
	}
	if latencyDeltaCount > 0 {
		summary.AverageLatencyDeltaMs = latencyDeltaTotal / float64(latencyDeltaCount)
	}
	if upstreamCallsBaseline > 0 {
		summary.UpstreamCallReduction = float64(upstreamCallsBaseline-upstreamCallsLiteLLM) / float64(upstreamCallsBaseline)
	}
	return results, summary
}

func aggregateBenchmarkRunMetrics(rows []cacheBenchmarkRow) benchmarkRunMetrics {
	if len(rows) == 0 {
		return benchmarkRunMetrics{}
	}
	var metrics benchmarkRunMetrics
	for _, row := range rows {
		metrics.InputTokens += row.InputTokens
		metrics.OutputTokens += row.OutputTokens
		metrics.TotalTokens += row.TotalTokens
		metrics.DurationMs += row.DurationMs
		metrics.FirstTokenMs += row.FirstTokenMs
		metrics.UpstreamCalls += row.UpstreamDelta
	}
	metrics.DurationMs = metrics.DurationMs / float64(len(rows))
	metrics.FirstTokenMs = metrics.FirstTokenMs / float64(len(rows))
	return metrics
}

func savedBenchmarkTokens(rows []cacheBenchmarkRow) (input int, output int, total int, hits int, cacheable int) {
	for _, row := range rows {
		if row.Cacheable {
			cacheable++
		}
		if row.CacheStatus != "hit" {
			continue
		}
		hits++
		input += row.GatewaySavedInputTokens
		output += row.GatewaySavedOutputTokens
		if row.GatewaySavedTokens > 0 {
			total += row.GatewaySavedTokens
		} else {
			total += row.GatewaySavedInputTokens + row.GatewaySavedOutputTokens
		}
	}
	return input, output, total, hits, cacheable
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// SearchUsers handles searching users by email keyword
// GET /api/v1/admin/usage/search-users
func (h *UsageHandler) SearchUsers(c *gin.Context) {
	keyword := c.Query("q")
	if keyword == "" {
		response.Success(c, []any{})
		return
	}

	// Limit to 30 results
	users, _, err := h.adminService.ListUsers(c.Request.Context(), 1, 30, service.UserListFilters{Search: keyword, IncludeDeleted: true}, "email", "asc")
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// Return simplified user list (only id, email and deleted flag)
	type SimpleUser struct {
		ID      int64  `json:"id"`
		Email   string `json:"email"`
		Deleted bool   `json:"deleted"`
	}

	result := make([]SimpleUser, len(users))
	for i, u := range users {
		result[i] = SimpleUser{
			ID:      u.ID,
			Email:   u.Email,
			Deleted: u.DeletedAt != nil,
		}
	}

	response.Success(c, result)
}

// SearchAPIKeys handles searching API keys by user
// GET /api/v1/admin/usage/search-api-keys
func (h *UsageHandler) SearchAPIKeys(c *gin.Context) {
	userIDStr := c.Query("user_id")
	keyword := c.Query("q")

	var userID int64
	if userIDStr != "" {
		id, err := strconv.ParseInt(userIDStr, 10, 64)
		if err != nil {
			response.BadRequest(c, "Invalid user_id")
			return
		}
		userID = id
	}

	keys, err := h.apiKeyService.SearchAPIKeys(c.Request.Context(), userID, keyword, 30)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// Return simplified API key list (only id and name)
	type SimpleAPIKey struct {
		ID     int64  `json:"id"`
		Name   string `json:"name"`
		UserID int64  `json:"user_id"`
	}

	result := make([]SimpleAPIKey, len(keys))
	for i, k := range keys {
		result[i] = SimpleAPIKey{
			ID:     k.ID,
			Name:   k.Name,
			UserID: k.UserID,
		}
	}

	response.Success(c, result)
}

// ListCleanupTasks handles listing usage cleanup tasks
// GET /api/v1/admin/usage/cleanup-tasks
func (h *UsageHandler) ListCleanupTasks(c *gin.Context) {
	if h.cleanupService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Usage cleanup service unavailable")
		return
	}
	operator := int64(0)
	if subject, ok := middleware.GetAuthSubjectFromContext(c); ok {
		operator = subject.UserID
	}
	page, pageSize := response.ParsePagination(c)
	logger.LegacyPrintf("handler.admin.usage", "[UsageCleanup] 请求清理任务列表: operator=%d page=%d page_size=%d", operator, page, pageSize)
	params := pagination.PaginationParams{Page: page, PageSize: pageSize}
	tasks, result, err := h.cleanupService.ListTasks(c.Request.Context(), params)
	if err != nil {
		logger.LegacyPrintf("handler.admin.usage", "[UsageCleanup] 查询清理任务列表失败: operator=%d page=%d page_size=%d err=%v", operator, page, pageSize, err)
		response.ErrorFrom(c, err)
		return
	}
	out := make([]dto.UsageCleanupTask, 0, len(tasks))
	for i := range tasks {
		out = append(out, *dto.UsageCleanupTaskFromService(&tasks[i]))
	}
	logger.LegacyPrintf("handler.admin.usage", "[UsageCleanup] 返回清理任务列表: operator=%d total=%d items=%d page=%d page_size=%d", operator, result.Total, len(out), page, pageSize)
	response.Paginated(c, out, result.Total, page, pageSize)
}

// CreateCleanupTask handles creating a usage cleanup task
// POST /api/v1/admin/usage/cleanup-tasks
func (h *UsageHandler) CreateCleanupTask(c *gin.Context) {
	if h.cleanupService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Usage cleanup service unavailable")
		return
	}
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "Unauthorized")
		return
	}

	var req CreateUsageCleanupTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	req.StartDate = strings.TrimSpace(req.StartDate)
	req.EndDate = strings.TrimSpace(req.EndDate)
	if req.StartDate == "" || req.EndDate == "" {
		response.BadRequest(c, "start_date and end_date are required")
		return
	}

	startTime, err := timezone.ParseInUserLocation("2006-01-02", req.StartDate, req.Timezone)
	if err != nil {
		response.BadRequest(c, "Invalid start_date format, use YYYY-MM-DD")
		return
	}
	endTime, err := timezone.ParseInUserLocation("2006-01-02", req.EndDate, req.Timezone)
	if err != nil {
		response.BadRequest(c, "Invalid end_date format, use YYYY-MM-DD")
		return
	}
	endTime = endTime.Add(24*time.Hour - time.Nanosecond)

	var requestType *int16
	stream := req.Stream
	if req.RequestType != nil {
		parsed, err := service.ParseUsageRequestType(*req.RequestType)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		value := int16(parsed)
		requestType = &value
		stream = nil
	}

	filters := service.UsageCleanupFilters{
		StartTime:   startTime,
		EndTime:     endTime,
		UserID:      req.UserID,
		APIKeyID:    req.APIKeyID,
		AccountID:   req.AccountID,
		GroupID:     req.GroupID,
		Model:       req.Model,
		RequestType: requestType,
		Stream:      stream,
		BillingType: req.BillingType,
	}

	var userID any
	if filters.UserID != nil {
		userID = *filters.UserID
	}
	var apiKeyID any
	if filters.APIKeyID != nil {
		apiKeyID = *filters.APIKeyID
	}
	var accountID any
	if filters.AccountID != nil {
		accountID = *filters.AccountID
	}
	var groupID any
	if filters.GroupID != nil {
		groupID = *filters.GroupID
	}
	var model any
	if filters.Model != nil {
		model = *filters.Model
	}
	var streamValue any
	if filters.Stream != nil {
		streamValue = *filters.Stream
	}
	var requestTypeName any
	if filters.RequestType != nil {
		requestTypeName = service.RequestTypeFromInt16(*filters.RequestType).String()
	}
	var billingType any
	if filters.BillingType != nil {
		billingType = *filters.BillingType
	}

	idempotencyPayload := struct {
		OperatorID int64                         `json:"operator_id"`
		Body       CreateUsageCleanupTaskRequest `json:"body"`
	}{
		OperatorID: subject.UserID,
		Body:       req,
	}
	executeAdminIdempotentJSON(c, "admin.usage.cleanup_tasks.create", idempotencyPayload, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		logger.LegacyPrintf("handler.admin.usage", "[UsageCleanup] 请求创建清理任务: operator=%d start=%s end=%s user_id=%v api_key_id=%v account_id=%v group_id=%v model=%v request_type=%v stream=%v billing_type=%v tz=%q",
			subject.UserID,
			filters.StartTime.Format(time.RFC3339),
			filters.EndTime.Format(time.RFC3339),
			userID,
			apiKeyID,
			accountID,
			groupID,
			model,
			requestTypeName,
			streamValue,
			billingType,
			req.Timezone,
		)

		task, err := h.cleanupService.CreateTask(ctx, filters, subject.UserID)
		if err != nil {
			logger.LegacyPrintf("handler.admin.usage", "[UsageCleanup] 创建清理任务失败: operator=%d err=%v", subject.UserID, err)
			return nil, err
		}
		logger.LegacyPrintf("handler.admin.usage", "[UsageCleanup] 清理任务已创建: task=%d operator=%d status=%s", task.ID, subject.UserID, task.Status)
		return dto.UsageCleanupTaskFromService(task), nil
	})
}

// CancelCleanupTask handles canceling a usage cleanup task
// POST /api/v1/admin/usage/cleanup-tasks/:id/cancel
func (h *UsageHandler) CancelCleanupTask(c *gin.Context) {
	if h.cleanupService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Usage cleanup service unavailable")
		return
	}
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "Unauthorized")
		return
	}
	idStr := strings.TrimSpace(c.Param("id"))
	taskID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || taskID <= 0 {
		response.BadRequest(c, "Invalid task id")
		return
	}
	logger.LegacyPrintf("handler.admin.usage", "[UsageCleanup] 请求取消清理任务: task=%d operator=%d", taskID, subject.UserID)
	if err := h.cleanupService.CancelTask(c.Request.Context(), taskID, subject.UserID); err != nil {
		logger.LegacyPrintf("handler.admin.usage", "[UsageCleanup] 取消清理任务失败: task=%d operator=%d err=%v", taskID, subject.UserID, err)
		response.ErrorFrom(c, err)
		return
	}
	logger.LegacyPrintf("handler.admin.usage", "[UsageCleanup] 清理任务已取消: task=%d operator=%d", taskID, subject.UserID)
	response.Success(c, gin.H{"id": taskID, "status": service.UsageCleanupStatusCanceled})
}
