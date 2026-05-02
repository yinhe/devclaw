package provider

import (
	"context"
	"math"
	"time"
)

// RetryProvider wraps a Provider with exponential backoff retry
type RetryProvider struct {
	inner      Provider
	maxRetries int
	baseDelay  time.Duration
}

// NewRetryProvider wraps a provider with retry logic
func NewRetryProvider(p Provider, maxRetries int) *RetryProvider {
	return &RetryProvider{
		inner:      p,
		maxRetries: maxRetries,
		baseDelay:  500 * time.Millisecond,
	}
}

func (r *RetryProvider) Name() string { return r.inner.Name() }

func (r *RetryProvider) Chat(ctx context.Context, req *ChatRequest) (<-chan *ChatChunk, error) {
	var lastErr error
	for attempt := 0; attempt <= r.maxRetries; attempt++ {
		ch, err := r.inner.Chat(ctx, req)
		if err == nil {
			return ch, nil
		}
		lastErr = err
		if attempt < r.maxRetries {
			delay := time.Duration(math.Pow(2, float64(attempt))) * r.baseDelay
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
	}
	return nil, lastErr
}

// FallbackProvider tries multiple models in order
type FallbackProvider struct {
	provider *OpenAIProvider
	models   []string
}

// NewFallbackProvider creates a provider that tries models in order
func NewFallbackProvider(p *OpenAIProvider, models []string) *FallbackProvider {
	return &FallbackProvider{provider: p, models: models}
}

func (f *FallbackProvider) Name() string { return "fallback" }

func (f *FallbackProvider) Chat(ctx context.Context, req *ChatRequest) (<-chan *ChatChunk, error) {
	originalModel := req.Model
	var lastErr error

	// Try each model
	for _, model := range f.models {
		req.Model = model
		ch, err := f.provider.Chat(ctx, req)
		if err == nil {
			return ch, nil
		}
		lastErr = err
	}

	// Restore original
	req.Model = originalModel
	return nil, lastErr
}
