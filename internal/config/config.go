package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultPushDeerEndpoint = "https://api2.pushdeer.com/message/push"
	defaultVisaURL          = "https://www.visa.go.kr/openPage.do?MENU_ID=10301"
)

type Config struct {
	PassportNumber string
	EnglishName    string
	Birthday       string
	WindowStart    time.Time
	WindowEnd      time.Time
	Location       *time.Location
	Debug          bool
	VisaURL        string

	PushChannel      string
	PushDeerKey      string
	PushDeerEndpoint string
	ServerChanKey    string

	StateStorage string
	StateFile    string
	S3Bucket     string
	S3Key        string
	S3Region     string
	S3Endpoint   string
	UpstashURL   string
	UpstashToken string
	UpstashKey   string
}

func Load() (Config, []string, error) {
	loc := time.FixedZone("UTC+8", 8*60*60)
	start, startWarning := parseClock("VISA_WINDOW_START", "08:00", loc)
	end, endWarning := parseClock("VISA_WINDOW_END", "20:00", loc)
	warnings := compact(startWarning, endWarning)

	cwd, err := os.Getwd()
	if err != nil {
		return Config{}, warnings, fmt.Errorf("获取当前目录失败: %w", err)
	}

	c := Config{
		PassportNumber:   strings.TrimSpace(os.Getenv("VISA_PASSPORT_NUMBER")),
		EnglishName:      strings.ToUpper(strings.TrimSpace(os.Getenv("VISA_ENGLISH_NAME"))),
		Birthday:         strings.TrimSpace(os.Getenv("VISA_BIRTHDAY")),
		WindowStart:      start,
		WindowEnd:        end,
		Location:         loc,
		Debug:            envBool("VISA_DEBUG"),
		VisaURL:          envDefault("VISA_QUERY_URL", defaultVisaURL),
		PushChannel:      strings.ToLower(envDefault("VISA_PUSH_CHANNEL", "pushdeer")),
		PushDeerKey:      strings.TrimSpace(os.Getenv("VISA_PUSHDEER_KEY")),
		PushDeerEndpoint: envDefault("VISA_PUSHDEER_ENDPOINT", defaultPushDeerEndpoint),
		ServerChanKey:    strings.TrimSpace(os.Getenv("VISA_SERVERCHAN_KEY")),
		StateStorage:     strings.ToLower(envDefault("VISA_STATE_STORAGE", "local")),
		StateFile:        envDefault("VISA_STATE_FILE", filepath.Join(cwd, "visa_state.json")),
		S3Bucket:         strings.TrimSpace(os.Getenv("VISA_S3_BUCKET")),
		S3Key:            envDefault("VISA_S3_KEY", "visa_state.json"),
		S3Region:         strings.TrimSpace(firstNonEmpty(os.Getenv("VISA_S3_REGION"), os.Getenv("AWS_DEFAULT_REGION"), "ap-northeast-2")),
		S3Endpoint:       normalizeEndpoint(os.Getenv("VISA_S3_ENDPOINT_URL")),
		UpstashURL:       strings.TrimRight(strings.TrimSpace(os.Getenv("UPSTASH_REDIS_REST_URL")), "/"),
		UpstashToken:     strings.TrimSpace(os.Getenv("UPSTASH_REDIS_REST_TOKEN")),
		UpstashKey:       envDefault("VISA_UPSTASH_KEY", "krvisa:visa_state"),
	}
	return c, warnings, nil
}

func (c Config) Validate() error {
	if err := c.ValidateQuery(); err != nil {
		return err
	}
	switch c.PushChannel {
	case "pushdeer":
		if c.PushDeerKey == "" {
			return fmt.Errorf("使用 PushDeer 时必须配置 VISA_PUSHDEER_KEY")
		}
		if err := validHTTPURL("VISA_PUSHDEER_ENDPOINT", c.PushDeerEndpoint); err != nil {
			return err
		}
	case "serverchan":
		if c.ServerChanKey == "" {
			return fmt.Errorf("使用 Server酱时必须配置 VISA_SERVERCHAN_KEY")
		}
	default:
		return fmt.Errorf("未知推送渠道 %q（可选：pushdeer / serverchan）", c.PushChannel)
	}
	switch c.StateStorage {
	case "local":
		if strings.TrimSpace(c.StateFile) == "" {
			return fmt.Errorf("VISA_STATE_FILE 不能为空")
		}
	case "s3":
		if c.S3Bucket == "" || c.S3Key == "" {
			return fmt.Errorf("使用 S3 时必须配置 VISA_S3_BUCKET，且 VISA_S3_KEY 不能为空")
		}
		if c.S3Endpoint != "" {
			if err := validHTTPURL("VISA_S3_ENDPOINT_URL", c.S3Endpoint); err != nil {
				return err
			}
		}
	case "upstash":
		if c.UpstashURL == "" || c.UpstashToken == "" || c.UpstashKey == "" {
			return fmt.Errorf("使用 Upstash 时必须配置 REST URL、Token，且 key 不能为空")
		}
		if err := validHTTPURL("UPSTASH_REDIS_REST_URL", c.UpstashURL); err != nil {
			return err
		}
	default:
		return fmt.Errorf("未知状态存储方式 %q（可选：local / s3 / upstash）", c.StateStorage)
	}
	return nil
}

func (c Config) ValidateQuery() error {
	if c.PassportNumber == "" || c.EnglishName == "" || c.Birthday == "" {
		return fmt.Errorf("请设置 VISA_PASSPORT_NUMBER / VISA_ENGLISH_NAME / VISA_BIRTHDAY")
	}
	if _, err := time.Parse("2006-01-02", c.Birthday); err != nil {
		return fmt.Errorf("VISA_BIRTHDAY 必须是有效的 YYYY-MM-DD 日期")
	}
	return validHTTPURL("VISA_QUERY_URL", c.VisaURL)
}

func (c Config) InWindow(now time.Time) bool {
	now = now.In(c.Location)
	minutes := now.Hour()*60 + now.Minute()
	start := c.WindowStart.Hour()*60 + c.WindowStart.Minute()
	end := c.WindowEnd.Hour()*60 + c.WindowEnd.Minute()
	if start == end {
		return true
	}
	if start < end {
		return minutes >= start && minutes < end
	}
	return minutes >= start || minutes < end
}

func parseClock(name, fallback string, loc *time.Location) (time.Time, string) {
	raw := envDefault(name, fallback)
	t, err := time.ParseInLocation("15:04", raw, loc)
	if err == nil {
		return t, ""
	}
	t, _ = time.ParseInLocation("15:04", fallback, loc)
	return t, fmt.Sprintf("环境变量 %s 无效（应为 HH:MM），使用默认值 %s", name, fallback)
}

func envDefault(name, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(name)); v != "" {
		return v
	}
	return fallback
}

func envBool(name string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(name))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func normalizeEndpoint(value string) string {
	value = strings.TrimSpace(value)
	if value != "" && !strings.Contains(value, "://") {
		return "https://" + value
	}
	return strings.TrimRight(value, "/")
}

func validHTTPURL(name, value string) error {
	u, err := url.Parse(value)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("%s 必须是有效的 HTTP(S) URL", name)
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func compact(values ...string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}
