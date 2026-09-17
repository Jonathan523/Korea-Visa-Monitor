package notify

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/websocket"
)

func TestNewmsgSend(t *testing.T) {
	type message struct {
		Type      string `json:"type"`
		APIKey    string `json:"apiKey"`
		Version   string `json:"version"`
		To        string `json:"to"`
		Content   string `json:"content"`
		MessageID string `json:"messageId"`
	}
	received := make(chan message, 1)
	server := httptest.NewServer(websocket.Handler(func(ws *websocket.Conn) {
		if ws.Request().Header.Get("X-API-Key") != "ak_test" {
			return
		}
		var auth message
		if err := websocket.JSON.Receive(ws, &auth); err != nil || auth.Type != "auth" || auth.APIKey != "ak_test" || auth.Version != "2.0" {
			return
		}
		_ = websocket.JSON.Send(ws, map[string]string{"type": "connected"})
		_ = websocket.JSON.Send(ws, map[string]string{"type": "auth_ok"})
		var sent message
		if err := websocket.JSON.Receive(ws, &sent); err == nil {
			received <- sent
		}
	}))
	defer server.Close()

	sender := &newmsg{key: "ak_test", endpoint: "ws" + strings.TrimPrefix(server.URL, "http")}
	if err := sender.Send(context.Background(), "状态变化", "当前状态：签发"); err != nil {
		t.Fatal(err)
	}
	select {
	case sent := <-received:
		if sent.Type != "send" || sent.APIKey != "ak_test" || sent.To != "ak_test" || sent.Content != "状态变化\n当前状态：签发" || sent.MessageID == "" {
			t.Fatalf("unexpected send payload: %+v", sent)
		}
	case <-time.After(time.Second):
		t.Fatal("server did not receive send payload")
	}
}

func TestNewmsgAuthFailure(t *testing.T) {
	server := httptest.NewServer(websocket.Handler(func(ws *websocket.Conn) {
		var auth json.RawMessage
		_ = websocket.JSON.Receive(ws, &auth)
		_ = websocket.JSON.Send(ws, map[string]string{"type": "auth_failed"})
	}))
	defer server.Close()

	sender := &newmsg{key: "ak_test", endpoint: "ws" + strings.TrimPrefix(server.URL, "http")}
	if err := sender.Send(context.Background(), "test", ""); err == nil || !strings.Contains(err.Error(), "认证失败") {
		t.Fatalf("expected auth failure, got %v", err)
	}
}
