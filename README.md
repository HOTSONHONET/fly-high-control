# fly-high-control

Distributed Slack control plane for the Fly High homelab.

Every machine runs the same service. Each node:

- connects to Slack over Socket Mode
- exposes a private authenticated peer API on port 8787
- executes its own announcements locally
- can forward commands to peer nodes
- can continue handling Slack commands even if another node is offline

## /announce

```text
/announce <num-times> <legion|acer|all> <message>
```

Examples:

```text
/announce 1 legion Dinner is ready
/announce 2 acer Please come to the living room
/announce 3 all Testing the announcement system
```

The repeat count must be between 1 and 10.

## Architecture

```text
                      Slack
                 Socket Mode
                 /          \
                /            \
             Acer          Legion
       fly-high-control  fly-high-control
             |              |
       local speaker    local speaker
             \              /
              \--- peer HTTP
                  :8787
```

Both nodes may keep a Socket Mode connection open. A Slack command is handled by whichever node receives it. That node becomes the coordinator for that command only.

For `/announce 3 all hello`, if Acer receives the Slack event:

1. Acer plays `hello` locally 3 times.
2. Acer sends an authenticated HTTP request to Legion.
3. Legion plays `hello` locally 3 times.
4. Slack receives per-node success/failure results.

If Legion is down, Acer still completes its own announcement and Slack reports Legion as unreachable.

## Slack app configuration

Configure the Slack app with:

- Socket Mode enabled
- App-level token with `connections:write`
- Bot scopes:
  - `commands`
  - `chat:write`
- Slash command:
  - `/announce`

Both nodes use the same Slack app credentials.

## Shared cluster authentication

Generate one strong random secret:

```bash
openssl rand -hex 32
```

Put the same value on every node:

```env
FLY_HIGH_CLUSTER_TOKEN=<same-secret-on-all-nodes>
```

Peer HTTP requests use:

```text
Authorization: Bearer <shared-cluster-token>
```

The peer API is intended only for your private LAN / WARP network.

## Configuration

Copy:

```bash
cp .env.example .env
```

Never commit `.env`.

### Acer .env

```env
SLACK_APP_TOKEN=xapp-...
SLACK_BOT_TOKEN=xoxb-...
SLACK_ALLOWED_USER_IDS=U...

NODE_NAME=acer
FLY_HIGH_CLUSTER_TOKEN=<shared-secret>
LISTEN_ADDR=:8787

ANNOUNCE_AUDIO_DEVICE=default

PEERS=legion=http://192.168.0.10:8787
```

### Legion .env

```env
SLACK_APP_TOKEN=xapp-...
SLACK_BOT_TOKEN=xoxb-...
SLACK_ALLOWED_USER_IDS=U...

NODE_NAME=legion
FLY_HIGH_CLUSTER_TOKEN=<shared-secret>
LISTEN_ADDR=:8787

ANNOUNCE_AUDIO_DEVICE=plughw:1,0

PEERS=acer=http://192.168.0.7:8787
```

The peer addresses use the machines' reserved LAN IPs, so no SSH aliases or DNS configuration are required.

## Docker deployment

Build and start on both Acer and Legion:

```bash
docker compose up -d --build
```

Check:

```bash
docker compose ps
docker compose logs -f fly-high-control
```

The service uses:

```yaml
restart: unless-stopped
```

so it returns automatically after Docker / host restarts.

The container exposes:

```text
8787/tcp
```

and receives the host ALSA devices:

```yaml
devices:
  - /dev/snd:/dev/snd
```

No SSH keys are required between nodes.

## Peer health check

From Acer:

```bash
curl http://192.168.0.10:8787/healthz
```

From Legion:

```bash
curl http://192.168.0.7:8787/healthz
```

Expected:

```text
ok
```

## Direct peer API test

The announcement endpoint requires the shared cluster token:

```bash
curl -X POST http://192.168.0.10:8787/api/v1/announce \
  -H "Authorization: Bearer $FLY_HIGH_CLUSTER_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"message":"hello from acer","repeat":1}'
```

A successful request returns HTTP 204.

## Host audio

Each node performs TTS locally using:

```text
espeak-ng -> WAV -> aplay -> local speaker
```

To inspect ALSA playback devices:

```bash
aplay -l
```

Then set the corresponding `ANNOUNCE_AUDIO_DEVICE`.

## Security

- `SLACK_ALLOWED_USER_IDS` is mandatory.
- Peer requests require `FLY_HIGH_CLUSTER_TOKEN`.
- The service executes fixed local TTS actions rather than accepting shell commands.
- Announcement text is passed as a process argument to `espeak-ng`.
- Keep port 8787 available only to your trusted LAN / private network.
