package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

type openAIToolCall struct {
	ID       string         `json:"id"`
	Type     string         `json:"type"`
	Function toolFunction   `json:"function"`
}

type toolFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type plannerResponse struct {
	Type      string            `json:"type"`
	Content   string            `json:"content,omitempty"`
	ToolCalls []plannerToolCall `json:"tool_calls,omitempty"`
}

type plannerToolCall struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

func hasToolMessages(messages []chatMessage) bool {
	for _, message := range messages {
		if message.Role == "tool" {
			return true
		}
	}
	return false
}

func buildConversationPrompt(messages []chatMessage) string {
	parts := make([]string, 0, len(messages))
	for _, message := range messages {
		role := strings.TrimSpace(message.Role)
		if role == "" {
			role = "user"
		}

		switch role {
		case "assistant":
			content := stringifyContent(message.Content)
			if content != "" {
				parts = append(parts, "ASSISTANT: "+content)
			}
			if len(message.ToolCalls) > 0 {
				raw, _ := json.Marshal(message.ToolCalls)
				parts = append(parts, "ASSISTANT_TOOL_CALLS: "+string(raw))
			}
		case "tool":
			content := stringifyContent(message.Content)
			label := "TOOL_RESULT"
			if message.ToolCallID != "" {
				label += " id=" + message.ToolCallID
			}
			parts = append(parts, label+": "+content)
		default:
			content := stringifyContent(message.Content)
			if content == "" {
				continue
			}
			parts = append(parts, strings.ToUpper(role)+": "+content)
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}

func buildToolPlannerPrompt(messages []chatMessage, tools []json.RawMessage, toolChoice any) string {
	toolsBlock := "[]"
	if len(tools) > 0 {
		raw, _ := json.MarshalIndent(tools, "", "  ")
		toolsBlock = string(raw)
	}

	toolChoiceBlock := "null"
	if toolChoice != nil {
		raw, _ := json.MarshalIndent(toolChoice, "", "  ")
		toolChoiceBlock = string(raw)
	}

	return strings.TrimSpace(fmt.Sprintf(`
You are an OpenAI-compatible tool planner.

Decide whether to answer directly or request one or more tool calls.

Available tools:
%s

Requested tool_choice:
%s

Conversation:
%s

Return strict JSON only, with no markdown fences and no extra commentary.

If you can answer directly, return:
{"type":"final","content":"your answer"}

If a tool is required, return:
{"type":"tool_calls","tool_calls":[{"name":"tool_name","arguments":{...}}]}
`, toolsBlock, toolChoiceBlock, buildConversationPrompt(messages)))
}

func parsePlannerResponse(raw string) (*plannerResponse, error) {
	jsonText := extractJSONObject(raw)
	if jsonText == "" {
		return nil, fmt.Errorf("planner did not return json")
	}

	var parsed plannerResponse
	if err := json.Unmarshal([]byte(jsonText), &parsed); err != nil {
		return nil, err
	}

	parsed.Type = strings.TrimSpace(parsed.Type)
	if parsed.Type == "" {
		return nil, fmt.Errorf("planner response missing type")
	}
	return &parsed, nil
}

func extractJSONObject(raw string) string {
	start := strings.Index(raw, "{")
	if start == -1 {
		return ""
	}

	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(raw); i++ {
		ch := raw[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}

		switch ch {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return strings.TrimSpace(raw[start : i+1])
			}
		}
	}

	return ""
}

func toOpenAIToolCalls(toolCalls []plannerToolCall) []openAIToolCall {
	out := make([]openAIToolCall, 0, len(toolCalls))
	for _, call := range toolCalls {
		args := "{}"
		if len(call.Arguments) > 0 {
			args = string(call.Arguments)
		}
		out = append(out, openAIToolCall{
			ID:   "call_" + shortID(),
			Type: "function",
			Function: toolFunction{
				Name:      strings.TrimSpace(call.Name),
				Arguments: args,
			},
		})
	}
	return out
}
