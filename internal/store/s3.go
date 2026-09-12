package store

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/Jonathan523/Korea-Visa-Monitor/internal/config"
	"github.com/Jonathan523/Korea-Visa-Monitor/internal/model"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

type s3API interface {
	GetObject(context.Context, *s3.GetObjectInput, ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	PutObject(context.Context, *s3.PutObjectInput, ...func(*s3.Options)) (*s3.PutObjectOutput, error)
}

type s3Store struct {
	client      s3API
	bucket, key string
}

func newS3Store(ctx context.Context, cfg config.Config) (*s3Store, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(cfg.S3Region))
	if err != nil {
		return nil, fmt.Errorf("加载 AWS 配置失败: %w", err)
	}
	client := s3.NewFromConfig(awsCfg, func(options *s3.Options) {
		if cfg.S3Endpoint != "" {
			options.BaseEndpoint = &cfg.S3Endpoint
			options.UsePathStyle = true
		}
	})
	return &s3Store{client: client, bucket: cfg.S3Bucket, key: cfg.S3Key}, nil
}

func (s *s3Store) Load(ctx context.Context) (*model.State, error) {
	output, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: &s.bucket, Key: &s.key})
	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			switch apiErr.ErrorCode() {
			case "NoSuchKey", "NotFound", "404":
				return nil, nil
			}
		}
		return nil, fmt.Errorf("从 S3 读取状态失败: %w", err)
	}
	defer output.Body.Close()
	var state model.State
	if err := json.NewDecoder(io.LimitReader(output.Body, 1<<20)).Decode(&state); err != nil {
		return nil, nil
	}
	return &state, nil
}

func (s *s3Store) Save(ctx context.Context, state model.State) error {
	body, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("编码状态失败: %w", err)
	}
	contentType := "application/json; charset=utf-8"
	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: &s.bucket, Key: &s.key, Body: bytes.NewReader(body), ContentType: &contentType,
	})
	if err != nil {
		return fmt.Errorf("向 S3 保存状态失败: %w", err)
	}
	return nil
}
