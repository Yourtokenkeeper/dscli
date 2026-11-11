// Package api speaks the chat.deepseek.com endpoints the mobile and web
// clients share. It is not the public platform API (api.deepseek.com): calls
// bill the signed-in account exactly like the app does.
package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/yg/dscli/internal/pow"
)

// BaseURL is overridable for tests and mirrors.
var BaseURL = "https://chat.deepseek.com"

const userAgent = "Mozilla/5.0 (Linux; Android 16; K) AppleWebKit/537.36 " +
	"(KHTML, like Gecko) Chrome/126.0.0.0 Mobile Safari/537.36"

// Client is a thin, dependency-free wrapper around the official endpoints.
type Client struct {
	Token    string
	DeviceID string
	HTTP     *http.Client
	// ThinkingVisible also prints the model's reasoning stream.
	ThinkingVisible bool
}

// New builds a client with sane timeouts.
func New(token, deviceID string) *Client {
	return &Client{
		Token:    token,
		DeviceID: deviceID,
		HTTP: &http.Client{
			Timeout: 0, // streaming requests manage their own deadline
			Transport: &http.Transport{
				Proxy:                 http.ProxyFromEnvironment,
				MaxIdleConns:          16,
				IdleConnTimeout:       90 * time.Second,
				TLSHandshakeTimeout:   15 * time.Second,
				ResponseHeaderTimeout: 60 * time.Second,
			},
		},
	}
}

func (c *Client) headers(extra map[string]string) map[string]string {
	h := map[string]string{
		"accept":                   "*/*",
		"content-type":             "application/json",
		"user-agent":               userAgent,
		"x-client-platform":        "web",
		"x-client-version":         "1.0.0-always",
		"x-app-version":            "20241129.1",
		"x-client-locale":          "zh_CN",
		"x-client-timezone-offset": "28800",
		"authorization":            "Bearer " + c.Token,
	}
	if c.DeviceID != "" {
		h["x-device-id"] = c.DeviceID
	}
	for k, v := range extra {
		h[k] = v
	}
	return h
}

// envelope is the standard {code,msg,data} wrapper.
type envelope struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

type bizEnvelope struct {
	BizCode int             `json:"biz_code"`
	BizMsg  string          `json:"biz_msg"`
	BizData json.RawMessage `json:"biz_data"`
}

func (c *Client) do(ctx context.Context, method, path string, body any, extra map[string]string, out any) error {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, BaseURL+path, reader)
	if err != nil {
		return err
	}
	for k, v := range c.headers(extra) {
		req.Header.Set(k, v)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("%s %s: HTTP %d %s", method, path, resp.StatusCode, strings.TrimSpace(string(raw[:min(len(raw), 200)])))
	}
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("%s %s: bad envelope: %w", method, path, err)
	}
	if env.Code != 0 {
		return fmt.Errorf("%s %s: code=%d msg=%s", method, path, env.Code, env.Msg)
	}
	var biz bizEnvelope
	if err := json.Unmarshal(env.Data, &biz); err != nil {
		// Some endpoints return the payload directly under data.
		if out != nil {
			return json.Unmarshal(env.Data, out)
		}
		return nil
	}
	if biz.BizCode != 0 {
		return fmt.Errorf("%s %s: biz_code=%d biz_msg=%s", method, path, biz.BizCode, biz.BizMsg)
	}
	if out != nil {
		return json.Unmarshal(biz.BizData, out)
	}
	return nil
}

// User is the caller's own account summary.
type User struct {
	ID     string `json:"id"`
	Email  string `json:"email"`
	Mobile string `json:"mobile_number"`
	Status int    `json:"status"`
}

// Whoami returns the signed-in identity.
func (c *Client) Whoami(ctx context.Context) (User, error) {
	var u User
	err := c.do(ctx, http.MethodGet, "/api/v0/users/current", nil, nil, &u)
	return u, err
}

// Session is one chat session row.
type Session struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// Sessions lists recent sessions.
func (c *Client) Sessions(ctx context.Context, count int) ([]Session, error) {
	q := url.Values{"count": {fmt.Sprint(count)}}
	var out struct {
		ChatSessions []Session `json:"chat_sessions"`
	}
	err := c.do(ctx, http.MethodGet, "/api/v0/chat_session/fetch_page?"+q.Encode(), nil, nil, &out)
	return out.ChatSessions, err
}

// Fragment is one piece of a message.
type Fragment struct {
	Type    string `json:"type"`
	Content string `json:"content"`
}

// Message is one row of a session history.
type Message struct {
	MessageID  int64      `json:"message_id"`
	Role       string     `json:"role"`
	Fragments  []Fragment `json:"fragments"`
	Content    string     `json:"content"`
	InsertedAt float64    `json:"inserted_at"`
}

// Text joins the visible fragments (falls back to a flat content field).
func (m Message) Text() string {
	var b strings.Builder
	for _, f := range m.Fragments {
		if f.Type == "RESPONSE" {
			b.WriteString(f.Content)
		}
	}
	if b.Len() == 0 {
		for _, f := range m.Fragments {
			if f.Type != "THINK" {
				b.WriteString(f.Content)
			}
		}
	}
	if b.Len() == 0 {
		return m.Content
	}
	return b.String()
}

// Thinking joins the reasoning fragments.
func (m Message) Thinking() string {
	var b strings.Builder
	for _, f := range m.Fragments {
		if f.Type == "THINK" {
			b.WriteString(f.Content)
		}
	}
	return b.String()
}

