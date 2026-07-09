package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type gatewayResponseCacheUpstreamStub struct {
	calls int
	body  []byte
}

func (s *gatewayResponseCacheUpstreamStub) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	s.calls++
	if req != nil && req.Body != nil {
		s.body, _ = io.ReadAll(req.Body)
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
			"X-Request-Id": []string{"upstream-req-1"},
		},
		Body: io.NopCloser(bytes.NewBufferString(`{"id":"resp_test","object":"response","created_at":1,"model":"gpt-5","output":[],"usage":{"input_tokens":10,"output_tokens":5}}`)),
	}, nil
}

func (s *gatewayResponseCacheUpstreamStub) DoWithTLS(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return s.Do(req, proxyURL, accountID, accountConcurrency)
}

func TestGatewayResponseCacheKeyStableAndIncludesToolsAndResponseFormat(t *testing.T) {
	cacheSvc := NewGatewayResponseCacheService(config.GatewayResponseCacheConfig{
		Enabled: true,
		Mode:    "always_on",
	}, nil)

	req := GatewayResponseCacheRequest{
		Endpoint:      "/v1/chat/completions",
		Platform:      PlatformOpenAI,
		AccountType:   AccountTypeAPIKey,
		UpstreamBase:  "https://api.openai.com",
		Model:         "gpt-5",
		UpstreamModel: "gpt-5",
		APIKeyID:      42,
		Body: []byte(`{
			"model":"gpt-5",
			"temperature":0,
			"messages":[{"role":"user","content":"hi"}],
			"tools":[{"type":"function","function":{"name":"lookup","parameters":{"type":"object"}}}],
			"response_format":{"type":"json_object"}
		}`),
	}
	reordered := req
	reordered.Body = []byte(`{
		"response_format":{"type":"json_object"},
		"tools":[{"function":{"parameters":{"type":"object"},"name":"lookup"},"type":"function"}],
		"messages":[{"content":"hi","role":"user"}],
		"temperature":0,
		"model":"gpt-5",
		"cache":{"use-cache":true}
	}`)

	key1, err := cacheSvc.CanonicalKey(req)
	require.NoError(t, err)
	key2, err := cacheSvc.CanonicalKey(reordered)
	require.NoError(t, err)
	require.Equal(t, key1, key2)

	toolChanged := req
	toolChanged.Body = []byte(`{"model":"gpt-5","temperature":0,"messages":[{"role":"user","content":"hi"}],"tools":[{"type":"function","function":{"name":"lookup_v2","parameters":{"type":"object"}}}],"response_format":{"type":"json_object"}}`)
	key3, err := cacheSvc.CanonicalKey(toolChanged)
	require.NoError(t, err)
	require.NotEqual(t, key1, key3)

	formatChanged := req
	formatChanged.Body = []byte(`{"model":"gpt-5","temperature":0,"messages":[{"role":"user","content":"hi"}],"tools":[{"type":"function","function":{"name":"lookup","parameters":{"type":"object"}}}],"response_format":{"type":"text"}}`)
	key4, err := cacheSvc.CanonicalKey(formatChanged)
	require.NoError(t, err)
	require.NotEqual(t, key1, key4)
}

func TestGatewayResponseCacheBypassesNonZeroTemperature(t *testing.T) {
	cacheSvc := NewGatewayResponseCacheService(config.GatewayResponseCacheConfig{
		Enabled: true,
		Mode:    "always_on",
	}, nil)

	lookup, err := cacheSvc.Lookup(context.Background(), GatewayResponseCacheRequest{
		Endpoint:      "/v1/responses",
		Platform:      PlatformOpenAI,
		AccountType:   AccountTypeAPIKey,
		UpstreamBase:  "https://api.openai.com",
		Model:         "gpt-5",
		UpstreamModel: "gpt-5",
		APIKeyID:      42,
		Body:          []byte(`{"model":"gpt-5","input":"hi","temperature":0.7}`),
	})
	require.NoError(t, err)
	require.Equal(t, GatewayResponseCacheStatusBypass, lookup.Status)
	require.Equal(t, "temperature_non_zero", lookup.BypassReason)
}

