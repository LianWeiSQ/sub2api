package apicompat

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ResponsesToChatCompletionsRequest converts an OpenAI Responses request into
// a Chat Completions request for OpenAI-compatible upstreams that do not expose
// /v1/responses.
func ResponsesToChatCompletionsRequest(req *ResponsesRequest) (*ChatCompletionsRequest, error) {
	if req == nil {
		return nil, fmt.Errorf("nil responses request")
	}

	messages, err := convertResponsesInputToChatMessages(req.Instructions, req.Input)
	if err != nil {
		return nil, err
	}

	out := &ChatCompletionsRequest{
		Model:       req.Model,
		Messages:    messages,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Stream:      req.Stream,
	}
	if req.Stream {
		out.StreamOptions = &ChatStreamOptions{IncludeUsage: true}
	}
	if req.MaxOutputTokens != nil && *req.MaxOutputTokens > 0 {
		maxTokens := *req.MaxOutputTokens
		out.MaxTokens = &maxTokens
	}
	if req.Reasoning != nil && strings.TrimSpace(req.Reasoning.Effort) != "" {
		out.ReasoningEffort = req.Reasoning.Effort
	}
	if len(req.Tools) > 0 {
		out.Tools = convertResponsesToolsToChatTools(req.Tools)
	}
	if len(req.ToolChoice) > 0 {
		toolChoice, err := convertResponsesToolChoiceToChat(req.ToolChoice)
		if err != nil {
			return nil, fmt.Errorf("convert tool_choice: %w", err)
		}
		out.ToolChoice = toolChoice
	}

	return out, nil
}

func convertResponsesInputToChatMessages(instructions string, inputRaw json.RawMessage) ([]ChatMessage, error) {
	var messages []ChatMessage
	if strings.TrimSpace(instructions) != "" {
		content, _ := json.Marshal(instructions)
		messages = append(messages, ChatMessage{Role: "system", Content: content})
	}

	var inputStr string
	if err := json.Unmarshal(inputRaw, &inputStr); err == nil {
		content, _ := json.Marshal(inputStr)
		messages = append(messages, ChatMessage{Role: "user", Content: content})
		return messages, nil
	}

	var items []ResponsesInputItem
	if err := json.Unmarshal(inputRaw, &items); err != nil {
		return nil, fmt.Errorf("parse responses input: %w", err)
	}

	for _, item := range items {
		switch {
		case item.Type == "function_call":
			callID := strings.TrimSpace(item.CallID)
			if callID == "" {
				callID = strings.TrimSpace(item.ID)
			}
			messages = append(messages, ChatMessage{
				Role: "assistant",
				ToolCalls: []ChatToolCall{{
					ID:   callID,
					Type: "function",
					Function: ChatFunctionCall{
						Name:      item.Name,
						Arguments: defaultJSONString(item.Arguments),
					},
				}},
			})
		case item.Type == "function_call_output":
			content, _ := json.Marshal(item.Output)
			messages = append(messages, ChatMessage{
				Role:       "tool",
				Content:    content,
				ToolCallID: strings.TrimSpace(item.CallID),
			})
		case item.Role != "":
			content, err := convertResponsesMessageContentToChat(item.Content, item.Role)
			if err != nil {
				return nil, err
			}
			messages = append(messages, ChatMessage{
				Role:    normalizeResponsesRoleForChat(item.Role),
				Content: content,
			})
		case len(item.Content) > 0:
			content, err := convertResponsesMessageContentToChat(item.Content, "user")
			if err != nil {
				return nil, err
			}
			messages = append(messages, ChatMessage{Role: "user", Content: content})
		}
	}

	if len(messages) == 0 {
		content, _ := json.Marshal("")
		messages = append(messages, ChatMessage{Role: "user", Content: content})
	}
	return messages, nil
}

func normalizeResponsesRoleForChat(role string) string {
	switch role {
	case "system", "developer":
		return "system"
	case "assistant":
		return "assistant"
	default:
		return "user"
	}
}

