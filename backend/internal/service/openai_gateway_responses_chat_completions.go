package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/util/responseheaders"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const (
	openAIResponsesModeForceChatCompletions = "force_chat_completions"
	openAIChatCompletionsEndpoint           = "/v1/chat/completions"
)

func shouldForwardOpenAIResponsesAsChatCompletions(account *Account) bool {
	if account == nil || account.Platform != PlatformOpenAI || account.Type != AccountTypeAPIKey {
		return false
	}
	mode := strings.ToLower(strings.TrimSpace(account.getExtraString("openai_responses_mode")))
	return mode == openAIResponsesModeForceChatCompletions
}

func buildOpenAIChatCompletionsURL(base string) string {
	normalized := strings.TrimRight(strings.TrimSpace(base), "/")
	if strings.HasSuffix(normalized, "/chat/completions") {
		return normalized
	}
	if strings.HasSuffix(normalized, "/v1") {
		return normalized + "/chat/completions"
	}
	return normalized + "/v1/chat/completions"
}

func (s *OpenAIGatewayService) buildChatCompletionsUpstreamRequest(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
	token string,
	stream bool,
) (*http.Request, error) {
	baseURL := account.GetOpenAIBaseURL()
	targetURL := "https://api.openai.com/v1/chat/completions"
	if baseURL != "" {
		validatedURL, err := s.validateUpstreamBaseURL(baseURL)
		if err != nil {
			return nil, err
		}
		targetURL = buildOpenAIChatCompletionsURL(validatedURL)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", targetURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("authorization", "Bearer "+token)
	req.Header.Set("content-type", "application/json")
	if stream {
		req.Header.Set("accept", "text/event-stream")
	} else {
		req.Header.Set("accept", "application/json")
	}
	for key, values := range c.Request.Header {
		lowerKey := strings.ToLower(key)
		if !openaiAllowedHeaders[lowerKey] {
			continue
		}
		if lowerKey == "accept" || lowerKey == "content-type" || lowerKey == "authorization" || lowerKey == "openai-beta" {
			continue
		}
		for _, v := range values {
			req.Header.Add(key, v)
		}
	}
	if customUA := account.GetOpenAIUserAgent(); customUA != "" {
		req.Header.Set("user-agent", customUA)
	}
	return req, nil
}

func (s *OpenAIGatewayService) forwardResponsesAsChatCompletions(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
	token string,
	clientStream bool,
	originalModel string,
	upstreamModel string,
	serviceTier *string,
	reasoningEffort *string,
	startTime time.Time,
) (*OpenAIForwardResult, error) {
	var responsesReq apicompat.ResponsesRequest
	if err := json.Unmarshal(body, &responsesReq); err != nil {
		return nil, fmt.Errorf("parse responses request for chat-completions upstream: %w", err)
	}
	responsesReq.Model = upstreamModel
	responsesReq.Stream = clientStream

	chatReq, err := apicompat.ResponsesToChatCompletionsRequest(&responsesReq)
	if err != nil {
		return nil, fmt.Errorf("convert responses to chat completions: %w", err)
	}
	chatReq.Model = upstreamModel
	chatReq.Stream = clientStream
	if clientStream && chatReq.StreamOptions == nil {
		chatReq.StreamOptions = &apicompat.ChatStreamOptions{IncludeUsage: true}
	}

	chatBody, err := json.Marshal(chatReq)
	if err != nil {
		return nil, fmt.Errorf("marshal chat completions request: %w", err)
	}
	setOpsUpstreamRequestBody(c, chatBody)

	upstreamReq, err := s.buildChatCompletionsUpstreamRequest(ctx, c, account, chatBody, token, clientStream)
	if err != nil {
		return nil, fmt.Errorf("build chat completions upstream request: %w", err)
	}

	proxyURL := ""
	if account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	resp, err := s.httpUpstream.Do(upstreamReq, proxyURL, account.ID, account.Concurrency)
	if err != nil {
		safeErr := sanitizeUpstreamErrorMessage(err.Error())
		setOpsUpstreamError(c, 0, safeErr, "")
		return nil, fmt.Errorf("chat completions upstream request failed: %s", safeErr)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		return s.handleErrorResponse(ctx, resp, c, account, chatBody)
	}

	var result *OpenAIForwardResult
	if clientStream {
		result, err = s.handleChatCompletionsUpstreamStreamAsResponses(resp, c, originalModel, upstreamModel, startTime)
	} else {
		result, err = s.handleChatCompletionsUpstreamBufferedAsResponses(resp, c, originalModel, upstreamModel, startTime)
	}
	if result != nil {
		result.ServiceTier = serviceTier
		result.ReasoningEffort = reasoningEffort
	}
	return result, err
}

func (s *OpenAIGatewayService) handleChatCompletionsUpstreamBufferedAsResponses(
	resp *http.Response,
	c *gin.Context,
	originalModel string,
	upstreamModel string,
	startTime time.Time,
) (*OpenAIForwardResult, error) {
	body, err := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
	if err != nil {
		return nil, err
	}
	var chatResp apicompat.ChatCompletionsResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return nil, fmt.Errorf("parse chat completions response: %w", err)
	}

	responsesResp := apicompat.ChatCompletionsResponseToResponses(&chatResp, originalModel)
	if responsesResp == nil {
		return nil, fmt.Errorf("convert chat completions response: empty response")
	}
	responseBytes, err := json.Marshal(responsesResp)
	if err != nil {
		return nil, err
	}
	usage := openAIUsageFromChatUsage(chatResp.Usage)
	s.storeOpenAIGatewayResponseCache(
		c.Request.Context(),
		c,
		openAIGatewayResponseCacheLookupFromContext(c),
		http.StatusOK,
		"application/json",
		resp.Header,
		responseBytes,
		resp.Header.Get("x-request-id"),
		&usage,
	)
	if s.responseHeaderFilter != nil {
		responseheaders.WriteFilteredHeaders(c.Writer.Header(), resp.Header, s.responseHeaderFilter)
	}
	c.Data(http.StatusOK, "application/json; charset=utf-8", responseBytes)

	result := &OpenAIForwardResult{
		RequestID:        resp.Header.Get("x-request-id"),
		Usage:            usage,
		Model:            originalModel,
		BillingModel:     originalModel,
		UpstreamModel:    upstreamModel,
		UpstreamEndpoint: openAIChatCompletionsEndpoint,
		Stream:           false,
		Duration:         time.Since(startTime),
	}
	applyOpenAIGatewayResponseCacheResultMetadata(result, openAIGatewayResponseCacheLookupFromContext(c))
	return result, nil
}