// History returns the messages of a session.
func (c *Client) History(ctx context.Context, sessionID string) ([]Message, error) {
	q := url.Values{"chat_session_id": {sessionID}}
	var out struct {
		ChatMessages []Message `json:"chat_messages"`
	}
	err := c.do(ctx, http.MethodGet, "/api/v0/chat/history_messages?"+q.Encode(), nil, nil, &out)
	return out.ChatMessages, err
}

// NewSession creates an empty session and returns its id.
func (c *Client) NewSession(ctx context.Context) (string, error) {
	var out struct {
		ChatSession Session `json:"chat_session"`
		ID          string  `json:"id"`
	}
	if err := c.do(ctx, http.MethodPost, "/api/v0/chat_session/create", map[string]any{}, nil, &out); err != nil {
		return "", err
	}
	if out.ChatSession.ID != "" {
		return out.ChatSession.ID, nil
	}
	return out.ID, nil
}

// powHeader fetches a challenge and solves it, returning the base64 header the
// server expects in X-DS-PoW-Response.
func (c *Client) powHeader(ctx context.Context, target string) (string, time.Duration, error) {
	var out struct {
		Challenge pow.Challenge `json:"challenge"`
	}
	if err := c.do(ctx, http.MethodPost, "/api/v0/chat/create_pow_challenge",
		map[string]string{"target_path": target}, nil, &out); err != nil {
		return "", 0, err
	}
	start := time.Now()
	answer, err := pow.Solve(ctx, out.Challenge)
	if err != nil {
		return "", 0, err
	}
	payload := map[string]any{
		"algorithm":   out.Challenge.Algorithm,
		"challenge":   out.Challenge.Challenge,
		"salt":        out.Challenge.Salt,
		"answer":      answer,
		"signature":   out.Challenge.Signature,
		"target_path": target,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return "", 0, err
	}
	return base64.StdEncoding.EncodeToString(b), time.Since(start), nil
}

// AskOptions tunes one completion call.
type AskOptions struct {
	SessionID string
	Thinking  bool
	Search    bool
	ModelType string
	// OnText receives incremental answer text; OnThinking incremental reasoning.
	OnText     func(string)
	OnThinking func(string)
}

// Ask sends a prompt and streams the answer back through the callbacks.
func (c *Client) Ask(ctx context.Context, prompt string, opt AskOptions) (sessionID string, err error) {
	sessionID = opt.SessionID
	if sessionID == "" {
		sessionID, err = c.NewSession(ctx)
		if err != nil {
			return "", fmt.Errorf("create session: %w", err)
		}
	}
	hdr, _, err := c.powHeader(ctx, "/api/v0/chat/completion")
	if err != nil {
		return sessionID, fmt.Errorf("pow: %w", err)
	}
	modelType := opt.ModelType
	if modelType == "" {
		modelType = "default"
		if opt.Thinking {
			modelType = "expert"
		}
	}
	body := map[string]any{
		"chat_session_id":   sessionID,
		"parent_message_id": nil,
		"model_type":        modelType,
		"prompt":            prompt,
		"ref_file_ids":      []string{},
		"thinking_enabled":  opt.Thinking,
		"search_enabled":    opt.Search,
		"source":            "default",
		"action":            nil,
		"preempt":           false,
	}
	b, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, BaseURL+"/api/v0/chat/completion", bytes.NewReader(b))
	if err != nil {
		return sessionID, err
	}
	for k, v := range c.headers(map[string]string{"x-ds-pow-response": hdr}) {
		req.Header.Set(k, v)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return sessionID, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return sessionID, fmt.Errorf("completion: HTTP %d %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	return sessionID, c.consumeSSE(resp.Body, opt)
}

// consumeSSE decodes the delta protocol: {"p":path,"o":op,"v":value} frames,
// where a frame without "p" continues the previous path, and o=BATCH carries
// a nested list of frames.
func (c *Client) consumeSSE(r io.Reader, opt AskOptions) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 4<<20)
	cur := "response/fragments/-1/content"

	var handle func(frame map[string]any)
	handle = func(f map[string]any) {
		if op, _ := f["o"].(string); op == "BATCH" {
			if list, ok := f["v"].([]any); ok {
				for _, item := range list {
					if sub, ok := item.(map[string]any); ok {
						handle(sub)
					}
				}
			}
			return
		}
		if p, _ := f["p"].(string); p != "" && p != "response" {
			cur = p
		}
		seed := f["v"]
		if m, ok := seed.(map[string]any); ok {
			if inner, ok := m["response"].(map[string]any); ok {
				seed = inner
			}
		}
		if v, ok := seed.(map[string]any); ok && f["p"] != "response/fragments/-1/content" {
			if frags, ok := v["fragments"].([]any); ok {
				for _, fr := range frags {
					frMap, _ := fr.(map[string]any)
					content, _ := frMap["content"].(string)
					switch frMap["type"] {
					case "RESPONSE":
						if opt.OnText != nil {
							opt.OnText(content)
						}
					case "THINK":
						if opt.OnThinking != nil {
							opt.OnThinking(content)
						}
					}
				}
			}
			return
		}
		s, ok := f["v"].(string)
		if !ok {
			return
		}
		p, _ := f["p"].(string)
		path := p
		if path == "" {
			path = cur
		}
		if strings.Contains(path, "thinking") {
			if opt.OnThinking != nil {
				opt.OnThinking(s)
			}
		} else if strings.Contains(path, "content") {
			if opt.OnText != nil {
				opt.OnText(s)
			}
		}
	}

	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		raw := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if raw == "" || raw == "[DONE]" {
			continue
		}
		var frame map[string]any
		if err := json.Unmarshal([]byte(raw), &frame); err != nil {
			continue
		}
		handle(frame)
	}
	return sc.Err()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