func convertResponsesMessageContentToChat(raw json.RawMessage, role string) (json.RawMessage, error) {
	if len(raw) == 0 {
		return json.Marshal("")
	}

	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return json.Marshal(text)
	}

	var parts []ResponsesContentPart
	if err := json.Unmarshal(raw, &parts); err != nil {
		return raw, nil
	}

	var textParts []string
	var chatParts []ChatContentPart
	for _, part := range parts {
		switch part.Type {
		case "input_text", "output_text", "text":
			if part.Text == "" {
				continue
			}
			textParts = append(textParts, part.Text)
			chatParts = append(chatParts, ChatContentPart{Type: "text", Text: part.Text})
		case "input_image":
			if role == "assistant" || part.ImageURL == "" {
				continue
			}
			chatParts = append(chatParts, ChatContentPart{
				Type:     "image_url",
				ImageURL: &ChatImageURL{URL: part.ImageURL},
			})
		}
	}

	if role == "assistant" || len(chatParts) == 0 || len(chatParts) == len(textParts) {
		return json.Marshal(strings.Join(textParts, "\n\n"))
	}
	return json.Marshal(chatParts)
}

func convertResponsesToolsToChatTools(tools []ResponsesTool) []ChatTool {
	out := make([]ChatTool, 0, len(tools))
	for _, tool := range tools {
		if tool.Type != "function" || strings.TrimSpace(tool.Name) == "" {
			continue
		}
		out = append(out, ChatTool{
			Type: "function",
			Function: &ChatFunction{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.Parameters,
				Strict:      tool.Strict,
			},
		})
	}
	return out
}

func convertResponsesToolChoiceToChat(raw json.RawMessage) (json.RawMessage, error) {
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return json.Marshal(text)
	}

	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return raw, nil
	}
	if _, ok := obj["function"]; ok {
		return raw, nil
	}
	if typ, _ := obj["type"].(string); typ == "function" {
		if name, _ := obj["name"].(string); strings.TrimSpace(name) != "" {
			return json.Marshal(map[string]any{
				"type": "function",
				"function": map[string]any{
					"name": name,
				},
			})
		}
	}
	return raw, nil
}

func defaultJSONString(value string) string {
	if strings.TrimSpace(value) == "" {
		return "{}"
	}
	return value
}

// ChatCompletionsResponseToResponses converts a non-streaming Chat Completions
// response into a Responses API response.
func ChatCompletionsResponseToResponses(resp *ChatCompletionsResponse, model string) *ResponsesResponse {
	if resp == nil {
		return nil
	}
	if model == "" {
		model = resp.Model
	}
	out := &ResponsesResponse{
		ID:     responseIDFromChatID(resp.ID),
		Object: "response",
		Model:  model,
		Status: "completed",
		Output: []ResponsesOutput{},
	}
	if resp.Usage != nil {
		out.Usage = chatUsageToResponsesUsage(resp.Usage)
	}
	if len(resp.Choices) == 0 {
		return out
	}

	msg := resp.Choices[0].Message
	if strings.TrimSpace(msg.ReasoningContent) != "" {
		out.Output = append(out.Output, ResponsesOutput{
			Type: "reasoning",
			ID:   generateItemID(),
			Summary: []ResponsesSummary{{
				Type: "summary_text",
				Text: msg.ReasoningContent,
			}},
		})
	}
	if content := chatMessageContentAsString(msg.Content); content != "" {
		out.Output = append(out.Output, ResponsesOutput{
			Type:   "message",
			ID:     generateItemID(),
			Role:   "assistant",
			Status: "completed",
			Content: []ResponsesContentPart{{
				Type: "output_text",
				Text: content,
			}},
		})
	}
	for _, call := range msg.ToolCalls {
		out.Output = append(out.Output, ResponsesOutput{
			Type:      "function_call",
			ID:        generateItemID(),
			CallID:    call.ID,
			Name:      call.Function.Name,
			Arguments: defaultJSONString(call.Function.Arguments),
			Status:    "completed",
		})
	}
	return out
}

