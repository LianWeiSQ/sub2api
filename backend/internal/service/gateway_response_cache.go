package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const (
	GatewayResponseCacheStatusHit      = "hit"
	GatewayResponseCacheStatusMiss     = "miss"
	GatewayResponseCacheStatusBypass   = "bypass"
	GatewayResponseCacheStatusStore    = "store"
	GatewayResponseCacheStatusDisabled = "disabled"

	GatewayResponseCacheHeaderStatus            = "X-Sub2API-Gateway-Cache"
	GatewayResponseCacheHeaderKey               = "X-Sub2API-Gateway-Cache-Key"
	GatewayResponseCacheHeaderBypassReason      = "X-Sub2API-Gateway-Cache-Bypass-Reason"
	GatewayResponseCacheHeaderSavedInputTokens  = "X-Sub2API-Gateway-Saved-Input-Tokens"
	GatewayResponseCacheHeaderSavedOutputTokens = "X-Sub2API-Gateway-Saved-Output-Tokens"
	GatewayResponseCacheHeaderSavedTokens       = "X-Sub2API-Gateway-Saved-Tokens"
	GatewayResponseCacheHeaderSavedCost         = "X-Sub2API-Gateway-Saved-Cost"
)

type GatewayResponseCacheRequest struct {
	Endpoint       string
	Platform       string
	AccountType    string
	UpstreamBase   string
	Model          string
	UpstreamModel  string
	Body           []byte
	Stream         bool
	APIKeyID       int64
	RequestHeaders http.Header
}

type GatewayResponseCacheEntry struct {
	StatusCode  int
	ContentType string
	Headers     http.Header
	Body        []byte
	RequestID   string
	Usage       OpenAIUsage
	CreatedAt   time.Time
	ExpiresAt   time.Time
	LastUsedAt  time.Time
}

type GatewayResponseCacheLookup struct {
	Status       string
	Key          string
	BypassReason string
	Entry        *GatewayResponseCacheEntry
	request      GatewayResponseCacheRequest
}

type GatewayResponseCacheRecentEvent struct {
	At          string `json:"at"`
	Status      string `json:"status"`
	Reason      string `json:"reason,omitempty"`
	Key         string `json:"key,omitempty"`
	Endpoint    string `json:"endpoint,omitempty"`
	Model       string `json:"model,omitempty"`
	SavedTokens int    `json:"saved_tokens,omitempty"`
}

type GatewayResponseCacheStats struct {
	Enabled               bool                              `json:"enabled"`
	Mode                  string                            `json:"mode"`
	Namespace             string                            `json:"namespace"`
	TTLSeconds            int                               `json:"ttl_seconds"`
	MaxEntries            int                               `json:"max_entries"`
	MaxEntryBytes         int64                             `json:"max_entry_bytes"`
	MaxRequestBytes       int64                             `json:"max_request_bytes"`
	Entries               int                               `json:"entries"`
	Hits                  int64                             `json:"hits"`
	Misses                int64                             `json:"misses"`
	Bypasses              int64                             `json:"bypasses"`
	Stores                int64                             `json:"stores"`
	Evictions             int64                             `json:"evictions"`
	Expired               int64                             `json:"expired"`
	HitRate               float64                           `json:"hit_rate"`
	UpstreamCallReduction float64                           `json:"upstream_call_reduction"`
	SavedInputTokens      int64                             `json:"saved_input_tokens"`
	SavedOutputTokens     int64                             `json:"saved_output_tokens"`
	SavedTokens           int64                             `json:"saved_tokens"`
	SavedCost             float64                           `json:"saved_cost"`
	BypassReasons         map[string]int64                  `json:"bypass_reasons"`
	RecentEvents          []GatewayResponseCacheRecentEvent `json:"recent_events"`
	UpdatedAt             string                            `json:"updated_at"`
}

type GatewayResponseCacheService struct {
	mu      sync.Mutex
	cfg     config.GatewayResponseCacheConfig
	entries map[string]*GatewayResponseCacheEntry

	hits              int64
	misses            int64
	bypasses          int64
	stores            int64
	evictions         int64
	expired           int64
	savedInputTokens  int64
	savedOutputTokens int64
	savedTokens       int64
	savedCost         float64
	bypassReasons     map[string]int64
	recentEvents      []GatewayResponseCacheRecentEvent
}

