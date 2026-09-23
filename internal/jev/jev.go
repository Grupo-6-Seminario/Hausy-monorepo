// Package jev is the Vercel AI Gateway transport for typesafe-ai/jev: typed
// questions about a piece of state, typed answers back. Callers own the
// questions; this package owns HTTP, retries and error hygiene.
package jev

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const GatewayURL = "https://ai-gateway.vercel.sh"

// Question is one typed question. Criteria is a map of option -> description
// for "choice", a true/false map for "boolean", an ordered list for "score".
type Question struct {
	Type         string `json:"type"`
	Instructions string `json:"instructions"`
	Criteria     any    `json:"criteria,omitempty"`
}

type Answer struct {
	Type          string             `json:"type"`
	Choice        string             `json:"choice,omitempty"`
	Probability   float64            `json:"probability,omitempty"`
	Probabilities map[string]float64 `json:"probabilities,omitempty"`
	Score         float64            `json:"score,omitempty"`
}

// Evaluator is the call callers depend on, so tests can fake Jev without HTTP;
// (*Client).Evaluate fits.
type Evaluator func(ctx context.Context, state any, questions map[string]Question) (map[string]Answer, error)

type Client struct {
	url, key string
	http     *http.Client
	// Retries and Backoff bound transient-failure retries (429, 5xx,
	// transport): attempt n waits Backoff << (n-1). The caller's context
	// deadline always wins.
	Retries int
	Backoff time.Duration
}

// New disables redirects so credentials are never forwarded elsewhere.
func New(baseURL, key string, client *http.Client) *Client {
	if baseURL == "" {
		baseURL = GatewayURL
	}
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	copy := *client
	copy.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &Client{url: strings.TrimRight(baseURL, "/"), key: key, http: &copy, Retries: 3, Backoff: 500 * time.Millisecond}
}

// transient marks failures worth another attempt.
type transient struct{ error }

// Evaluate asks every question about state. It returns an error unless every
// question has an answer.
func (c *Client) Evaluate(ctx context.Context, state any, questions map[string]Question) (map[string]Answer, error) {
	if c.key == "" {
		return nil, errors.New("jev: AI_GATEWAY_API_KEY is required")
	}
	payload, err := json.Marshal(map[string]any{"model": "typesafe-ai/jev", "state": state, "questions": questions})
	if err != nil {
		return nil, fmt.Errorf("jev: encode request: %w", err)
	}
	for attempt := 0; ; attempt++ {
		answers, err := c.once(ctx, payload, len(questions))
		var retry transient
		if err == nil || !errors.As(err, &retry) || attempt >= c.Retries {
			return answers, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(c.Backoff << attempt):
		}
	}
}

func (c *Client) once(ctx context.Context, payload []byte, want int) (map[string]Answer, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url+"/v1/evaluate", bytes.NewReader(payload))
	if err != nil {
		return nil, errors.New("jev: invalid endpoint")
	}
	req.Header.Set("Authorization", "Bearer "+c.key)
	req.Header.Set("Content-Type", "application/json")
	response, err := c.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		// url.Error wraps the cause (dial, TLS, EOF); the inner error never holds headers.
		return nil, transient{fmt.Errorf("jev: gateway transport failed: %v", errors.Unwrap(err))}
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		err := gatewayError(response)
		if response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= 500 {
			return nil, transient{err}
		}
		return nil, err
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		return nil, transient{errors.New("jev: response read failed")}
	}
	var body struct {
		Answers map[string]Answer `json:"answers"`
	}
	if err := json.Unmarshal(data, &body); err != nil {
		return nil, errors.New("jev: invalid response JSON")
	}
	if len(body.Answers) != want {
		return nil, fmt.Errorf("jev: got %d answers for %d questions", len(body.Answers), want)
	}
	return body.Answers, nil
}

// errorType matches Gateway's machine-readable codes, e.g. customer_verification_required.
var errorType = regexp.MustCompile(`^[a-z0-9_.-]{1,64}$`)

// gatewayError reports status, error type and request ID. The error message is
// free text that can echo request content, so it is never included.
func gatewayError(response *http.Response) error {
	var body struct {
		Error struct {
			Type string `json:"type"`
		} `json:"error"`
	}
	data, _ := io.ReadAll(io.LimitReader(response.Body, 64<<10))
	msg := fmt.Sprintf("jev: gateway HTTP %d", response.StatusCode)
	if json.Unmarshal(data, &body) == nil && errorType.MatchString(body.Error.Type) {
		msg += " " + body.Error.Type
	}
	if id := response.Header.Get("X-Vercel-Id"); id != "" {
		msg += fmt.Sprintf(" (request %q)", id)
	}
	return errors.New(msg)
}
