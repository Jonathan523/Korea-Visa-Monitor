package config

import (
	"testing"
	"time"
)

func TestInWindowSupportsDayAndOvernightRanges(t *testing.T) {
	loc := time.FixedZone("UTC+8", 8*60*60)
	at := func(hour, minute int) time.Time { return time.Date(2026, 9, 12, hour, minute, 0, 0, loc) }
	cases := []struct {
		start, end string
		now        time.Time
		want       bool
	}{
		{"08:00", "20:00", at(8, 0), true},
		{"08:00", "20:00", at(20, 0), false},
		{"22:00", "06:00", at(23, 30), true},
		{"22:00", "06:00", at(12, 0), false},
	}
	for _, tc := range cases {
		start, _ := time.ParseInLocation("15:04", tc.start, loc)
		end, _ := time.ParseInLocation("15:04", tc.end, loc)
		cfg := Config{WindowStart: start, WindowEnd: end, Location: loc}
		if got := cfg.InWindow(tc.now); got != tc.want {
			t.Errorf("%s-%s at %s: got %v want %v", tc.start, tc.end, tc.now.Format("15:04"), got, tc.want)
		}
	}
}

func TestLoadNormalizesValues(t *testing.T) {
	t.Setenv("VISA_PASSPORT_NUMBER", " P123 ")
	t.Setenv("VISA_ENGLISH_NAME", " zhang san ")
	t.Setenv("VISA_BIRTHDAY", "1990-01-31")
	t.Setenv("VISA_PUSHDEER_KEY", "key")
	t.Setenv("VISA_S3_ENDPOINT_URL", "s3.example.com")
	cfg, warnings, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 || cfg.EnglishName != "ZHANG SAN" || cfg.S3Endpoint != "https://s3.example.com" {
		t.Fatalf("unexpected config %#v, warnings %#v", cfg, warnings)
	}
}
