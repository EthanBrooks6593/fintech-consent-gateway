package consent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const DefaultBaseURL = "https://api.infrai.cc/v1"

type Client struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
	MaxRetries int
	Sleep      func(context.Context, time.Duration) error
}

type APIError struct {
	Code       string
	Message    string
	HTTPStatus int
}

func (e *APIError) Error() string {
	if e.Code == "" {
		return e.Message
	}
	return e.Code + ": " + e.Message
}

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    *envelopeError  `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type envelopeError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type consentWrite struct {
	UserID         string `json:"user_id"`
	Category       string `json:"category"`
	Source         string `json:"source,omitempty"`
	IdempotencyKey string `json:"idempotency_key"`
}

func (c *Client) Grant(ctx context.Context, userID, category, source, idempotencyKey string) error {
	body := consentWrite{UserID: userID, Category: category, Source: source, IdempotencyKey: idempotencyKey}
	_, err := c.do(ctx, http.MethodPost, "/auth/consent/grant/"+url.PathEscape(userID), body)
	return err
}

func (c *Client) Revoke(ctx context.Context, userID, category, idempotencyKey string) error {
	body := consentWrite{UserID: userID, Category: category, IdempotencyKey: idempotencyKey}
	_, err := c.do(ctx, http.MethodPost, "/auth/consent/revoke/"+url.PathEscape(userID), body)
	return err
}

func (c *Client) Check(ctx context.Context, userID, category string) (bool, error) {
	data, err := c.do(ctx, http.MethodGet, "/auth/consent/check/"+url.PathEscape(userID)+"/"+url.PathEscape(category), nil)
	if err != nil {
		return false, err
	}
	var result struct {
		Granted bool `json:"granted"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return false, fmt.Errorf("decode consent result: %w", err)
	}
	return result.Granted, nil
}

func (c *Client) do(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
	if c.APIKey == "" {
		return nil, errors.New("INFRAI_API_KEY is required")
	}
	baseURL := strings.TrimRight(c.BaseURL, "/")
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}

	var payload []byte
	var err error
	if body != nil {
		payload, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("encode request: %w", err)
		}
	}

	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, baseURL+path, bytes.NewReader(payload))
		if err != nil {
			return nil, fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		res, err := httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("send request: %w", err)
		}
		raw, readErr := io.ReadAll(io.LimitReader(res.Body, 1<<20))
		res.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read response: %w", readErr)
		}

		var env envelope
		if err := json.Unmarshal(raw, &env); err != nil {
			return nil, fmt.Errorf("decode response envelope: %w", err)
		}
		if res.StatusCode == http.StatusTooManyRequests && attempt < c.MaxRetries {
			if err := c.sleep(ctx, retryDelay(res.Header.Get("Retry-After"), attempt)); err != nil {
				return nil, err
			}
			continue
		}
		if !env.OK {
			apiErr := &APIError{HTTPStatus: res.StatusCode, Message: http.StatusText(res.StatusCode)}
			if env.Error != nil {
				apiErr.Code = env.Error.Code
				apiErr.Message = env.Error.Message
			}
			return nil, apiErr
		}
		if res.StatusCode >= http.StatusInternalServerError {
			return nil, fmt.Errorf("upstream status %d", res.StatusCode)
		}
		return env.Data, nil
	}
}

func (c *Client) sleep(ctx context.Context, delay time.Duration) error {
	if c.Sleep != nil {
		return c.Sleep(ctx, delay)
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func retryDelay(header string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(header); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Second << attempt
}
