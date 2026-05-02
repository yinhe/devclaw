package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// OpenAIProvider implements Provider for OpenAI-compatible APIs (OpenAI, Synapse, Ollama)
type OpenAIProvider struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

// OpenAIConfig configures the OpenAI provider
type OpenAIConfig struct {
	APIKey  string
	BaseURL string
}

// NewOpenAIProvider creates a new OpenAI-compatible provider
func NewOpenAIProvider(cfg OpenAIConfig) *OpenAIProvider {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	return &OpenAIProvider{
		apiKey:  cfg.APIKey,
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: 5 * time.Minute},
	}
}

func (p *OpenAIProvider) Name() string { return "openai" }

func (p *OpenAIProvider) Chat(ctx context.Context, req *ChatRequest) (<-chan *ChatChunk, error) {
	req.Stream = true

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		var buf bytes.Buffer
		buf.ReadFrom(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, buf.String())
	}

	ch := make(chan *ChatChunk, 32)

	go func() {
		defer close(ch)
		defer resp.Body.Close()

		// Accumulate tool calls across deltas
		toolBufs := make(map[int]*ToolCall)

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 256*1024), 256*1024)

		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				// Emit accumulated tool calls
				for _, tc := range toolBufs {
					select {
					case ch <- &ChatChunk{Tool: tc}:
					case <-ctx.Done():
						return
					}
				}
				select {
				case ch <- &ChatChunk{Done: true}:
				case <-ctx.Done():
				}
				return
			}

			var streamResp openAIStreamResponse
			if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
				continue
			}

			if len(streamResp.Choices) == 0 {
				// Check for usage in stream_options
				if streamResp.Usage != nil {
					select {
					case ch <- &ChatChunk{Usage: streamResp.Usage}:
					case <-ctx.Done():
						return
					}
				}
				continue
			}

			delta := streamResp.Choices[0].Delta

			// Text content
			if delta.Content != "" {
				select {
				case ch <- &ChatChunk{Content: delta.Content}:
				case <-ctx.Done():
					return
				}
			}

			// Tool call deltas — accumulate
			for _, tc := range delta.ToolCalls {
				idx := tc.Index
				if _, ok := toolBufs[idx]; !ok {
					toolBufs[idx] = &ToolCall{
						ID:   tc.ID,
						Type: "function",
						Function: FunctionCall{
							Name:      tc.Function.Name,
							Arguments: tc.Function.Arguments,
						},
					}
				} else {
					buf := toolBufs[idx]
					if tc.ID != "" {
						buf.ID = tc.ID
					}
					if tc.Function.Name != "" {
						buf.Function.Name = tc.Function.Name
					}
					buf.Function.Arguments += tc.Function.Arguments
				}
			}

			// Usage
			if streamResp.Usage != nil {
				select {
				case ch <- &ChatChunk{Usage: streamResp.Usage}:
				case <-ctx.Done():
					return
				}
			}

			// Finish reason — emit tool calls if finished
			if streamResp.Choices[0].FinishReason == "tool_calls" {
				for _, tc := range toolBufs {
					select {
					case ch <- &ChatChunk{Tool: tc}:
					case <-ctx.Done():
						return
					}
				}
				toolBufs = make(map[int]*ToolCall)
			}
		}
	}()

	return ch, nil
}

// Internal SSE response types
type openAIStreamResponse struct {
	ID      string               `json:"id"`
	Choices []openAIStreamChoice `json:"choices"`
	Usage   *TokenUsage          `json:"usage,omitempty"`
}

type openAIStreamChoice struct {
	Delta        openAIStreamDelta `json:"delta"`
	FinishReason string            `json:"finish_reason"`
}

type openAIStreamDelta struct {
	Role      string                 `json:"role"`
	Content   string                 `json:"content"`
	ToolCalls []openAIStreamToolCall `json:"tool_calls"`
}

type openAIStreamToolCall struct {
	Index    int          `json:"index"`
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}
