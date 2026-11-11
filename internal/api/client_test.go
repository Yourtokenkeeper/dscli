package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The completion endpoint streams frames; a frame without "p" continues the
// previous path and the first snapshot seeds the answer. Missing that seed is
// the bug that silently eats the opening characters of a reply.
func TestConsumeSSESeedAndDeltas(t *testing.T) {
	body := strings.Join([]string{
		`event: ready`,
		`data: {"request_message_id":1,"response_message_id":2}`,
		``,
		`data: {"v":{"response":{"fragments":[{"type":"RESPONSE","content":"收到"}]}}}`,
		``,
		`data: {"p":"response/fragments/-1/content","o":"APPEND","v":"，明白"}`,
		``,
		`data: {"v":"了"}`,
		``,
		`data: {"p":"response","o":"BATCH","v":[{"p":"accumulated_token_usage","v":12}]}`,
		``,
	}, "\n")

	var got strings.Builder
	c := New("token", "")
	err := c.consumeSSE(strings.NewReader(body), AskOptions{OnText: func(s string) { got.WriteString(s) }})
	if err != nil {
		t.Fatalf("consumeSSE: %v", err)
	}
	if want := "收到，明白了"; got.String() != want {
		t.Fatalf("got %q want %q", got.String(), want)
	}
}

func TestMessageTextAndThinking(t *testing.T) {
	m := Message{Fragments: []Fragment{
		{Type: "THINK", Content: "想一想"},
		{Type: "RESPONSE", Content: "答案"},
	}}
	if m.Text() != "答案" {
		t.Fatalf("Text() = %q", m.Text())
	}
	if m.Thinking() != "想一想" {
		t.Fatalf("Thinking() = %q", m.Thinking())
	}
}

// Envelope handling: a non-zero code must surface as an error, not silence.
func TestEnvelopeErrorSurfaces(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		w.Write([]byte(`{"code":40005,"msg":"CLIENT_VERSION_TOO_LOW","data":null}`))
	}))
	defer srv.Close()
	old := BaseURL
	BaseURL = srv.URL
	defer func() { BaseURL = old }()

	c := New("token", "")
	if _, err := c.Whoami(context.Background()); err == nil {
		t.Fatal("expected error for code!=0")
	} else if !strings.Contains(err.Error(), "CLIENT_VERSION_TOO_LOW") {
		t.Fatalf("error should carry server message, got %v", err)
	}
}
