package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type Peer struct {
	Name string
	URL  string
}

type Config struct {
	AppToken       string
	BotToken       string
	AllowedUsers   map[string]struct{}
	NodeName       string
	ListenAddr     string
	ClusterToken   string
	AudioDevice    string
	AnnounceTimeout time.Duration
	Peers          map[string]Peer
	TargetNames    map[string]struct{}
}

func Load() (Config, error) {
	cfg := Config{
		AppToken:     strings.TrimSpace(os.Getenv("SLACK_APP_TOKEN")),
		BotToken:     strings.TrimSpace(os.Getenv("SLACK_BOT_TOKEN")),
		AllowedUsers: csvSet(os.Getenv("SLACK_ALLOWED_USER_IDS")),
		NodeName:     strings.ToLower(strings.TrimSpace(os.Getenv("NODE_NAME"))),
		ListenAddr:   defaultIfEmpty(os.Getenv("LISTEN_ADDR"), ":8787"),
		ClusterToken: strings.TrimSpace(os.Getenv("FLY_HIGH_CLUSTER_TOKEN")),
		AudioDevice:  strings.TrimSpace(os.Getenv("ANNOUNCE_AUDIO_DEVICE")),
		AnnounceTimeout: durationOrDefault(os.Getenv("ANNOUNCE_TIMEOUT"), 5*time.Minute),
		Peers:        map[string]Peer{},
		TargetNames:  map[string]struct{}{},
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
	if cfg.NodeName == "" {
		return Config{}, fmt.Errorf("NODE_NAME must be set")
	}
	if cfg.ClusterToken == "" {
		return Config{}, fmt.Errorf("FLY_HIGH_CLUSTER_TOKEN must be set")
	}

	cfg.TargetNames[cfg.NodeName] = struct{}{}

	for _, item := range strings.Split(os.Getenv("PEERS"), ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		parts := strings.SplitN(item, "=", 2)
		if len(parts) != 2 {
			return Config{}, fmt.Errorf("invalid PEERS entry %q, expected name=url", item)
		}
		name := strings.ToLower(strings.TrimSpace(parts[0]))
		url := strings.TrimRight(strings.TrimSpace(parts[1]), "/")
		if name == "" || url == "" {
			return Config{}, fmt.Errorf("invalid PEERS entry %q", item)
		}
		cfg.Peers[name] = Peer{Name: name, URL: url}
		cfg.TargetNames[name] = struct{}{}
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

func durationOrDefault(value string, fallback time.Duration) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}
