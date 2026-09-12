package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/Jonathan523/Korea-Visa-Monitor/internal/model"
)

type localStore struct{ path string }

func (s *localStore) Load(_ context.Context) (*model.State, error) {
	f, err := os.Open(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("打开状态文件失败: %w", err)
	}
	defer f.Close()
	var state model.State
	if err := json.NewDecoder(io.LimitReader(f, 1<<20)).Decode(&state); err != nil {
		// Keep compatibility with the Python app, which treated corrupt local
		// state as an absent state and recovered on the next successful check.
		return nil, nil
	}
	return &state, nil
}

func (s *localStore) Save(_ context.Context, state model.State) error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("创建状态目录失败: %w", err)
	}
	f, err := os.CreateTemp(dir, ".visa-state-*.tmp")
	if err != nil {
		return fmt.Errorf("创建临时状态文件失败: %w", err)
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(state); err != nil {
		f.Close()
		return fmt.Errorf("写入状态失败: %w", err)
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return fmt.Errorf("同步状态失败: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("关闭状态文件失败: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("替换状态文件失败: %w", err)
	}
	return nil
}