func (s *OpenAIGatewayService) handleChatCompletionsUpstreamStreamAsResponses(
	resp *http.Response,
	c *gin.Context,
	originalModel string,
	upstreamModel string,
	startTime time.Time,
) (*OpenAIForwardResult, error) {
	if s.responseHeaderFilter != nil {
		responseheaders.WriteFilteredHeaders(c.Writer.Header(), resp.Header, s.responseHeaderFilter)
	}
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.WriteHeader(http.StatusOK)

	state := apicompat.NewChatCompletionsToResponsesStreamState(originalModel)
	requestID := resp.Header.Get("x-request-id")
	var usage OpenAIUsage
	var firstTokenMs *int
	firstEvent := true

	scanner := bufio.NewScanner(resp.Body)
	maxLineSize := defaultMaxLineSize
	if s.cfg != nil && s.cfg.Gateway.MaxLineSize > 0 {
		maxLineSize = s.cfg.Gateway.MaxLineSize
	}
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineSize)

	writeEvents := func(events []apicompat.ResponsesStreamEvent) bool {
		for _, event := range events {
			sse, err := apicompat.ResponsesEventToSSE(event)
			if err != nil {
				logger.L().Warn("openai responses-as-chat: failed to marshal responses event", zap.Error(err))
				continue
			}
			if _, err := fmt.Fprint(c.Writer, sse); err != nil {
				logger.L().Info("openai responses-as-chat: client disconnected", zap.String("request_id", requestID))
				return false
			}
		}
		if len(events) > 0 {
			c.Writer.Flush()
		}
		return true
	}

	for scanner.Scan() {
		data, ok := extractOpenAISSEDataLine(scanner.Text())
		if !ok || data == "" {
			continue
		}
		if data == "[DONE]" {
			break
		}
		if firstEvent {
			firstEvent = false
			ms := int(time.Since(startTime).Milliseconds())
			firstTokenMs = &ms
		}

		var chunk apicompat.ChatCompletionsChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			logger.L().Warn("openai responses-as-chat: failed to parse chat stream chunk", zap.Error(err), zap.String("request_id", requestID))
			continue
		}
		if chunk.Usage != nil {
			usage = openAIUsageFromChatUsage(chunk.Usage)
		}
		if !writeEvents(apicompat.ChatCompletionsChunkToResponsesEvents(&chunk, state)) {
			return &OpenAIForwardResult{
				RequestID:        requestID,
				Usage:            usage,
				Model:            originalModel,
				BillingModel:     originalModel,
				UpstreamModel:    upstreamModel,
				UpstreamEndpoint: openAIChatCompletionsEndpoint,
				Stream:           true,
				Duration:         time.Since(startTime),
				FirstTokenMs:     firstTokenMs,
			}, nil
		}
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		logger.L().Warn("openai responses-as-chat: stream read error", zap.Error(err), zap.String("request_id", requestID))
	}
	writeEvents(apicompat.FinalizeChatCompletionsResponsesStream(state))

	return &OpenAIForwardResult{
		RequestID:        requestID,
		Usage:            usage,
		Model:            originalModel,
		BillingModel:     originalModel,
		UpstreamModel:    upstreamModel,
		UpstreamEndpoint: openAIChatCompletionsEndpoint,
		Stream:           true,
		Duration:         time.Since(startTime),
		FirstTokenMs:     firstTokenMs,
	}, nil
}

func openAIUsageFromChatUsage(usage *apicompat.ChatUsage) OpenAIUsage {
	if usage == nil {
		return OpenAIUsage{}
	}
	out := OpenAIUsage{
		InputTokens:  usage.PromptTokens,
		OutputTokens: usage.CompletionTokens,
	}
	if usage.PromptTokensDetails != nil {
		out.CacheReadInputTokens = usage.PromptTokensDetails.CachedTokens
	}
	return out
}
