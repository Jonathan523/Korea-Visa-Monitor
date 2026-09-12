package visa

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseFoundResult(t *testing.T) {
	html := `<html><div id="result3_2">
		<span id="ONLINE_APPL_NO"> AB-123 </span>
		<span id="ENTRY_PURPOSE"> 短期 <b>访问</b> </span>
		<span id="PROC_STS_CDNM_1">签发 (2026.09.12.)</span>
	</div></html>`
	result, err := Parse(strings.NewReader(html))
	if err != nil {
		t.Fatal(err)
	}
	if !result.Found || result.ApplicationNumber != "AB-123" || result.EntryPurpose != "短期 访问" || !result.Issued() {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestParseMissingResult(t *testing.T) {
	result, err := Parse(strings.NewReader(`<div id="result3_2"><span>empty</span></div>`))
	if err != nil {
		t.Fatal(err)
	}
	if result.Found || result.Message != "未查询到签证申请记录" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestClientUsesSessionAndExpectedForm(t *testing.T) {
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method == http.MethodGet {
			http.SetCookie(w, &http.Cookie{Name: "JSESSIONID", Value: "session"})
			fmt.Fprint(w, "ready")
			return
		}
		cookie, err := r.Cookie("JSESSIONID")
		if err != nil || cookie.Value != "session" {
			t.Errorf("session cookie not retained: %v", err)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if got := r.Form.Get("sEK_NM"); got != "ZHANG SAN" {
			t.Errorf("name = %q", got)
		}
		if got := r.Form.Get("pRADIOSEARCH"); got != "gb03" {
			t.Errorf("search type = %q", got)
		}
		fmt.Fprint(w, `<div id="result3_2"><i id="ONLINE_APPL_NO">A1</i><i id="ENTRY_PURPOSE">Tour</i><i id="PROC_STS_CDNM_1">审查中</i></div>`)
	}))
	defer server.Close()

	result, err := NewClient(server.URL).Query(context.Background(), "P123", "zhang san", "1990-01-31")
	if err != nil {
		t.Fatal(err)
	}
	if requests != 2 || !result.Found || result.Status != "审查中" {
		t.Fatalf("requests=%d result=%#v", requests, result)
	}
}
