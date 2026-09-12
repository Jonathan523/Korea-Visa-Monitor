package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Jonathan523/Korea-Visa-Monitor/internal/config"
	"github.com/Jonathan523/Korea-Visa-Monitor/internal/monitor"
	"github.com/Jonathan523/Korea-Visa-Monitor/internal/notify"
	"github.com/Jonathan523/Korea-Visa-Monitor/internal/store"
	"github.com/Jonathan523/Korea-Visa-Monitor/internal/visa"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "运行失败："+err.Error())
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	cfg, warnings, err := config.Load()
	for _, warning := range warnings {
		fmt.Fprintln(os.Stderr, "警告："+warning)
	}
	if err != nil {
		return fmt.Errorf("配置错误: %w", err)
	}
	client := visa.NewClient(cfg.VisaURL)
	if len(args) > 0 {
		if args[0] != "query" {
			return fmt.Errorf("未知命令 %q；可用命令：query", args[0])
		}
		if err := cfg.ValidateQuery(); err != nil {
			return fmt.Errorf("配置错误: %w", err)
		}
		result, err := client.Query(ctx, cfg.PassportNumber, cfg.EnglishName, cfg.Birthday)
		if err != nil {
			return err
		}
		fmt.Fprintln(os.Stdout, result.Summary())
		return nil
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("配置错误: %w", err)
	}
	stateStore, err := store.New(ctx, cfg)
	if err != nil {
		return err
	}
	runner := monitor.Runner{
		Config: cfg, Store: stateStore, Querier: client, Notifier: notify.New(cfg),
		Now: time.Now, Output: os.Stdout,
	}
	return runner.Run(ctx)
}
