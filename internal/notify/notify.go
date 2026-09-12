package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Jonathan523/Korea-Visa-Monitor/internal/config"
)

type Sender interface {
	Send(context.Context, string, string) error
}

func New(cfg config.Config) Sender {
	client := &http.Client{Timeout: 20 * time.Second}
	if cfg.PushChannel == "serverchan" {
		return &serverChan{key: cfg.ServerChanKey, client: client}
	}
	return &pushDeer{key: cfg.PushDeerKey, endpoint: cfg.PushDeerEndpoint, client: client}
}

type pushDeer struct {
	key, endpoint string
	client        *http.Client
}

func (p *pushDeer) Send(ctx context.Context, title, body string) error {
	form := url.Values{"pushkey": {p.key}, "text": {title}, "desp": {body}, "type": {"markdown"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	var payload struct {
		Code int             `json:"code"`
		Data json.RawMessage `json:"data"`
	}
	if err := doJSON(p.client, req, &payload); err != nil {
		return fmt.Errorf("PushDeer 推送失败: %w", err)
	}
	if payload.Code != 0 {
		return fmt.Errorf("PushDeer 返回错误代码 %d", payload.Code)
	}
	return nil
}

type serverChan struct {
	key    string
	client *http.Client
}

func (s *serverChan) Send(ctx context.Context, title, body string) error {
	endpoint := "https://sctapi.ftqq.com/" + url.PathEscape(s.key) + ".send"
	form := url.Values{"title": {title}, "desp": {body}, "tags": {"签证监控|图片"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	var payload struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := doJSON(s.client, req, &payload); err != nil {
		return fmt.Errorf("Server酱推送失败: %w", err)
	}
	if payload.Code != 0 {
		return fmt.Errorf("Server酱返回错误代码 %d: %s", payload.Code, payload.Message)
	}
	return nil
}

func doJSON(client *http.Client, req *http.Request, target any) error {
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("HTTP %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(target); err != nil {
		return fmt.Errorf("响应不是有效 JSON: %w", err)
	}
	return nil
}
