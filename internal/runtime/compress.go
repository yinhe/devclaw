package runtime

import (
	"context"
	"fmt"
	"strings"

	"github.com/yinhe/devclaw/internal/provider"
)

// CompressMessages compresses conversation history to stay within context limits.
// Uses the LLM itself to summarize older messages while preserving recent context.
func CompressMessages(ctx context.Context, p provider.Provider, model string, messages []provider.ChatMessage, keepRecent int) ([]provider.ChatMessage, string, error) {
	if keepRecent <= 0 {
		keepRecent = 6 // Keep last 6 messages (3 turns)
	}
	if len(messages) <= keepRecent {
		return messages, "", nil
	}

	// Split into old (to compress) and recent (to keep)
	oldMsgs := messages[:len(messages)-keepRecent]
	recentMsgs := messages[len(messages)-keepRecent:]

	// Build a summary of old messages
	var sb strings.Builder
	for _, m := range oldMsgs {
		if m.Role == "system" {
			continue
		}
		content := m.Content
		if len(content) > 500 {
			content = content[:500] + "..."
		}
		if m.Role == "tool" {
			if len(content) > 200 {
				content = content[:200] + "..."
			}
			sb.WriteString(fmt.Sprintf("[tool result]: %s\n", content))
		} else {
			sb.WriteString(fmt.Sprintf("[%s]: %s\n", m.Role, content))
		}
	}

	// Ask LLM to compress
	compressPrompt := fmt.Sprintf(`Compress the following conversation history into a concise summary. 
Preserve: key decisions, code changes, file paths, error messages, task progress.
Drop: verbose tool outputs, redundant exchanges, thinking process.
Output a single paragraph or short bullet list. Be factual, not narrative.

--- CONVERSATION ---
%s
--- END ---

Summary:`, sb.String())

	req := &provider.ChatRequest{
		Model: model,
		Messages: []provider.ChatMessage{
			{Role: "user", Content: compressPrompt},
		},
		MaxTokens:   1024,
		Temperature: 0.3,
	}

	ch, err := p.Chat(ctx, req)
	if err != nil {
		// On failure, do simple truncation instead
		return simpleCompress(messages, keepRecent), "(compressed via truncation)", nil
	}

	var summary string
	for chunk := range ch {
		if chunk.Content != "" {
			summary += chunk.Content
		}
	}

	if summary == "" {
		return simpleCompress(messages, keepRecent), "(compressed via truncation)", nil
	}

	// Build new message list: summary + recent messages
	compressed := make([]provider.ChatMessage, 0, keepRecent+2)
	compressed = append(compressed, provider.ChatMessage{
		Role:    "user",
		Content: fmt.Sprintf("[Previous task progress summary]\n%s", summary),
	})
	compressed = append(compressed, provider.ChatMessage{
		Role:    "assistant",
		Content: "Understood. I have the context from previous work. Continuing the task.",
	})
	compressed = append(compressed, recentMsgs...)

	stats := fmt.Sprintf("Compressed %d messages → summary + %d recent (saved ~%d tokens)",
		len(oldMsgs), keepRecent, estimateTokens(oldMsgs)-estimateTokens(compressed[:2]))

	return compressed, stats, nil
}

// simpleCompress just keeps recent messages with a note about truncation
func simpleCompress(messages []provider.ChatMessage, keepRecent int) []provider.ChatMessage {
	if len(messages) <= keepRecent {
		return messages
	}
	truncated := len(messages) - keepRecent
	result := make([]provider.ChatMessage, 0, keepRecent+2)
	result = append(result, provider.ChatMessage{
		Role:    "user",
		Content: fmt.Sprintf("[Note: %d earlier messages were truncated to save context space]", truncated),
	})
	result = append(result, provider.ChatMessage{
		Role:    "assistant",
		Content: "Understood, I'll continue with the available context.",
	})
	result = append(result, messages[len(messages)-keepRecent:]...)
	return result
}

// EstimateTokens gives a rough token count (1 token ≈ 4 chars)
func EstimateTokens(msgs []provider.ChatMessage) int {
	return estimateTokens(msgs)
}

func estimateTokens(msgs []provider.ChatMessage) int {
	total := 0
	for _, m := range msgs {
		total += len(m.Content)/4 + 4
		for _, tc := range m.ToolCalls {
			total += len(tc.Function.Arguments)/4 + 10
		}
	}
	return total
}

// ShouldCompress returns true if messages are getting too long
func ShouldCompress(messages []provider.ChatMessage, maxTokens int) bool {
	if maxTokens <= 0 {
		maxTokens = 32000 // Default ~32K context
	}
	return estimateTokens(messages) > maxTokens*3/4 // Compress at 75% capacity
}
