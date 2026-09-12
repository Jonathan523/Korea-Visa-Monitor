package model

import (
	"fmt"
	"strings"
	"time"
)

// Result is the normalized response returned by the Korea Visa Portal.
type Result struct {
	Found             bool
	Message           string
	ApplicationNumber string
	EntryPurpose      string
	Status            string
}

func (r Result) Summary() string {
	if !r.Found {
		return "未查询到记录：" + r.Message
	}
	return fmt.Sprintf("申请编号：%s\n入境目的：%s\n当前状态：%s", r.ApplicationNumber, r.EntryPurpose, r.Status)
}

func (r Result) StateString() string {
	if !r.Found {
		return "not_found|" + r.Message
	}
	return strings.Join([]string{"found", r.ApplicationNumber, r.EntryPurpose, r.Status}, "|")
}

func (r Result) Issued() bool {
	return r.Found && strings.HasPrefix(strings.TrimSpace(r.Status), "签发")
}

// State is persisted between checks. JSON field names intentionally match the
// previous Python implementation so existing state remains usable.
type State struct {
	StatusText string    `json:"status_text"`
	Summary    string    `json:"summary"`
	Issued     bool      `json:"issued"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (s State) IsIssued() bool {
	if s.Issued {
		return true
	}
	if strings.HasPrefix(s.StatusText, "found|") {
		parts := strings.Split(s.StatusText, "|")
		return len(parts) > 0 && strings.HasPrefix(strings.TrimSpace(parts[len(parts)-1]), "签发")
	}
	return false
}
