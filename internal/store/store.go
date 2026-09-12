package store

import (
	"context"
	"fmt"

	"github.com/Jonathan523/Korea-Visa-Monitor/internal/config"
	"github.com/Jonathan523/Korea-Visa-Monitor/internal/model"
)

type Store interface {
	Load(context.Context) (*model.State, error)
	Save(context.Context, model.State) error
}

func New(ctx context.Context, cfg config.Config) (Store, error) {
	switch cfg.StateStorage {
	case "local":
		return &localStore{path: cfg.StateFile}, nil
	case "upstash":
		return newUpstashStore(cfg), nil
	case "s3":
		return newS3Store(ctx, cfg)
	default:
		return nil, fmt.Errorf("不支持的状态存储 %q", cfg.StateStorage)
	}
}