func NewGatewayResponseCacheService(cfg config.GatewayResponseCacheConfig, _ any) *GatewayResponseCacheService {
	cfg = normalizeGatewayResponseCacheConfig(cfg)
	return &GatewayResponseCacheService{
		cfg:           cfg,
		entries:       make(map[string]*GatewayResponseCacheEntry),
		bypassReasons: make(map[string]int64),
	}
}

func NewGatewayResponseCacheServiceFromConfig(cfg *config.Config) *GatewayResponseCacheService {
	cacheCfg := config.GatewayResponseCacheConfig{}
	if cfg != nil {
		cacheCfg = cfg.Gateway.ResponseCache
	}
	return NewGatewayResponseCacheService(cacheCfg, nil)
}

func normalizeGatewayResponseCacheConfig(cfg config.GatewayResponseCacheConfig) config.GatewayResponseCacheConfig {
	cfg.Mode = strings.ToLower(strings.TrimSpace(cfg.Mode))
	if cfg.Mode == "" {
		cfg.Mode = "default_off"
	}
	cfg.Namespace = strings.TrimSpace(cfg.Namespace)
	if cfg.Namespace == "" {
		cfg.Namespace = "sub2api:gateway_response_cache"
	}
	if cfg.DefaultTTLSeconds <= 0 {
		cfg.DefaultTTLSeconds = 600
	}
	if cfg.MaxEntries <= 0 {
		cfg.MaxEntries = 1024
	}
	if cfg.MaxEntryBytes <= 0 {
		cfg.MaxEntryBytes = 4 * 1024 * 1024
	}
	if cfg.MaxRequestBytes <= 0 {
		cfg.MaxRequestBytes = 1024 * 1024
	}
	return cfg
}

func (s *GatewayResponseCacheService) CanonicalKey(req GatewayResponseCacheRequest) (string, error) {
	if s == nil {
		return "", errors.New("gateway response cache service is nil")
	}
	bodyHash, err := canonicalGatewayResponseCacheBodyHash(req.Body)
	if err != nil {
		return "", err
	}
	parts := []string{
		s.cfg.Namespace,
		strings.TrimSpace(req.Endpoint),
		strings.TrimSpace(req.Platform),
		strings.TrimSpace(req.AccountType),
		strings.TrimRight(strings.TrimSpace(req.UpstreamBase), "/"),
		strings.TrimSpace(req.Model),
		strings.TrimSpace(req.UpstreamModel),
		strconv.FormatInt(req.APIKeyID, 10),
		bodyHash,
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:]), nil
}

func (s *GatewayResponseCacheService) Lookup(_ context.Context, req GatewayResponseCacheRequest) (*GatewayResponseCacheLookup, error) {
	if s == nil {
		return &GatewayResponseCacheLookup{Status: GatewayResponseCacheStatusDisabled, BypassReason: "service_unavailable"}, nil
	}
	if !s.cfg.Enabled {
		return &GatewayResponseCacheLookup{Status: GatewayResponseCacheStatusDisabled, BypassReason: "disabled"}, nil
	}
	if reason := s.bypassReason(req); reason != "" {
		return s.recordBypass(req, reason), nil
	}
	key, err := s.CanonicalKey(req)
	if err != nil {
		return s.recordBypass(req, "invalid_json"), nil
	}

	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	if entry, ok := s.entries[key]; ok {
		if !entry.ExpiresAt.IsZero() && now.After(entry.ExpiresAt) {
			delete(s.entries, key)
			s.expired++
			s.misses++
			s.pushEventLocked(req, GatewayResponseCacheStatusMiss, "expired", key, 0)
			return &GatewayResponseCacheLookup{Status: GatewayResponseCacheStatusMiss, Key: key, request: req}, nil
		}
		entry.LastUsedAt = now
		s.hits++
		savedInput := int64(entry.Usage.InputTokens)
		savedOutput := int64(entry.Usage.OutputTokens)
		savedTotal := savedInput + savedOutput
		s.savedInputTokens += savedInput
		s.savedOutputTokens += savedOutput
		s.savedTokens += savedTotal
		s.pushEventLocked(req, GatewayResponseCacheStatusHit, "", key, int(savedTotal))
		return &GatewayResponseCacheLookup{Status: GatewayResponseCacheStatusHit, Key: key, Entry: cloneGatewayResponseCacheEntry(entry), request: req}, nil
	}

	s.misses++
	s.pushEventLocked(req, GatewayResponseCacheStatusMiss, "", key, 0)
	return &GatewayResponseCacheLookup{Status: GatewayResponseCacheStatusMiss, Key: key, request: req}, nil
}