func chatMessageContentAsString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text
	}
	var parts []ChatContentPart
	if err := json.Unmarshal(raw, &parts); err == nil {
		var texts []string
		for _, part := range parts {
			if part.Type == "text" && part.Text != "" {
				texts = append(texts, part.Text)
			}
		}
		return strings.Join(texts, "\n\n")
	}
	return ""
}

func chatUsageToResponsesUsage(usage *ChatUsage) *ResponsesUsage {
	if usage == nil {
		return nil
	}
	out := &ResponsesUsage{
		InputTokens:  usage.PromptTokens,
		OutputTokens: usage.CompletionTokens,
		TotalTokens:  usage.TotalTokens,
	}
	if usage.PromptTokensDetails != nil && usage.PromptTokensDetails.CachedTokens > 0 {
		out.InputTokensDetails = &ResponsesInputTokensDetails{
			CachedTokens: usage.PromptTokensDetails.CachedTokens,
		}
	}
	return out
}

func responseIDFromChatID(id string) string {
	if strings.TrimSpace(id) == "" {
		return generateResponsesID()
	}
	if strings.HasPrefix(id, "resp_") {
		return id
	}
	return "resp_" + strings.TrimPrefix(id, "chatcmpl_")
}

type chatResponsesToolState struct {
	OutputIndex int
	ItemID      string
	CallID      string
	Name        string
	Arguments   strings.Builder
	Open        bool
}

// ChatCompletionsToResponsesStreamState tracks a Chat Completions stream while
// emitting Responses SSE events.
type ChatCompletionsToResponsesStreamState struct {
	ResponseID     string
	Model          string
	Created        int64
	SequenceNumber int
	CreatedSent    bool
	CompletedSent  bool

	NextOutputIndex int
	MessageOpen     bool
	MessageItemID   string
	MessageIndex    int
	TextDoneSent    bool

	ReasoningOpen   bool
	ReasoningItemID string
	ReasoningIndex  int

	Tools map[int]*chatResponsesToolState
	Usage *ChatUsage
}

func NewChatCompletionsToResponsesStreamState(model string) *ChatCompletionsToResponsesStreamState {
	return &ChatCompletionsToResponsesStreamState{
		Model:   model,
		Created: time.Now().Unix(),
		Tools:   make(map[int]*chatResponsesToolState),
	}
}

// ChatCompletionsChunkToResponsesEvents converts one Chat Completions stream
// chunk into zero or more Responses SSE events.
func ChatCompletionsChunkToResponsesEvents(chunk *ChatCompletionsChunk, state *ChatCompletionsToResponsesStreamState) []ResponsesStreamEvent {
	if chunk == nil || state == nil {
		return nil
	}
	if state.ResponseID == "" {
		state.ResponseID = responseIDFromChatID(chunk.ID)
	}
	if state.Model == "" {
		state.Model = chunk.Model
	}
	if chunk.Usage != nil {
		state.Usage = chunk.Usage
	}

	var events []ResponsesStreamEvent
	events = append(events, ensureChatResponsesCreated(state)...)

	for _, choice := range chunk.Choices {
		if choice.Delta.ReasoningContent != nil && *choice.Delta.ReasoningContent != "" {
			events = append(events, openChatResponsesReasoning(state)...)
			events = append(events, nextChatResponsesEvent(state, "response.reasoning_summary_text.delta", &ResponsesStreamEvent{
				OutputIndex:  state.ReasoningIndex,
				SummaryIndex: 0,
				ItemID:       state.ReasoningItemID,
				Delta:        *choice.Delta.ReasoningContent,
			}))
		}
		if choice.Delta.Content != nil && *choice.Delta.Content != "" {
			events = append(events, closeChatResponsesReasoning(state)...)
			events = append(events, openChatResponsesMessage(state)...)
			events = append(events, nextChatResponsesEvent(state, "response.output_text.delta", &ResponsesStreamEvent{
				OutputIndex:  state.MessageIndex,
				ContentIndex: 0,
				ItemID:       state.MessageItemID,
				Delta:        *choice.Delta.Content,
			}))
		}
		for _, toolCall := range choice.Delta.ToolCalls {
			idx := 0
			if toolCall.Index != nil {
				idx = *toolCall.Index
			}
			events = append(events, closeChatResponsesMessage(state)...)
			events = append(events, openChatResponsesTool(state, idx, toolCall)...)
			if toolCall.Function.Arguments != "" {
				tool := state.Tools[idx]
				tool.Arguments.WriteString(toolCall.Function.Arguments)
				events = append(events, nextChatResponsesEvent(state, "response.function_call_arguments.delta", &ResponsesStreamEvent{
					OutputIndex: tool.OutputIndex,
					ItemID:      tool.ItemID,
					CallID:      tool.CallID,
					Name:        tool.Name,
					Delta:       toolCall.Function.Arguments,
				}))
			}
		}
		if choice.FinishReason != nil {
			events = append(events, closeChatResponsesReasoning(state)...)
			events = append(events, closeChatResponsesMessage(state)...)
			for idx := range state.Tools {
				events = append(events, closeChatResponsesTool(state, idx)...)
			}
		}
	}
	return events
}

