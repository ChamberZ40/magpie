package feishu

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	lark "github.com/larksuite/oapi-sdk-go/v3"
)

func newChatNameTestPlatform(t *testing.T, chatBody map[string]any, status int) (*Platform, *int32) {
	t.Helper()
	var chatCalls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/open-apis/auth/v3/tenant_access_token/internal":
			writeJSON(t, w, map[string]any{"code": 0, "msg": "success", "expire": 7200, "tenant_access_token": "tenant-token"})
		case strings.HasPrefix(r.URL.Path, "/open-apis/im/v1/chats/"):
			atomic.AddInt32(&chatCalls, 1)
			w.WriteHeader(status)
			writeJSON(t, w, chatBody)
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	t.Cleanup(srv.Close)
	p := &Platform{
		platformName: "feishu",
		client: lark.NewClient("cli_chat_name", "secret-chat-name",
			lark.WithOpenBaseUrl(srv.URL),
			lark.WithHttpClient(srv.Client()),
		),
	}
	return p, &chatCalls
}

// A p2p chat answers Chat.Get with no name. That answer never changes, so
// asking again on every inbound message only adds a round trip in front of
// each turn.
func TestResolveChatNameCachesAChatWithNoName(t *testing.T) {
	p, calls := newChatNameTestPlatform(t, map[string]any{"code": 0, "msg": "success", "data": map[string]any{}}, http.StatusOK)

	for i := 0; i < 3; i++ {
		if got := p.resolveChatName("oc_p2p"); got != "oc_p2p" {
			t.Fatalf("resolveChatName = %q, want the chat id back", got)
		}
	}
	if n := atomic.LoadInt32(calls); n != 1 {
		t.Errorf("Chat.Get called %d times, want 1 — the nameless answer was not cached", n)
	}
}

// A request that failed outright says nothing about the chat, so it is not
// remembered: the next message tries again.
func TestResolveChatNameRetriesAfterAFailedRequest(t *testing.T) {
	p, calls := newChatNameTestPlatform(t, map[string]any{"code": 99991663, "msg": "internal error"}, http.StatusInternalServerError)

	p.resolveChatName("oc_group")
	p.resolveChatName("oc_group")
	if n := atomic.LoadInt32(calls); n < 2 {
		t.Errorf("Chat.Get called %d times, want a retry after a failed request", n)
	}
}