func (s *GatewayResponseCacheService) Store(_ context.Context, lookup *GatewayResponseCacheLookup, entry *GatewayResponseCacheEntry) error {
	if s == nil || lookup == nil || entry == nil {
		return nil
	}
	if !s.cfg.Enabled || lookup.Status != GatewayResponseCacheStatusMiss || lookup.Key == "" {
		return nil
	}
	if entry.StatusCode < http.StatusOK || entry.StatusCode >= http.StatusMultipleChoices {
		lookup.Status = GatewayResponseCacheStatusBypass
		lookup.BypassReason = "status_not_cacheable"
		return nil
	}
	if !strings.Contains(strings.ToLower(entry.ContentType), "json") {
		lookup.Status = GatewayResponseCacheStatusBypass
		lookup.BypassReason = "content_type_not_cacheable"
		return nil
	}
	if int64(len(entry.Body)) > s.cfg.MaxEntryBytes {
		lookup.Status = GatewayResponseCacheStatusBypass
		lookup.BypassReason = "response_too_large"
		return nil
	}

	now := time.Now()
	clone := cloneGatewayResponseCacheEntry(entry)
	clone.Body = append([]byte(nil), entry.Body...)
	clone.Headers = cloneGatewayResponseCacheHeader(entry.Headers)
	clone.CreatedAt = now
	clone.LastUsedAt = now
	clone.ExpiresAt = now.Add(time.Duration(s.cfg.DefaultTTLSeconds) * time.Second)
	if clone.StatusCode == 0 {
		clone.StatusCode = http.StatusOK
	}
	if strings.TrimSpace(clone.ContentType) == "" {
		clone.ContentType = "application/json"
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.deleteExpiredLocked(now)
	for len(s.entries) >= s.cfg.MaxEntries {
		if !s.evictOneLocked() {
			break
		}
	}
	s.entries[lookup.Key] = clone
	s.stores++
	lookup.Status = GatewayResponseCacheStatusStore
	s.pushEventLocked(lookup.request, GatewayResponseCacheStatusStore, "", lookup.Key, 0)
	return nil
}

func (s *GatewayResponseCacheService) Stats() GatewayResponseCacheStats {
	if s == nil {
		return GatewayResponseCacheStats{UpdatedAt: time.Now().UTC().Format(time.RFC3339)}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	totalLookups := s.hits + s.misses
	hitRate := 0.0
	if totalLookups > 0 {
		hitRate = float64(s.hits) / float64(totalLookups)
	}
	reduction := 0.0
	if totalLookups > 0 {
		reduction = float64(s.hits) / float64(totalLookups)
	}
	reasons := make(map[string]int64, len(s.bypassReasons))
	for k, v := range s.bypassReasons {
		reasons[k] = v
	}
	return GatewayResponseCacheStats{
		Enabled:               s.cfg.Enabled,
		Mode:                  s.cfg.Mode,
		Namespace:             s.cfg.Namespace,
		TTLSeconds:            s.cfg.DefaultTTLSeconds,
		MaxEntries:            s.cfg.MaxEntries,
		MaxEntryBytes:         s.cfg.MaxEntryBytes,
		MaxRequestBytes:       s.cfg.MaxRequestBytes,
		Entries:               len(s.entries),
		Hits:                  s.hits,
		Misses:                s.misses,
		Bypasses:              s.bypasses,
		Stores:                s.stores,
		Evictions:             s.evictions,
		Expired:               s.expired,
		HitRate:               hitRate,
		UpstreamCallReduction: reduction,
		SavedInputTokens:      s.savedInputTokens,
		SavedOutputTokens:     s.savedOutputTokens,
		SavedTokens:           s.savedTokens,
		SavedCost:             roundGatewayResponseCacheCost(s.savedCost),
		BypassReasons:         reasons,
		RecentEvents:          append([]GatewayResponseCacheRecentEvent(nil), s.recentEvents...),
		UpdatedAt:             time.Now().UTC().Format(time.RFC3339),
	}
}

func (s *GatewayResponseCacheService) RecordSavedCost(cost float64) {
	if s == nil || cost <= 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.savedCost += cost
}

func (s *GatewayResponseCacheService) bypassReason(req GatewayResponseCacheRequest) string {
	if req.Stream {
		return "stream"
	}
	if hasGatewayResponseCacheNoStore(req) {
		return "request_no_store"
	}
	if s.cfg.Mode == "default_off" && !(s.cfg.AllowRequestOptIn && hasGatewayResponseCacheOptIn(req)) {
		return "opt_in_required"
	}
	if len(req.Body) == 0 {
		return "empty_body"
	}
	if int64(len(req.Body)) > s.cfg.MaxRequestBytes {
		return "request_too_large"
	}
	if strings.TrimSpace(gjson.GetBytes(req.Body, "previous_response_id").String()) != "" {
		return "previous_response_id"
	}
	if hasGatewayResponseCacheFunctionCallOutput(req.Body) {
		return "function_call_output"
	}
	if hasGatewayResponseCacheUnsafeBuiltInTool(req.Body) {
		return "unsafe_builtin_tool"
	}
	if !s.cfg.CacheNonDeterministic {
		if temp := gjson.GetBytes(req.Body, "temperature"); temp.Exists() && math.Abs(temp.Float()) > 0.000001 {
			return "temperature_non_zero"
		}
		if topP := gjson.GetBytes(req.Body, "top_p"); topP.Exists() && math.Abs(topP.Float()-1) > 0.000001 {
			return "top_p_not_one"
		}
		if n := gjson.GetBytes(req.Body, "n"); n.Exists() && n.Int() > 1 {
			return "n_gt_one"
		}
	}
	return ""
}

func (s *GatewayResponseCacheService) recordBypass(req GatewayResponseCacheRequest, reason string) *GatewayResponseCacheLookup {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bypasses++
	s.bypassReasons[reason]++
	s.pushEventLocked(req, GatewayResponseCacheStatusBypass, reason, "", 0)
	return &GatewayResponseCacheLookup{Status: GatewayResponseCacheStatusBypass, BypassReason: reason, request: req}
}

func canonicalGatewayResponseCacheBodyHash(body []byte) (string, error) {
	if len(body) == 0 {
		return "", errors.New("empty body")
	}
	withoutCache, err := sjson.DeleteBytes(body, "cache")
	if err == nil {
		body = withoutCache
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var payload any
	if err := decoder.Decode(&payload); err != nil {
		return "", err
	}
	canonical, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:]), nil
}

func hasGatewayResponseCacheOptIn(req GatewayResponseCacheRequest) bool {
	headerValue := strings.ToLower(strings.TrimSpace(req.RequestHeaders.Get("X-Sub2API-Cache")))
	switch headerValue {
	case "1", "true", "on", "force", "use-cache":
		return true
	}
	for _, path := range []string{"cache.use-cache", "cache.use_cache", "cache.enabled"} {
		if gjson.GetBytes(req.Body, path).Bool() {
			return true
		}
	}
	return false
}

func hasGatewayResponseCacheNoStore(req GatewayResponseCacheRequest) bool {
	cacheControl := strings.ToLower(req.RequestHeaders.Get("Cache-Control"))
	pragma := strings.ToLower(req.RequestHeaders.Get("Pragma"))
	if strings.Contains(cacheControl, "no-store") || strings.Contains(cacheControl, "no-cache") || strings.Contains(pragma, "no-cache") {
		return true
	}
	headerValue := strings.ToLower(strings.TrimSpace(req.RequestHeaders.Get("X-Sub2API-Cache")))
	switch headerValue {
	case "bypass", "no-cache", "no-store", "refresh":
		return true
	}
	for _, path := range []string{"cache.no-store", "cache.no_store"} {
		if gjson.GetBytes(req.Body, path).Bool() {
			return true
		}
	}
	return false
}

func hasGatewayResponseCacheFunctionCallOutput(body []byte) bool {
	input := gjson.GetBytes(body, "input")
	if !input.IsArray() {
		return false
	}
	for _, item := range input.Array() {
		if strings.EqualFold(strings.TrimSpace(item.Get("type").String()), "function_call_output") {
			return true
		}
	}
	return false
}

func hasGatewayResponseCacheUnsafeBuiltInTool(body []byte) bool {
	tools := gjson.GetBytes(body, "tools")
	if !tools.IsArray() {
		return false
	}
	for _, tool := range tools.Array() {
		switch strings.ToLower(strings.TrimSpace(tool.Get("type").String())) {
		case "web_search", "web_search_preview", "file_search", "image_generation", "computer_use", "computer_use_preview", "mcp":
			return true
		}
	}
	return false
}

func (s *GatewayResponseCacheService) deleteExpiredLocked(now time.Time) {
	for key, entry := range s.entries {
		if entry != nil && !entry.ExpiresAt.IsZero() && now.After(entry.ExpiresAt) {
			delete(s.entries, key)
			s.expired++
		}
	}
}

func (s *GatewayResponseCacheService) evictOneLocked() bool {
	var oldestKey string
	var oldest time.Time
	for key, entry := range s.entries {
		if entry == nil {
			oldestKey = key
			break
		}
		t := entry.LastUsedAt
		if t.IsZero() {
			t = entry.CreatedAt
		}
		if oldestKey == "" || t.Before(oldest) {
			oldestKey = key
			oldest = t
		}
	}
	if oldestKey == "" {
		return false
	}
	delete(s.entries, oldestKey)
	s.evictions++
	return true
}

func (s *GatewayResponseCacheService) pushEventLocked(req GatewayResponseCacheRequest, status, reason, key string, savedTokens int) {
	if len(key) > 16 {
		key = key[:16]
	}
	s.recentEvents = append(s.recentEvents, GatewayResponseCacheRecentEvent{
		At:          time.Now().UTC().Format(time.RFC3339),
		Status:      status,
		Reason:      reason,
		Key:         key,
		Endpoint:    req.Endpoint,
		Model:       req.Model,
		SavedTokens: savedTokens,
	})
	if len(s.recentEvents) > 50 {
		s.recentEvents = append([]GatewayResponseCacheRecentEvent(nil), s.recentEvents[len(s.recentEvents)-50:]...)
	}
}

func cloneGatewayResponseCacheEntry(entry *GatewayResponseCacheEntry) *GatewayResponseCacheEntry {
	if entry == nil {
		return nil
	}
	clone := *entry
	clone.Body = append([]byte(nil), entry.Body...)
	clone.Headers = cloneGatewayResponseCacheHeader(entry.Headers)
	return &clone
}

func cloneGatewayResponseCacheHeader(in http.Header) http.Header {
	if len(in) == 0 {
		return nil
	}
	out := make(http.Header, len(in))
	for k, values := range in {
		out[k] = append([]string(nil), values...)
	}
	return out
}

func writeGatewayResponseCacheHeaders(c interface{ Header(string, string) }, lookup *GatewayResponseCacheLookup, savedInput, savedOutput int, savedCost float64) {
	if c == nil || lookup == nil {
		return
	}
	c.Header(GatewayResponseCacheHeaderStatus, lookup.Status)
	if lookup.Key != "" {
		c.Header(GatewayResponseCacheHeaderKey, shortGatewayResponseCacheKey(lookup.Key))
	}
	if lookup.BypassReason != "" {
		c.Header(GatewayResponseCacheHeaderBypassReason, lookup.BypassReason)
	}
	if savedInput > 0 {
		c.Header(GatewayResponseCacheHeaderSavedInputTokens, strconv.Itoa(savedInput))
	}
	if savedOutput > 0 {
		c.Header(GatewayResponseCacheHeaderSavedOutputTokens, strconv.Itoa(savedOutput))
	}
	if total := savedInput + savedOutput; total > 0 {
		c.Header(GatewayResponseCacheHeaderSavedTokens, strconv.Itoa(total))
	}
	if savedCost > 0 {
		c.Header(GatewayResponseCacheHeaderSavedCost, strconv.FormatFloat(savedCost, 'f', 9, 64))
	}
}

func shortGatewayResponseCacheKey(key string) string {
	if len(key) <= 16 {
		return key
	}
	return key[:16]
}

func roundGatewayResponseCacheCost(v float64) float64 {
	return math.Round(v*1_000_000_000) / 1_000_000_000
}
