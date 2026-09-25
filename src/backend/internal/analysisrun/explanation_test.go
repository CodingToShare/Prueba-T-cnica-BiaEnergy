package analysisrun

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"bia-energy.local/backend/internal/analysis"
)

type auditProviderFunc func(context.Context, ExplanationInput) (Explanation, error)

func (f auditProviderFunc) Explain(ctx context.Context, in ExplanationInput) (Explanation, error) {
	return f(ctx, in)
}

func TestExplain_RunCancellationStopsWithoutCallingOrLoggingFallback(t *testing.T) {
	started := make(chan struct{})
	var fallbackCalls atomic.Int64
	s := &Service{
		explainers: Explainers{
			Primary: auditProviderFunc(func(ctx context.Context, _ ExplanationInput) (Explanation, error) {
				close(started)
				<-ctx.Done()
				return Explanation{}, ctx.Err()
			}),
			Fallback: auditProviderFunc(func(context.Context, ExplanationInput) (Explanation, error) {
				fallbackCalls.Add(1)
				return Explanation{}, nil
			}),
			Settings: ExplanationSettings{Provider: "audit", PromptVersion: "audit-v1"},
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-started
		cancel()
	}()

	out, err := s.explain(ctx, []analysis.Finding{{MeterID: "SYN-1"}}, []Evidence{{}}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	require.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled))
	assert.Nil(t, out)
	assert.Zero(t, fallbackCalls.Load(), "run cancellation is lifecycle control, not provider fallback")
}
