package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"arcee/arcee"
	appconfig "arcee/config"
)

type chatCompletionsRequest struct {
	Model       string            `json:"model"`
	Messages    []chatMessage     `json:"messages"`
	Temperature *float64          `json:"temperature,omitempty"`
	Stream      bool              `json:"stream,omitempty"`
	Tools       []json.RawMessage `json:"tools,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type chatCompletionsResponse struct {
	ID      string                 `json:"id"`
	Object  string                 `json:"object"`
	Created int64                  `json:"created"`
	Model   string                 `json:"model"`
	Choices []chatCompletionChoice `json:"choices"`
}

type chatCompletionChoice struct {
	Index        int              `json:"index"`
	Message      assistantMessage `json:"message,omitempty"`
	Delta        assistantMessage `json:"delta,omitempty"`
	FinishReason string           `json:"finish_reason,omitempty"`
}

type assistantMessage struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

func runServer(cfg *appconfig.Config) {
	accessToken, err := cfg.Server.ResolvedAccessToken()
	if err != nil {
		log.Fatal(err)
	}

	httpClient := &http.Client{Timeout: 60 * time.Second}
	arceeClient := arcee.NewClient(arcee.WithHTTPClient(httpClient))

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	modelsHandler := func(w http.ResponseWriter, r *http.Request) {
		if !authorize(cfg.Server, w, r) {
			return
		}
		models := make([]map[string]any, 0, len(cfg.Server.SupportedModels()))
		for _, model := range cfg.Server.SupportedModels() {
			models = append(models, map[string]any{
				"id":       model,
				"object":   "model",
				"created":  time.Now().Unix(),
				"owned_by": "arcee",
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"object": "list",
			"data":   models,
		})
	}
	mux.HandleFunc("/v1/models", modelsHandler)
	mux.HandleFunc("/models", modelsHandler)
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !authorize(cfg.Server, w, r) {
			return
		}
		handleChatCompletions(cfg.Server, accessToken, arceeClient, w, r)
	})

	server := &http.Server{
		Addr:              cfg.Server.ResolvedListen(),
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("openai-compatible gateway listening on http://%s", cfg.Server.ResolvedListen())
	log.Fatal(server.ListenAndServe())
}

func handleChatCompletions(cfg appconfig.ServerConfig, accessToken string, client *arcee.Client, w http.ResponseWriter, r *http.Request) {
	var req chatCompletionsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}

	prompt := buildPrompt(req.Messages)
	if prompt == "" {
		http.Error(w, "at least one message with content is required", http.StatusBadRequest)
		return
	}

	temp := 0.3
	if req.Temperature != nil {
		temp = *req.Temperature
	}
	modelName := resolveModelName(cfg, req.Model)

	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()

	result, err := client.CreateChat(ctx, accessToken, arcee.CreateChatRequest{
		Message:            prompt,
		Title:              buildTitle(req.Messages),
		BaseModelName:      modelName,
		EnabledTools:       resolveEnabledTools(cfg, req.Tools),
		FileReferences:     []any{},
		Temperature:        temp,
		ProviderPreference: nil,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	if req.Stream {
		writeStreamResponse(w, modelName, result)
		return
	}

	writeJSON(w, http.StatusOK, chatCompletionsResponse{
		ID:      "chatcmpl-" + shortID(),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   modelName,
		Choices: []chatCompletionChoice{{
			Index: 0,
			Message: assistantMessage{
				Role:    "assistant",
				Content: result.Content,
			},
			FinishReason: "stop",
		}},
	})
}

func resolveModelName(cfg appconfig.ServerConfig, requested string) string {
	requested = strings.TrimSpace(requested)
	if requested == "" {
		return cfg.ResolvedModel()
	}
	for _, model := range cfg.SupportedModels() {
		if model == requested {
			return requested
		}
	}
	return cfg.ResolvedModel()
}

func resolveEnabledTools(cfg appconfig.ServerConfig, requestTools []json.RawMessage) []string {
	if len(cfg.EnabledTools) > 0 {
		return cfg.EnabledTools
	}

	enabled := []string{}
	for _, raw := range requestTools {
		if strings.Contains(strings.ToLower(string(raw)), "web_search") {
			enabled = append(enabled, "web_search")
			break
		}
	}
	return enabled
}

func authorize(cfg appconfig.ServerConfig, w http.ResponseWriter, r *http.Request) bool {
	if cfg.OpenAIAPIKey == "" {
		return true
	}
	if strings.TrimSpace(r.Header.Get("Authorization")) != "Bearer "+cfg.OpenAIAPIKey {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return false
	}
	return true
}

func buildPrompt(messages []chatMessage) string {
	parts := make([]string, 0, len(messages))
	for _, message := range messages {
		content := stringifyContent(message.Content)
		if content == "" {
			continue
		}
		role := message.Role
		if role == "" {
			role = "user"
		}
		parts = append(parts, strings.ToUpper(role)+": "+content)
	}
	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}

func buildTitle(messages []chatMessage) string {
	for _, message := range messages {
		if message.Role == "user" {
			if content := stringifyContent(message.Content); content != "" {
				return firstNRunes(content, 80)
			}
		}
	}
	return "New Chat"
}

func stringifyContent(content any) string {
	switch value := content.(type) {
	case string:
		return strings.TrimSpace(value)
	case []any:
		parts := make([]string, 0, len(value))
		for _, item := range value {
			if m, ok := item.(map[string]any); ok {
				if text, ok := m["text"].(string); ok {
					parts = append(parts, strings.TrimSpace(text))
				}
			}
		}
		return strings.TrimSpace(strings.Join(parts, "\n"))
	default:
		return ""
	}
}

func firstNRunes(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}

func writeStreamResponse(w http.ResponseWriter, model string, result *arcee.CreateChatResult) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	writeSSE(w, chatCompletionsResponse{
		ID:      "chatcmpl-" + shortID(),
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []chatCompletionChoice{{
			Index: 0,
			Delta: assistantMessage{
				Role:    "assistant",
				Content: result.Content,
			},
		}},
	})

	writeSSE(w, chatCompletionsResponse{
		ID:      "chatcmpl-" + shortID(),
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []chatCompletionChoice{{
			Index:        0,
			Delta:        assistantMessage{},
			FinishReason: "stop",
		}},
	})

	_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
	flusher.Flush()
}

func writeSSE(w http.ResponseWriter, payload any) {
	raw, _ := json.Marshal(payload)
	_, _ = fmt.Fprintf(w, "data: %s\n\n", raw)
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func shortID() string {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(raw[:])
}
