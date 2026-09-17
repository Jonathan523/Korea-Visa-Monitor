package notify

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/url"
	"time"

	"golang.org/x/net/websocket"
)

type newmsg struct {
	key, endpoint string
}

func (n *newmsg) Send(ctx context.Context, title, body string) error {
	endpoint, err := url.Parse(n.endpoint)
	if err != nil {
		return fmt.Errorf("newmsg 地址无效: %w", err)
	}
	originScheme := "https"
	if endpoint.Scheme == "ws" {
		originScheme = "http"
	}
	config, err := websocket.NewConfig(n.endpoint, originScheme+"://"+endpoint.Host+"/")
	if err != nil {
		return fmt.Errorf("newmsg 地址无效: %w", err)
	}
	config.Header.Set("X-API-Key", n.key)

	dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	conn, err := config.DialContext(dialCtx)
	if err != nil {
		return fmt.Errorf("newmsg 连接失败: %w", err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return fmt.Errorf("newmsg 设置超时失败: %w", err)
	}
	if err := websocket.JSON.Send(conn, struct {
		Type    string `json:"type"`
		APIKey  string `json:"apiKey"`
		Version string `json:"version"`
	}{"auth", n.key, "2.0"}); err != nil {
		return fmt.Errorf("newmsg 认证请求失败: %w", err)
	}
	authenticated := false
	for !authenticated {
		var response struct {
			Type string `json:"type"`
		}
		if err := websocket.JSON.Receive(conn, &response); err != nil {
			return fmt.Errorf("newmsg 认证响应失败: %w", err)
		}
		switch response.Type {
		case "auth_ok":
			authenticated = true
		case "auth_failed", "error":
			return fmt.Errorf("newmsg 认证失败或服务端返回错误")
		}
	}
	var suffix [5]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return fmt.Errorf("newmsg 生成消息 ID 失败: %w", err)
	}
	content := title
	if body != "" {
		content += "\n" + body
	}
	message := struct {
		Type      string `json:"type"`
		APIKey    string `json:"apiKey"`
		To        string `json:"to"`
		Content   string `json:"content"`
		MessageID string `json:"messageId"`
	}{"send", n.key, n.key, content, fmt.Sprintf("msg_%d_%x", time.Now().UnixMilli(), suffix)}
	if err := websocket.JSON.Send(conn, message); err != nil {
		return fmt.Errorf("newmsg 发送失败: %w", err)
	}
	return nil
}