func FinalizeChatCompletionsResponsesStream(state *ChatCompletionsToResponsesStreamState) []ResponsesStreamEvent {
	if state == nil || state.CompletedSent {
		return nil
	}
	var events []ResponsesStreamEvent
	events = append(events, ensureChatResponsesCreated(state)...)
	events = append(events, closeChatResponsesReasoning(state)...)
	events = append(events, closeChatResponsesMessage(state)...)
	for idx := range state.Tools {
		events = append(events, closeChatResponsesTool(state, idx)...)
	}
	events = append(events, nextChatResponsesEvent(state, "response.completed", &ResponsesStreamEvent{
		Response: &ResponsesResponse{
			ID:     state.ResponseID,
			Object: "response",
			Model:  state.Model,
			Status: "completed",
			Output: []ResponsesOutput{},
			Usage:  chatUsageToResponsesUsage(state.Usage),
		},
	}))
	state.CompletedSent = true
	return events
}

func ensureChatResponsesCreated(state *ChatCompletionsToResponsesStreamState) []ResponsesStreamEvent {
	if state.CreatedSent {
		return nil
	}
	state.CreatedSent = true
	if state.ResponseID == "" {
		state.ResponseID = generateResponsesID()
	}
	return []ResponsesStreamEvent{nextChatResponsesEvent(state, "response.created", &ResponsesStreamEvent{
		Response: &ResponsesResponse{
			ID:     state.ResponseID,
			Object: "response",
			Model:  state.Model,
			Status: "in_progress",
			Output: []ResponsesOutput{},
		},
	})}
}

func openChatResponsesMessage(state *ChatCompletionsToResponsesStreamState) []ResponsesStreamEvent {
	if state.MessageOpen {
		return nil
	}
	state.MessageOpen = true
	state.TextDoneSent = false
	state.MessageItemID = generateItemID()
	state.MessageIndex = state.NextOutputIndex
	state.NextOutputIndex++
	return []ResponsesStreamEvent{nextChatResponsesEvent(state, "response.output_item.added", &ResponsesStreamEvent{
		OutputIndex: state.MessageIndex,
		Item: &ResponsesOutput{
			Type:   "message",
			ID:     state.MessageItemID,
			Role:   "assistant",
			Status: "in_progress",
		},
	})}
}

func closeChatResponsesMessage(state *ChatCompletionsToResponsesStreamState) []ResponsesStreamEvent {
	if !state.MessageOpen {
		return nil
	}
	var events []ResponsesStreamEvent
	if !state.TextDoneSent {
		events = append(events, nextChatResponsesEvent(state, "response.output_text.done", &ResponsesStreamEvent{
			OutputIndex:  state.MessageIndex,
			ContentIndex: 0,
			ItemID:       state.MessageItemID,
		}))
	}
	events = append(events, nextChatResponsesEvent(state, "response.output_item.done", &ResponsesStreamEvent{
		OutputIndex: state.MessageIndex,
		Item: &ResponsesOutput{
			Type:   "message",
			ID:     state.MessageItemID,
			Role:   "assistant",
			Status: "completed",
		},
	}))
	state.MessageOpen = false
	state.MessageItemID = ""
	state.TextDoneSent = true
	return events
}

