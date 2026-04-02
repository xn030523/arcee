package arcee

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type CreateChatRequest struct {
	Message            string   `json:"message"`
	Title              string   `json:"title"`
	BaseModelName      string   `json:"base_model_name"`
	ChatID             string   `json:"chat_id,omitempty"`
	EnabledTools       []string `json:"enabledTools"`
	FileReferences     []any    `json:"fileReferences"`
	Temperature        float64  `json:"temperature"`
	ProviderPreference any      `json:"provider_preference"`
}

type StreamInit struct {
	AssistantMessageID string `json:"assistant_message_id"`
}

type StreamMetadata struct {
	ChatID             string `json:"chat_id"`
	UserMessageID      string `json:"user_message_id"`
	AssistantMessageID string `json:"assistant_message_id"`
	BaseModelName      string `json:"base_model_name"`
}

type CreateChatResult struct {
	Init     StreamInit
	Content  string
	Metadata StreamMetadata
	Raw      string
}

const (
	streamInitStart = "__STREAM_INIT__"
	streamInitEnd   = "__STREAM_INIT_END__"
	metadataStart   = "__METADATA__"
	metadataEnd     = "__METADATA_END__"
)

func (c *Client) CreateChat(ctx context.Context, accessToken string, reqBody CreateChatRequest) (*CreateChatResult, error) {
	if accessToken == "" {
		return nil, fmt.Errorf("access token is required")
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal create-chat request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/completions/create-chat", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create create-chat request: %w", err)
	}

	if err := setBrowserHeaders(httpReq); err != nil {
		return nil, err
	}
	httpReq.Header.Set("Accept", "text/plain")
	httpReq.Header.Set("Cookie", "access_token="+accessToken)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send create-chat request: %w", err)
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read create-chat response: %w", err)
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("arcee create-chat failed: status=%d body=%s", resp.StatusCode, string(rawBody))
	}

	return parseCreateChatResponse(string(rawBody))
}

func parseCreateChatResponse(raw string) (*CreateChatResult, error) {
	result := &CreateChatResult{Raw: raw}

	initStart := strings.Index(raw, streamInitStart)
	initEnd := strings.Index(raw, streamInitEnd)
	metaStart := strings.Index(raw, metadataStart)
	metaEnd := strings.Index(raw, metadataEnd)

	if initStart == -1 || initEnd == -1 || metaStart == -1 || metaEnd == -1 {
		return nil, fmt.Errorf("unexpected create-chat response format")
	}

	initJSON := raw[initStart+len(streamInitStart) : initEnd]
	if err := json.Unmarshal([]byte(initJSON), &result.Init); err != nil {
		return nil, fmt.Errorf("decode stream init: %w", err)
	}

	content := raw[initEnd+len(streamInitEnd) : metaStart]
	result.Content = strings.TrimSpace(content)

	metaJSON := raw[metaStart+len(metadataStart) : metaEnd]
	if err := json.Unmarshal([]byte(metaJSON), &result.Metadata); err != nil {
		return nil, fmt.Errorf("decode stream metadata: %w", err)
	}

	return result, nil
}
