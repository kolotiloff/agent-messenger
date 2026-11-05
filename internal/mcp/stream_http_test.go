package mcp

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStreamHTTPRequiresBearerAndRejectsModelRouting(t *testing.T) {
	handler := &StreamHandler{Secret: "private-random-key"}
	for _, tc := range []struct {
		auth, origin string
		want         int
	}{{"", "", 401}, {"Bearer wrong", "", 401}, {"Bearer private-random-key", "https://example.com", 401}, {"Bearer private-random-key", "", 200}} {
		request := httptest.NewRequest("POST", "http://localhost/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
		request.Header.Set("Authorization", tc.auth)
		request.Header.Set("Origin", tc.origin)
		result := httptest.NewRecorder()
		handler.ServeHTTP(result, request)
		if result.Code != tc.want {
			t.Fatal(result.Code, tc.want)
		}
	}
	if _, err := handler.tool("reply", map[string]any{"receipt": "r", "text": "hi", "recipient": "other"}); err == nil {
		t.Fatal("caller chose address")
	}
	if _, err := handler.tool("reply", map[string]any{"text": "hi"}); err == nil {
		t.Fatal("missing receipt allowed")
	}
	if _, err := handler.tool("read_current", map[string]any{"id": "other"}); err == nil {
		t.Fatal("caller selected message")
	}
}
