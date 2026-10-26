package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/HOTSONHONET/fly-high-control/internal/announce"
	"github.com/HOTSONHONET/fly-high-control/internal/config"
	"github.com/HOTSONHONET/fly-high-control/internal/peer"
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

	runner := announce.NewRunner(cfg.AudioDevice)
	peerService := peer.New(cfg, runner)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	httpServer := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           peerService.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("peer API listening on %s as node=%s", cfg.ListenAddr, cfg.NodeName)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("peer API: %v", err)
		}
	}()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()

	api := slack.New(
		cfg.BotToken,
		slack.OptionAppLevelToken(cfg.AppToken),
	)
	client := socketmode.New(api)

	go handleEvents(ctx, client, api, cfg, peerService)

	log.Printf("fly-high-control connecting to Slack Socket Mode as node=%s", cfg.NodeName)
	if err := client.RunContext(ctx); err != nil && ctx.Err() == nil {
		log.Fatalf("socket mode: %v", err)
	}
}

func handleEvents(
	ctx context.Context,
	client *socketmode.Client,
	api *slack.Client,
	cfg config.Config,
	peerService *peer.Service,
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
				handleSlashCommand(client, api, cfg, peerService, evt)
			}
		}
	}
}

func handleSlashCommand(
	client *socketmode.Client,
	api *slack.Client,
	cfg config.Config,
	peerService *peer.Service,
	evt socketmode.Event,
) {
	cmd, ok := evt.Data.(slack.SlashCommand)
	if !ok || evt.Request == nil {
		return
	}

	if _, allowed := cfg.AllowedUsers[strings.ToLower(cmd.UserID)]; !allowed {
		client.Ack(*evt.Request, map[string]any{"text": "⛔ You are not allowed to use Fly High control commands."})
		log.Printf("rejected %s from Slack user %s", cmd.Command, cmd.UserID)
		return
	}

	switch cmd.Command {
	case "/announce":
		handleAnnounceCommand(client, api, cfg, peerService, evt, cmd)
	case "/fly-status", "/gpu", "/disk", "/docker", "/uptime", "/battery":
		handleMonitorCommand(client, api, cfg, peerService, evt, cmd)
	default:
		client.Ack(*evt.Request, map[string]any{"text": "Unsupported command."})
	}
}

func handleAnnounceCommand(
	client *socketmode.Client,
	api *slack.Client,
	cfg config.Config,
	peerService *peer.Service,
	evt socketmode.Event,
	cmd slack.SlashCommand,
) {
	parsed, err := announce.Parse(cmd.Text, cfg.TargetNames)
	if err != nil {
		client.Ack(*evt.Request, map[string]any{
			"text": fmt.Sprintf("❌ %v\nExample: /announce 3 legion Dinner is ready", err),
		})
		return
	}

	client.Ack(*evt.Request, map[string]any{
		"text": fmt.Sprintf("🔊 Sending announcement to *%s* %d time(s)…", parsed.Target, parsed.Repeat),
	})

	go func() {
		runCtx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
		defer cancel()

		results := peerService.Execute(runCtx, parsed)
		var okNodes []string
		var failed []string
		for _, result := range results {
			if result.Error != nil {
				failed = append(failed, fmt.Sprintf("%s: %v", result.Node, result.Error))
				continue
			}
			okNodes = append(okNodes, result.Node)
		}

		if len(failed) > 0 {
			log.Printf("announcement partial/failed target=%s user=%s failures=%v", parsed.Target, cmd.UserID, failed)
			text := "⚠️ Announcement completed with issues."
			if len(okNodes) > 0 {
				text += fmt.Sprintf("\n✅ %s", strings.Join(okNodes, ", "))
			}
			text += fmt.Sprintf("\n❌ %s", strings.Join(failed, "\n❌ "))
			_, errPost := api.PostEphemeral(cmd.ChannelID, cmd.UserID, slack.MsgOptionText(text, false))
			if errPost != nil {
				log.Printf("failed to post Slack result: %v", errPost)
			}
			return
		}

		log.Printf("announcement sent target=%s user=%s nodes=%v", parsed.Target, cmd.UserID, okNodes)
		_, errPost := api.PostEphemeral(
			cmd.ChannelID,
			cmd.UserID,
			slack.MsgOptionText(
				fmt.Sprintf("✅ Announcement sent %d time(s) to: *%s*.", parsed.Repeat, strings.Join(okNodes, ", ")),
				false,
			),
		)
		if errPost != nil {
			log.Printf("failed to post Slack success response: %v", errPost)
		}
	}()
}

func handleMonitorCommand(
	client *socketmode.Client,
	api *slack.Client,
	cfg config.Config,
	peerService *peer.Service,
	evt socketmode.Event,
	cmd slack.SlashCommand,
) {
	target := strings.ToLower(strings.TrimSpace(cmd.Text))
	if target == "" {
		target = cfg.NodeName
	}
	if target != "all" {
		if _, ok := cfg.TargetNames[target]; !ok {
			client.Ack(*evt.Request, map[string]any{
				"text": fmt.Sprintf("❌ Unknown target %q. Use: acer, legion, or all.", target),
			})
			return
		}
	}

	kind := strings.TrimPrefix(cmd.Command, "/")
	if kind == "fly-status" {
		kind = "status"
	}
	client.Ack(*evt.Request, map[string]any{
		"text": fmt.Sprintf("📊 Fetching %s from *%s*…", kind, target),
	})

	go func() {
		runCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		reports := peerService.Monitor(runCtx, target, kind)
		sections := make([]string, 0, len(reports))
		for _, report := range reports {
			sections = append(sections, fmt.Sprintf("*%s*\n%s", report.Node, report.Summary))
		}

		_, errPost := api.PostEphemeral(
			cmd.ChannelID,
			cmd.UserID,
			slack.MsgOptionText(strings.Join(sections, "\n\n"), false),
		)
		if errPost != nil {
			log.Printf("failed to post %s result: %v", kind, errPost)
		}
	}()
}
