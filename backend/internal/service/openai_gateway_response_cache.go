package service

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/util/responseheaders"
	"github.com/gin-gonic/gin"
)

const openAIGatewayResponseCacheLookupKey = "openai_gateway_response_cache_lookup"

func (s *OpenAIGatewayService) lookupOpenAIGatewayResponseCache(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	endpoint string,
	body []byte,
	clientStream bool,
	originalModel string,
	upstreamModel string,
) (*GatewayResponseCacheLookup, error) {
	if s == nil || s.responseCacheService == nil {
		return nil, nil
	}
	req := GatewayResponseCacheRequest{
		Endpoint:       endpoint,
		Platform:       accountString(account, func(a *Account) string { return a.Platform }),
		AccountType:    accountString(account, func(a *Account) string { return a.Type }),
		UpstreamBase:   accountString(account, func(a *Account) string { return a.GetOpenAIBaseURL() }),
		Model:          originalModel,
		UpstreamModel:  upstreamModel,
		Body:           body,
		Stream:         clientStream,
		APIKeyID:       getAPIKeyIDFromContext(c),
		RequestHeaders: requestHeadersFromGin(c),
	}
	lookup, err := s.responseCacheService.Lookup(ctx, req)
	if lookup != nil && lookup.Status == GatewayResponseCacheStatusBypass {
		writeGatewayResponseCacheHeaders(c, lookup, 0, 0, 0)
	}
	if lookup != nil && c != nil {
		c.Set(openAIGatewayResponseCacheLookupKey, lookup)
	}
	return lookup, err
}

func (s *OpenAIGatewayService) storeOpenAIGatewayResponseCache(
	ctx context.Context,
	c *gin.Context,
	lookup *GatewayResponseCacheLookup,
	statusCode int,
	contentType string,
	headers http.Header,
	body []byte,
	requestID string,
	usage *OpenAIUsage,
) {
	if s == nil || s.responseCacheService == nil || lookup == nil || lookup.Status != GatewayResponseCacheStatusMiss {
		return
	}
	entryUsage := OpenAIUsage{}
	if usage != nil {
		entryUsage = *usage
	}
	entry := &GatewayResponseCacheEntry{
		StatusCode:  statusCode,
		ContentType: contentType,
		Headers:     responseheaders.FilterHeaders(headers, s.responseHeaderFilter),
		Body:        body,
		RequestID:   requestID,
		Usage:       entryUsage,
	}
	if err := s.responseCacheService.Store(ctx, lookup, entry); err != nil {
		lookup.Status = GatewayResponseCacheStatusBypass
		lookup.BypassReason = "store_error"
	}
	writeGatewayResponseCacheHeaders(c, lookup, 0, 0, 0)
}

func (s *OpenAIGatewayService) writeOpenAIGatewayResponseCacheHit(
	ctx context.Context,
	c *gin.Context,
	lookup *GatewayResponseCacheLookup,
	originalModel string,
	upstreamModel string,
	billingModel string,
	serviceTier *string,
	reasoningEffort *string,
	startTime time.Time,
) (*OpenAIForwardResult, bool) {
	if lookup == nil || lookup.Status != GatewayResponseCacheStatusHit || lookup.Entry == nil || c == nil {
		return nil, false
	}
	entry := lookup.Entry
	for key, values := range entry.Headers {
		for _, value := range values {
			c.Writer.Header().Add(key, value)
		}
	}
	savedInput := entry.Usage.InputTokens
	savedOutput := entry.Usage.OutputTokens
	savedCost := s.estimateOpenAIGatewayResponseCacheSavedCost(ctx, upstreamModel, entry.Usage, serviceTier)
	if s.responseCacheService != nil {
		s.responseCacheService.RecordSavedCost(savedCost)
	}
	writeGatewayResponseCacheHeaders(c, lookup, savedInput, savedOutput, savedCost)
	contentType := strings.TrimSpace(entry.ContentType)
	if contentType == "" {
		contentType = "application/json"
	}
	statusCode := entry.StatusCode
	if statusCode == 0 {
		statusCode = http.StatusOK
	}
	c.Data(statusCode, contentType, entry.Body)

	return &OpenAIForwardResult{
		RequestID:                entry.RequestID,
		Usage:                    OpenAIUsage{},
		Model:                    originalModel,
		BillingModel:             billingModel,
		UpstreamModel:            upstreamModel,
		ServiceTier:              serviceTier,
		ReasoningEffort:          reasoningEffort,
		Stream:                   false,
		OpenAIWSMode:             false,
		Duration:                 time.Since(startTime),
		GatewayCacheStatus:       GatewayResponseCacheStatusHit,
		GatewayCacheKey:          lookup.Key,
		GatewaySavedInputTokens:  savedInput,
		GatewaySavedOutputTokens: savedOutput,
		GatewaySavedCost:         savedCost,
	}, true
}

func openAIGatewayResponseCacheLookupFromContext(c *gin.Context) *GatewayResponseCacheLookup {
	if c == nil {
		return nil
	}
	value, ok := c.Get(openAIGatewayResponseCacheLookupKey)
	if !ok {
		return nil
	}
	lookup, _ := value.(*GatewayResponseCacheLookup)
	return lookup
}

func applyOpenAIGatewayResponseCacheResultMetadata(result *OpenAIForwardResult, lookup *GatewayResponseCacheLookup) {
	if result == nil || lookup == nil {
		return
	}
	result.GatewayCacheStatus = lookup.Status
	result.GatewayCacheKey = lookup.Key
	result.GatewayCacheBypassReason = lookup.BypassReason
	if lookup.Status == GatewayResponseCacheStatusHit && lookup.Entry != nil {
		result.GatewaySavedInputTokens = lookup.Entry.Usage.InputTokens
		result.GatewaySavedOutputTokens = lookup.Entry.Usage.OutputTokens
	}
}

func (s *OpenAIGatewayService) estimateOpenAIGatewayResponseCacheSavedCost(ctx context.Context, model string, usage OpenAIUsage, serviceTier *string) float64 {
	if s == nil || s.billingService == nil {
		return 0
	}
	inputTokens := usage.InputTokens - usage.CacheReadInputTokens
	if inputTokens < 0 {
		inputTokens = 0
	}
	tokens := UsageTokens{
		InputTokens:         inputTokens,
		OutputTokens:        usage.OutputTokens,
		CacheCreationTokens: usage.CacheCreationInputTokens,
		CacheReadTokens:     usage.CacheReadInputTokens,
		ImageOutputTokens:   usage.ImageOutputTokens,
	}
	tier := ""
	if serviceTier != nil {
		tier = strings.TrimSpace(*serviceTier)
	}
	cost, err := s.calculateOpenAIRecordUsageCost(ctx, &OpenAIForwardResult{Usage: usage}, &APIKey{}, []string{model}, 1.0, 1.0, tokens, tier)
	if err != nil || cost == nil {
		return 0
	}
	return cost.TotalCost
}

func accountString(account *Account, getter func(*Account) string) string {
	if account == nil || getter == nil {
		return ""
	}
	return strings.TrimSpace(getter(account))
}

func requestHeadersFromGin(c *gin.Context) http.Header {
	if c == nil || c.Request == nil {
		return nil
	}
	return c.Request.Header
}

func (s *OpenAIGatewayService) GetGatewayResponseCacheStats() GatewayResponseCacheStats {
	if s == nil || s.responseCacheService == nil {
		return GatewayResponseCacheStats{Enabled: false, UpdatedAt: time.Now().UTC().Format(time.RFC3339)}
	}
	return s.responseCacheService.Stats()
}
