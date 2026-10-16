// Package bungie is a typed client for the Bungie.net Platform API.
package bungie

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"strings"
	"time"
)

// maxThrottleWait caps how long a Bungie-requested throttle delay can block a call.
const maxThrottleWait = 60 * time.Second

// Limiter paces requests across every caller of a Client. *ratelimit.Limiter implements it.
type Limiter interface {
	Wait(ctx context.Context) error
	PauseUntil(t time.Time)
}

type Client struct {
	apiKey         string
	baseURL        string
	http           *http.Client
	requestTimeout time.Duration
	fileTimeout    time.Duration
	maxRetries     int
	baseBackoff    time.Duration
	sleep          func(context.Context, time.Duration) error
	limiter        Limiter
	now            func() time.Time
}

func NewClient(apiKey, baseURL string, hc *http.Client) *Client {
	if hc == nil {
		hc = &http.Client{}
	}
	return &Client{
		apiKey:         apiKey,
		baseURL:        strings.TrimRight(baseURL, "/"),
		http:           hc,
		requestTimeout: 15 * time.Second,
		fileTimeout:    2 * time.Minute,
		maxRetries:     3,
		baseBackoff:    500 * time.Millisecond,
		sleep:          sleepCtx,
		now:            time.Now,
	}
}

// SetLimiter makes every attempt (including retries) wait for l. Bungie throttle responses
// pause l for everyone instead of sleeping only this caller.
func (c *Client) SetLimiter(l Limiter) { c.limiter = l }

type envelope struct {
	Response        json.RawMessage `json:"Response"`
	ErrorCode       int             `json:"ErrorCode"`
	ErrorStatus     string          `json:"ErrorStatus"`
	Message         string          `json:"Message"`
	ThrottleSeconds int             `json:"ThrottleSeconds"`
}

// call performs a Platform request and decodes the envelope's Response into T.
func call[T any](ctx context.Context, c *Client, method, path string, body any) (T, error) {
	var out T
	raw, err := c.callRaw(ctx, method, path, body)
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return out, fmt.Errorf("bungie: decode %s response: %w", path, err)
	}
	return out, nil
}

// callRaw performs a Platform request with retries and returns the raw Response JSON.
func (c *Client) callRaw(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
	var payload []byte
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("bungie: marshal %s body: %w", path, err)
		}
		payload = b
	}
	for attempt := 0; ; attempt++ {
		if c.limiter != nil {
			if err := c.limiter.Wait(ctx); err != nil {
				return nil, err
			}
		}
		raw, err := c.callOnce(ctx, method, path, payload)
		if err == nil {
			return raw, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		wait, retry := c.retryDelay(err, attempt)
		if !retry {
			return nil, err
		}
		if c.limiter != nil && IsThrottled(err) {
			// Pause every caller; the next Wait absorbs the delay.
			c.limiter.PauseUntil(c.now().Add(wait))
			continue
		}
		if err := c.sleep(ctx, wait); err != nil {
			return nil, err
		}
	}
}

func (c *Client) callOnce(ctx context.Context, method, path string, payload []byte) (json.RawMessage, error) {
	ctx, cancel := context.WithTimeout(ctx, c.requestTimeout)
	defer cancel()
	var body io.Reader
	if payload != nil {
		body = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("bungie: build request %s: %w", path, err)
	}
	req.Header.Set("X-API-Key", c.apiKey)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, &TransportError{Path: path, Err: err}
	}
	defer func() {
		// Drain so the keep-alive connection can be reused.
		io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
		resp.Body.Close()
	}()

	var env envelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return nil, &APIError{HTTPStatus: resp.StatusCode, Message: fmt.Sprintf("decode %s: %v", path, err)}
	}
	if env.ErrorCode != CodeSuccess {
		msg := env.Message
		if env.ErrorCode == 0 && msg == "" {
			msg = "missing envelope"
		}
		return nil, &APIError{
			Code:            env.ErrorCode,
			Status:          env.ErrorStatus,
			Message:         msg,
			ThrottleSeconds: env.ThrottleSeconds,
			HTTPStatus:      resp.StatusCode,
		}
	}
	return env.Response, nil
}

func (c *Client) retryDelay(err error, attempt int) (time.Duration, bool) {
	if attempt >= c.maxRetries {
		return 0, false
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		switch {
		case apiErr.Code == CodeSystemDisabled:
			return 0, false
		case apiErr.ThrottleSeconds > 0:
			return min(time.Duration(apiErr.ThrottleSeconds)*time.Second, maxThrottleWait), true
		case apiErr.Code == 0 && apiErr.HTTPStatus == http.StatusTooManyRequests:
			return c.backoff(attempt), true
		case apiErr.HTTPStatus >= 500 && apiErr.Code == 0:
			return c.backoff(attempt), true
		default:
			return 0, false
		}
	}
	var tErr *TransportError
	if errors.As(err, &tErr) {
		return c.backoff(attempt), true
	}
	return 0, false
}

// backoff returns base*2^attempt plus up to 50% jitter.
func (c *Client) backoff(attempt int) time.Duration {
	d := c.baseBackoff << attempt
	return d + time.Duration(rand.Int64N(int64(d)/2+1))
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