func openChatResponsesReasoning(state *ChatCompletionsToResponsesStreamState) []ResponsesStreamEvent {
	if state.ReasoningOpen {
		return nil
	}
	state.ReasoningOpen = true
	state.ReasoningItemID = generateItemID()
	state.ReasoningIndex = state.NextOutputIndex
	state.NextOutputIndex++
	return []ResponsesStreamEvent{nextChatResponsesEvent(state, "response.output_item.added", &ResponsesStreamEvent{
		OutputIndex: state.ReasoningIndex,
		Item: &ResponsesOutput{
			Type: "reasoning",
			ID:   state.ReasoningItemID,
		},
	})}
}

func closeChatResponsesReasoning(state *ChatCompletionsToResponsesStreamState) []ResponsesStreamEvent {
	if !state.ReasoningOpen {
		return nil
	}
	events := []ResponsesStreamEvent{
		nextChatResponsesEvent(state, "response.reasoning_summary_text.done", &ResponsesStreamEvent{
			OutputIndex:  state.ReasoningIndex,
			SummaryIndex: 0,
			ItemID:       state.ReasoningItemID,
		}),
		nextChatResponsesEvent(state, "response.output_item.done", &ResponsesStreamEvent{
			OutputIndex: state.ReasoningIndex,
			Item: &ResponsesOutput{
				Type: "reasoning",
				ID:   state.ReasoningItemID,
			},
		}),
	}
	state.ReasoningOpen = false
	state.ReasoningItemID = ""
	return events
}

func openChatResponsesTool(state *ChatCompletionsToResponsesStreamState, idx int, delta ChatToolCall) []ResponsesStreamEvent {
	tool := state.Tools[idx]
	if tool == nil {
		tool = &chatResponsesToolState{
			OutputIndex: state.NextOutputIndex,
			ItemID:      generateItemID(),
			CallID:      delta.ID,
			Name:        delta.Function.Name,
			Open:        true,
		}
		if tool.CallID == "" {
			tool.CallID = fmt.Sprintf("call_%d", idx)
		}
		state.Tools[idx] = tool
		state.NextOutputIndex++
		return []ResponsesStreamEvent{nextChatResponsesEvent(state, "response.output_item.added", &ResponsesStreamEvent{
			OutputIndex: tool.OutputIndex,
			Item: &ResponsesOutput{
				Type:   "function_call",
				ID:     tool.ItemID,
				CallID: tool.CallID,
				Name:   tool.Name,
				Status: "in_progress",
			},
		})}
	}
	if delta.ID != "" {
		tool.CallID = delta.ID
	}
	if delta.Function.Name != "" {
		tool.Name = delta.Function.Name
	}
	return nil
}

func closeChatResponsesTool(state *ChatCompletionsToResponsesStreamState, idx int) []ResponsesStreamEvent {
	tool := state.Tools[idx]
	if tool == nil || !tool.Open {
		return nil
	}
	tool.Open = false
	return []ResponsesStreamEvent{
		nextChatResponsesEvent(state, "response.function_call_arguments.done", &ResponsesStreamEvent{
			OutputIndex: tool.OutputIndex,
			ItemID:      tool.ItemID,
			CallID:      tool.CallID,
			Name:        tool.Name,
			Arguments:   tool.Arguments.String(),
		}),
		nextChatResponsesEvent(state, "response.output_item.done", &ResponsesStreamEvent{
			OutputIndex: tool.OutputIndex,
			Item: &ResponsesOutput{
				Type:      "function_call",
				ID:        tool.ItemID,
				CallID:    tool.CallID,
				Name:      tool.Name,
				Arguments: tool.Arguments.String(),
				Status:    "completed",
			},
		}),
	}
}

func nextChatResponsesEvent(state *ChatCompletionsToResponsesStreamState, eventType string, template *ResponsesStreamEvent) ResponsesStreamEvent {
	evt := *template
	evt.Type = eventType
	evt.SequenceNumber = state.SequenceNumber
	state.SequenceNumber++
	return evt
}
