package notify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPushDeerSendsMarkdownForm(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if r.Form.Get("pushkey") != "key" || r.Form.Get("text") != "title" || r.Form.Get("desp") != "body" || r.Form.Get("type") != "markdown" {
			t.Errorf("unexpected form: %#v", r.Form)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 0})
	}))
	defer server.Close()

	sender := &pushDeer{key: "key", endpoint: server.URL, client: server.Client()}
	if err := sender.Send(context.Background(), "title", "body"); err != nil {
		t.Fatal(err)
	}
}

func TestPushDeerRejectsAPIErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 1001})
	}))
	defer server.Close()
	sender := &pushDeer{key: "key", endpoint: server.URL, client: server.Client()}
	if err := sender.Send(context.Background(), "title", "body"); err == nil {
		t.Fatal("expected API error")
	}
}
