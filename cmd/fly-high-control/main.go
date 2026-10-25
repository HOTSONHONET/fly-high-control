package main

import (
	"context"
	"fmt"
	"log"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/HOTSONHONET/fly-high-control/internal/announce"
	"github.com/HOTSONHONET/fly-high-control/internal/config"
	"github.com/joho/godotenv"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/socketmode"
)

func main() {
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	api := slack.New(
		cfg.BotToken,
		slack.OptionAppLevelToken(cfg.AppToken),
	)
	client := socketmode.New(api)
	runner := announce.NewRunner(cfg.Targets)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go handleEvents(ctx, client, api, cfg, runner)

	log.Printf("fly-high-control connecting to Slack Socket Mode")
	if err := client.RunContext(ctx); err != nil && ctx.Err() == nil {
		log.Fatalf("socket mode: %v", err)
	}
}

func handleEvents(
	ctx context.Context,
	client *socketmode.Client,
	api *slack.Client,
	cfg config.Config,
	runner *announce.Runner,
) {
	for {
		select {
		case <-ctx.Done():
			return
		case evt := <-client.Events:
			switch evt.Type {
			case socketmode.EventTypeConnected:
				log.Printf("connected to Slack")
			case socketmode.EventTypeConnectionError:
				log.Printf("Slack connection error")
			case socketmode.EventTypeSlashCommand:
				handleSlashCommand(client, api, cfg, runner, evt)
			}
		}
	}
}

func handleSlashCommand(
	client *socketmode.Client,
	api *slack.Client,
	cfg config.Config,
	runner *announce.Runner,
	evt socketmode.Event,
) {
	cmd, ok := evt.Data.(slack.SlashCommand)
	if !ok || evt.Request == nil {
		return
	}

	if cmd.Command != "/announce" {
		client.Ack(*evt.Request, map[string]any{"text": "Unsupported command."})
		return
	}

	if _, allowed := cfg.AllowedUsers[strings.ToLower(cmd.UserID)]; !allowed {
		client.Ack(*evt.Request, map[string]any{"text": "⛔ You are not allowed to use /announce."})
		log.Printf("rejected /announce from Slack user %s", cmd.UserID)
		return
	}

	parsed, err := announce.Parse(cmd.Text, cfg.TargetNames)
	if err != nil {
		client.Ack(*evt.Request, map[string]any{
			"text": fmt.Sprintf("❌ %v\nExample: /announce legion Dinner is ready", err),
		})
		return
	}

	client.Ack(*evt.Request, map[string]any{
		"text": fmt.Sprintf("🔊 Sending announcement to *%s*…", parsed.Target),
	})

	go func() {
		runCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		err := runner.Run(runCtx, parsed)
		if err != nil {
			log.Printf("announcement failed target=%s user=%s: %v", parsed.Target, cmd.UserID, err)
			_, errPost := api.PostEphemeral(
				cmd.ChannelID,
				cmd.UserID,
				slack.MsgOptionText(fmt.Sprintf("❌ Announcement to *%s* failed: %v", parsed.Target, err), false),
			)
			if errPost != nil {
				log.Printf("failed to post Slack error response: %v", errPost)
			}
			return
		}

		log.Printf("announcement sent target=%s user=%s", parsed.Target, cmd.UserID)
		_, errPost := api.PostEphemeral(
			cmd.ChannelID,
			cmd.UserID,
			slack.MsgOptionText(fmt.Sprintf("✅ Announcement sent to *%s*.", parsed.Target), false),
		)
		if errPost != nil {
			log.Printf("failed to post Slack success response: %v", errPost)
		}
	}()
}