func TestGatewayResponseCacheBypassesStream(t *testing.T) {
	cacheSvc := NewGatewayResponseCacheService(config.GatewayResponseCacheConfig{
		Enabled: true,
		Mode:    "always_on",
	}, nil)

	lookup, err := cacheSvc.Lookup(context.Background(), GatewayResponseCacheRequest{
		Endpoint:      "/v1/responses",
		Platform:      PlatformOpenAI,
		AccountType:   AccountTypeAPIKey,
		UpstreamBase:  "https://api.openai.com",
		Model:         "gpt-5",
		UpstreamModel: "gpt-5",
		APIKeyID:      42,
		Stream:        true,
		Body:          []byte(`{"model":"gpt-5","input":"hi","stream":true}`),
	})
	require.NoError(t, err)
	require.Equal(t, GatewayResponseCacheStatusBypass, lookup.Status)
	require.Equal(t, "stream", lookup.BypassReason)
}

func TestOpenAIGatewayResponseCacheHitDoesNotCallUpstream(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstream := &gatewayResponseCacheUpstreamStub{}
	cfg := &config.Config{
		Gateway: config.GatewayConfig{
			OpenAIWS: config.GatewayOpenAIWSConfig{ForceHTTP: true},
			ResponseCache: config.GatewayResponseCacheConfig{
				Enabled:           true,
				Mode:              "always_on",
				DefaultTTLSeconds: 60,
				MaxEntries:        16,
				MaxEntryBytes:     1024 * 1024,
				AllowRequestOptIn: true,
			},
		},
	}
	svc := &OpenAIGatewayService{
		cfg:                  cfg,
		httpUpstream:         upstream,
		responseCacheService: NewGatewayResponseCacheServiceFromConfig(cfg),
	}
	account := &Account{
		ID:          1,
		Name:        "openai-key",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "sk-test"},
	}
	body := []byte(`{"model":"gpt-5","input":"hi","temperature":0}`)

	w1 := httptest.NewRecorder()
	c1, _ := gin.CreateTestContext(w1)
	c1.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	c1.Set("api_key", &APIKey{ID: 7})
	result1, err := svc.Forward(c1.Request.Context(), c1, account, body)
	require.NoError(t, err)
	require.NotNil(t, result1)
	require.Equal(t, http.StatusOK, w1.Code)
	require.Equal(t, GatewayResponseCacheStatusStore, w1.Header().Get(GatewayResponseCacheHeaderStatus))
	require.Equal(t, GatewayResponseCacheStatusStore, result1.GatewayCacheStatus)
	require.Equal(t, 1, upstream.calls)

	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	c2.Set("api_key", &APIKey{ID: 7})
	result2, err := svc.Forward(c2.Request.Context(), c2, account, body)
	require.NoError(t, err)
	require.NotNil(t, result2)
	require.Equal(t, http.StatusOK, w2.Code)
	require.Equal(t, 1, upstream.calls)
	require.Equal(t, GatewayResponseCacheStatusHit, w2.Header().Get(GatewayResponseCacheHeaderStatus))
	require.NotEmpty(t, w2.Header().Get(GatewayResponseCacheHeaderKey))
	require.Equal(t, "10", w2.Header().Get(GatewayResponseCacheHeaderSavedInputTokens))
	require.Equal(t, "5", w2.Header().Get(GatewayResponseCacheHeaderSavedOutputTokens))
	require.JSONEq(t, w1.Body.String(), w2.Body.String())
	require.Equal(t, GatewayResponseCacheStatusHit, result2.GatewayCacheStatus)
	require.Equal(t, 10, result2.GatewaySavedInputTokens)
	require.Equal(t, 5, result2.GatewaySavedOutputTokens)
}
