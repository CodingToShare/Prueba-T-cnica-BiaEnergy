package explanation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"bia-energy.local/backend/internal/analysisrun"
)

// maxResponseBytes bounds what is read from Ollama; a reply of four short
// fields is a few kilobytes.
const maxResponseBytes = 64 << 10

// maxOutputTokens bounds generation on the model side.
const maxOutputTokens = 600

// OllamaConfig configures the local Ollama provider. BaseURL is trusted
// server configuration, never user input.
type OllamaConfig struct {
	BaseURL string
	Model   string
	Timeout time.Duration // per explanation
}

// Ollama asks a local model, through the Ollama HTTP API, to reword the
// evidence of one finding. Every failure is returned as an
// *analysisrun.ExplanationError so that orchestration falls back to the
// deterministic text with a sanitized code.
type Ollama struct {
	endpoint string
	model    string
	timeout  time.Duration
	client   *http.Client
}

// NewOllama validates the configuration.
func NewOllama(cfg OllamaConfig) (*Ollama, error) {
	u, err := url.Parse(strings.TrimRight(cfg.BaseURL, "/"))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil {
		return nil, errors.New("the Ollama base URL must be an http(s) URL with a host and no credentials")
	}
	if strings.TrimSpace(cfg.Model) == "" {
		return nil, errors.New("an Ollama model is required")
	}
	if cfg.Timeout <= 0 {
		return nil, errors.New("the Ollama timeout must be positive")
	}
	return &Ollama{
		endpoint: u.JoinPath("api", "chat").String(),
		model:    strings.TrimSpace(cfg.Model),
		timeout:  cfg.Timeout,
		// The context carries the deadline; the client adds no redirects to
		// other hosts beyond the configured one.
		client: &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }},
	}, nil
}

// Settings is the run provenance of this provider (no URL).
func (o *Ollama) Settings() analysisrun.ExplanationSettings {
	model := o.model
	ms := o.timeout.Milliseconds()
	return analysisrun.ExplanationSettings{Provider: "ollama", Model: &model, PromptVersion: PromptVersion, TimeoutMS: &ms}
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model    string         `json:"model"`
	Messages []chatMessage  `json:"messages"`
	Stream   bool           `json:"stream"`
	Format   map[string]any `json:"format"`
	Options  map[string]any `json:"options"`
}

type chatResponse struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
	Done bool `json:"done"`
}

// Explain implements analysisrun.ExplanationProvider.
func (o *Ollama) Explain(ctx context.Context, in analysisrun.ExplanationInput) (analysisrun.Explanation, error) {
	payload := payloadOf(in)
	evidence, err := json.Marshal(payload)
	if err != nil {
		return analysisrun.Explanation{}, fail(analysisrun.FallbackProviderError, err)
	}
	body, err := json.Marshal(chatRequest{
		Model: o.model,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: string(evidence)},
		},
		Stream:  false,
		Format:  outputSchema(payload, in.Severity),
		Options: map[string]any{"temperature": 0.2, "num_predict": maxOutputTokens},
	})
	if err != nil {
		return analysisrun.Explanation{}, fail(analysisrun.FallbackProviderError, err)
	}

	ctx, cancel := context.WithTimeout(ctx, o.timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.endpoint, bytes.NewReader(body))
	if err != nil {
		return analysisrun.Explanation{}, fail(analysisrun.FallbackProviderError, err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := o.client.Do(req)
	if err != nil {
		if cerr := contextFailure(ctx); cerr != nil {
			return analysisrun.Explanation{}, cerr
		}
		return analysisrun.Explanation{}, fail(analysisrun.FallbackProviderUnavailable, errors.New("the provider could not be reached"))
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return analysisrun.Explanation{}, fail(analysisrun.FallbackProviderError, fmt.Errorf("the provider answered HTTP %d", resp.StatusCode))
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		if cerr := contextFailure(ctx); cerr != nil {
			return analysisrun.Explanation{}, cerr
		}
		return analysisrun.Explanation{}, fail(analysisrun.FallbackInvalidResponse, errors.New("the response could not be read"))
	}
	if len(data) > maxResponseBytes {
		return analysisrun.Explanation{}, fail(analysisrun.FallbackInvalidResponse, errors.New("the response is too large"))
	}
	var chat chatResponse
	if err := json.Unmarshal(data, &chat); err != nil || !chat.Done || strings.TrimSpace(chat.Message.Content) == "" {
		return analysisrun.Explanation{}, fail(analysisrun.FallbackInvalidResponse, errors.New("the response is not a complete chat reply"))
	}
	text, err := parseText(chat.Message.Content)
	if err != nil {
		return analysisrun.Explanation{}, fail(analysisrun.FallbackInvalidResponse, err)
	}
	if err := validate(text, in, evidence); err != nil {
		return analysisrun.Explanation{}, fail(analysisrun.FallbackValidationFailed, err)
	}
	model := o.model
	return analysisrun.Explanation{Text: text, Source: analysisrun.SourceOllama, Model: &model, PromptVersion: PromptVersion}, nil
}

func fail(code string, err error) error { return &analysisrun.ExplanationError{Code: code, Err: err} }

// contextFailure reports a call stopped by its context: a deadline (this
// call's timeout or the stage budget) is a timeout; a cancellation means the
// run itself is stopping, and the error keeps context.Canceled.
func contextFailure(ctx context.Context) error {
	switch err := ctx.Err(); {
	case errors.Is(err, context.DeadlineExceeded):
		return fail(analysisrun.FallbackTimeout, err)
	case err != nil:
		return fail(analysisrun.FallbackProviderError, err)
	default:
		return nil
	}
}
