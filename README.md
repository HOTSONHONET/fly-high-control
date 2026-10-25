# fly-high-control

Slack control plane for the Fly High NAS / homelab.

## Current feature

```text
/announce <legion|acer|all> <message>
```

Examples:

```text
/announce legion Dinner is ready
/announce acer Please come to the living room
/announce all Testing the announcement system
```

The controller uses Slack Socket Mode, so it does not require a public HTTP endpoint.

## Slack app configuration

Configure the Slack app with:

- Socket Mode enabled
- App-level token with `connections:write`
- Bot scopes:
  - `commands`
  - `chat:write`
- Slash command: `/announce`

## Configuration

Copy the template:

```bash
cp .env.example .env
```

Then fill in your real tokens and Slack member ID:

```env
SLACK_APP_TOKEN=xapp-...
SLACK_BOT_TOKEN=xoxb-...
SLACK_ALLOWED_USER_IDS=U...
```

Never commit `.env`.

### Announcement targets

A target can run in one of two modes:

- `local`: run `espeak-ng` and `aplay` on the controller host.
- `ssh`: connect to another host over SSH, then run `espeak-ng` and `aplay` there.

The default template assumes the controller runs on the Legion:

```env
ANNOUNCE_TARGETS=legion,acer

ANNOUNCE_LEGION_MODE=local
ANNOUNCE_LEGION_AUDIO_DEVICE=plughw:1,0

ANNOUNCE_ACER_MODE=ssh
ANNOUNCE_ACER_SSH_HOST=fly-high-acer
ANNOUNCE_ACER_AUDIO_DEVICE=default
```

For SSH targets, the controller host must already be able to SSH to the target non-interactively.

## Host dependencies

Install these on each machine that will speak:

```bash
sudo apt update
sudo apt install -y espeak-ng alsa-utils
```

For remote targets, `base64` is also required (provided by GNU coreutils on Ubuntu).

## Run

```bash
go mod tidy
go test ./...
go run ./cmd/fly-high-control
```

When the service connects successfully, it logs:

```text
connected to Slack
```

Then invoke `/announce` from Slack.

## Security

`SLACK_ALLOWED_USER_IDS` is mandatory. Requests from Slack users not in that allowlist are rejected before any announcement is executed.

Messages are passed directly as process arguments for local TTS and are base64 encoded before being sent to SSH targets, so announcement text is not treated as a shell command.
