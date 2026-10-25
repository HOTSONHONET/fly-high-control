package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/HOTSONHONET/fly-high-control/internal/announce"
)

type Config struct {
	AppToken     string
	BotToken     string
	AllowedUsers map[string]struct{}
	Targets      []announce.Target
	TargetNames  map[string]struct{}
}

func Load() (Config, error) {
	cfg := Config{
		AppToken:     strings.TrimSpace(os.Getenv("SLACK_APP_TOKEN")),
		BotToken:     strings.TrimSpace(os.Getenv("SLACK_BOT_TOKEN")),
		AllowedUsers: csvSet(os.Getenv("SLACK_ALLOWED_USER_IDS")),
		TargetNames:  csvSet(defaultIfEmpty(os.Getenv("ANNOUNCE_TARGETS"), "legion,acer")),
	}

	if !strings.HasPrefix(cfg.AppToken, "xapp-") {
		return Config{}, fmt.Errorf("SLACK_APP_TOKEN must be set and start with xapp-")
	}
	if !strings.HasPrefix(cfg.BotToken, "xoxb-") {
		return Config{}, fmt.Errorf("SLACK_BOT_TOKEN must be set and start with xoxb-")
	}
	if len(cfg.AllowedUsers) == 0 {
		return Config{}, fmt.Errorf("SLACK_ALLOWED_USER_IDS must contain at least one Slack user ID")
	}

	for name := range cfg.TargetNames {
		prefix := "ANNOUNCE_" + strings.ToUpper(strings.ReplaceAll(name, "-", "_")) + "_"
		mode := strings.ToLower(defaultIfEmpty(os.Getenv(prefix+"MODE"), "ssh"))

		cfg.Targets = append(cfg.Targets, announce.Target{
			Name:        name,
			Mode:        mode,
			SSHHost:     strings.TrimSpace(os.Getenv(prefix + "SSH_HOST")),
			AudioDevice: strings.TrimSpace(os.Getenv(prefix + "AUDIO_DEVICE")),
		})
	}

	return cfg, nil
}

func csvSet(value string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			out[strings.ToLower(item)] = struct{}{}
		}
	}
	return out
}

func defaultIfEmpty(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}
