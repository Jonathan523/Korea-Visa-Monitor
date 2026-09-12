package store

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Jonathan523/Korea-Visa-Monitor/internal/config"
	"github.com/Jonathan523/Korea-Visa-Monitor/internal/model"
)

type upstashStore struct {
	url, token, key string
	client          *http.Client
}

func newUpstashStore(cfg config.Config) *upstashStore {
	return &upstashStore{url: cfg.UpstashURL, token: cfg.UpstashToken, key: cfg.UpstashKey, client: &http.Client{Timeout: 20 * time.Second}}
}

func (s *upstashStore) Load(ctx context.Context) (*model.State, error) {
	var raw *string
	if err := s.command(ctx, []string{"GET", s.key}, &raw); err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, nil
	}
	var state model.State
	if err := json.Unmarshal([]byte(*raw), &state); err != nil {
		// Match the previous behavior: malformed stored data is treated as no state.
		return nil, nil
	}
	return &state, nil
}

func (s *upstashStore) Save(ctx context.Context, state model.State) error {
	value, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("编码状态失败: %w", err)
	}
	var result string
	if err := s.command(ctx, []string{"SET", s.key, string(value)}, &result); err != nil {
		return err
	}
	if result != "OK" {
		return fmt.Errorf("Upstash 写入状态失败: 返回值为 %q", result)
	}
	return nil
}

func (s *upstashStore) command(ctx context.Context, command []string, target any) error {
	body, err := json.Marshal(command)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("访问 Upstash 状态存储失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("访问 Upstash 状态存储失败: HTTP %s", resp.Status)
	}
	var envelope struct {
		Result json.RawMessage `json:"result"`
		Error  string          `json:"error"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&envelope); err != nil {
		return fmt.Errorf("解析 Upstash 响应失败: %w", err)
	}
	if envelope.Error != "" {
		return fmt.Errorf("Upstash 返回错误: %s", envelope.Error)
	}
	if envelope.Result == nil {
		return fmt.Errorf("Upstash 返回了无法识别的响应")
	}
	if err := json.Unmarshal(envelope.Result, target); err != nil {
		return fmt.Errorf("解析 Upstash 结果失败: %w", err)
	}
	return nil
}
