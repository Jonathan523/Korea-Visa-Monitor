package monitor

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/Jonathan523/Korea-Visa-Monitor/internal/config"
	"github.com/Jonathan523/Korea-Visa-Monitor/internal/model"
	"github.com/Jonathan523/Korea-Visa-Monitor/internal/notify"
	"github.com/Jonathan523/Korea-Visa-Monitor/internal/store"
)

type Querier interface {
	Query(context.Context, string, string, string) (model.Result, error)
}

type Runner struct {
	Config   config.Config
	Store    store.Store
	Querier  Querier
	Notifier notify.Sender
	Now      func() time.Time
	Output   io.Writer
}

func (r Runner) Run(ctx context.Context) error {
	now := r.Now().In(r.Config.Location)
	if !r.Config.InWindow(now) {
		fmt.Fprintf(r.Output, "%s 不在 %s-%s (UTC+8) 窗口内，跳过本次检查\n",
			now.Format("2006-01-02 15:04"), r.Config.WindowStart.Format("15:04"), r.Config.WindowEnd.Format("15:04"))
		return nil
	}

	previous, err := r.Store.Load(ctx)
	if err != nil {
		return fmt.Errorf("读取历史状态失败: %w", err)
	}
	if previous != nil && previous.IsIssued() {
		fmt.Fprintf(r.Output, "%s 持久化状态已签发，跳过网页检查\n", now.Format("15:04"))
		return nil
	}

	result, err := r.Querier.Query(ctx, r.Config.PassportNumber, r.Config.EnglishName, r.Config.Birthday)
	if err != nil {
		return fmt.Errorf("查询签证状态失败: %w", err)
	}
	current := model.State{
		StatusText: result.StateString(), Summary: result.Summary(),
		Issued: result.Issued(), UpdatedAt: now,
	}

	if previous == nil {
		title := "签证监控已启动（首次运行）"
		if current.Issued {
			title = "恭喜！您的签证已被签发！"
		}
		if err := r.Notifier.Send(ctx, title, current.Summary); err != nil {
			return fmt.Errorf("发送首次通知失败: %w", err)
		}
		if err := r.Store.Save(ctx, current); err != nil {
			return fmt.Errorf("保存初始状态失败: %w", err)
		}
		fmt.Fprintf(r.Output, "%s 首次运行，已发送通知并记录初始状态\n", now.Format("15:04"))
		return nil
	}

	if current.StatusText == previous.StatusText {
		fmt.Fprintf(r.Output, "%s 状态无变化，不通知\n", now.Format("15:04"))
		return nil
	}
	title := "签证申请状态有变化"
	if current.Issued {
		title = "恭喜！您的签证已被签发！"
	}
	oldSummary := previous.Summary
	if oldSummary == "" {
		oldSummary = previous.StatusText
	}
	body := fmt.Sprintf("【旧状态】\n%s\n\n【新状态】\n%s", oldSummary, current.Summary)
	if err := r.Notifier.Send(ctx, title, body); err != nil {
		return fmt.Errorf("发送状态变化通知失败: %w", err)
	}
	if err := r.Store.Save(ctx, current); err != nil {
		return fmt.Errorf("保存新状态失败: %w", err)
	}
	fmt.Fprintf(r.Output, "%s 状态变化，已发送通知\n", now.Format("15:04"))
	return nil
}
