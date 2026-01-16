# Deploying reviewGOOSE:Discord

This guide covers deploying your own instance of the reviewGOOSE Discord bot.

## Prerequisites

- A Google Cloud Platform project (for Cloud Run deployment)
- Access to create Discord applications
- Access to create GitHub Apps

## 1. Create a Discord Application

1. Go to the [Discord Developer Portal](https://discord.com/developers/applications)
2. Click "New Application" and give it a name (e.g., "reviewGOOSE")
3. Note down the **Application ID** (also called Client ID)

## 2. Create a Bot User

1. In your application, go to the "Bot" section
2. Click "Add Bot"
3. Under "Privileged Gateway Intents", enable:
   - **Server Members Intent** (for user lookups)
   - **Message Content Intent** (for commands)
4. Click "Reset Token" and copy the **Bot Token** — save this securely

## 3. Generate Bot Invite URL

Go to "OAuth2" -> "URL Generator":
- **Scopes**: `bot`, `applications.commands`
- **Bot Permissions**:
  - Send Messages
  - Create Public Threads
  - Send Messages in Threads
  - Embed Links
  - Read Message History
  - Use Slash Commands

The permissions integer is `2147485696`.

## 4. Set Up GitHub App

Create a GitHub App for repository access. See the [GitHub App documentation](https://docs.github.com/en/developers/apps/building-github-apps/creating-a-github-app).

Required permissions:
- Repository contents: Read
- Pull requests: Read
- Metadata: Read

Subscribe to webhook events:
- Pull requests
- Pull request reviews
- Check runs

## Environment Variables

### Required

```bash
# GitHub App credentials
GITHUB_APP_ID=123456
GITHUB_PRIVATE_KEY="-----BEGIN RSA PRIVATE KEY-----\n..."

# Discord Bot token
DISCORD_BOT_TOKEN=MTIz...
```

### Optional

```bash
# Port to listen on (default: 9119)
PORT=9119

# GCP project for Secret Manager
GCP_PROJECT=my-project

# Sprinkler WebSocket URL (default: production)
SPRINKLER_URL=wss://webhook.github.codegroove.app/ws

# Turn API URL (default: production)
TURN_URL=https://turn.github.codegroove.app

# Allow personal GitHub accounts (default: false)
ALLOW_PERSONAL_ACCOUNTS=false
```

## Deployment Options

### Cloud Run (Recommended)

```bash
# Set your GCP project
export GCP_PROJECT=my-project

# Deploy
./hacks/deploy.sh
```

### Docker

```bash
# Build
docker build -t discordian .

# Run (uses local filesystem for persistence outside Cloud Run)
docker run -p 9119:9119 \
  -e GITHUB_APP_ID=... \
  -e GITHUB_PRIVATE_KEY=... \
  -e DISCORD_BOT_TOKEN=... \
  discordian
```

### Local Development

```bash
# Set environment variables
export GITHUB_APP_ID=...
export GITHUB_PRIVATE_KEY=...
export DISCORD_BOT_TOKEN=...

# Run
go run ./cmd/server
```

## Secret Manager (Recommended for Production)

The bot automatically reads secrets from Google Cloud Secret Manager. Environment variables take precedence if set.

```bash
# Create secrets (use the exact names as environment variables)
echo -n "your-bot-token" | gcloud secrets create DISCORD_BOT_TOKEN --data-file=-
echo -n "your-private-key" | gcloud secrets create GITHUB_PRIVATE_KEY --data-file=-

# Grant access to service account
for secret in DISCORD_BOT_TOKEN GITHUB_PRIVATE_KEY; do
  gcloud secrets add-iam-policy-binding $secret \
    --member="serviceAccount:discordian@${GCP_PROJECT}.iam.gserviceaccount.com" \
    --role="roles/secretmanager.secretAccessor"
done
```

Secrets loaded from Secret Manager:
- `DISCORD_BOT_TOKEN` - Discord bot token
- `GITHUB_PRIVATE_KEY` - GitHub App private key

## Datastore Setup (Cloud Run)

The bot uses Google Cloud Datastore for persistent state. You must create these databases before deploying:

```bash
# Create required Datastore databases
for db in discordian-threads discordian-dms discordian-dmusers discordian-reports discordian-pending discordian-events discordian-claims; do
  gcloud firestore databases create --database=$db --location=nam5 --type=datastore-mode
done
```

**Databases:**
| Database | Purpose | TTL |
|----------|---------|-----|
| `discordian-threads` | PR to Discord thread/message mapping | 30 days |
| `discordian-dms` | DM message tracking | 7 days |
| `discordian-dmusers` | DM user lists (prURL → user IDs) | 7 days |
| `discordian-reports` | Daily report tracking | 36 hours |
| `discordian-pending` | Pending DM queue | 4 hours |
| `discordian-events` | Event deduplication (cross-instance safety) | 2 hours |
| `discordian-claims` | Distributed claims (prevents duplicate threads) | 10 seconds |

**Optional: Enable TTL for automatic cleanup**

```bash
for db in discordian-threads discordian-dms discordian-dmusers discordian-reports discordian-pending discordian-events discordian-claims; do
  gcloud firestore fields ttls update expiry \
    --collection-group=CacheEntry \
    --enable-ttl \
    --database=$db
done
```

This automatically deletes expired entries within 24 hours.

## Architecture

```
GitHub Webhooks
      |
      v
  Sprinkler (WebSocket hub)
      |
      v
  Discordian
      |
      +---> Turn API (PR analysis)
      |
      +---> Discord API
```

The bot:
1. Connects to Sprinkler via WebSocket for real-time GitHub events
2. Discovers all orgs with the GitHub App installed
3. For each org with `.codeGROOVE/discord.yaml`, starts monitoring PRs
4. Posts/updates notifications to configured Discord channels
5. Queues DM reminders with configurable delays
6. Persists state to Datastore (Cloud Run) or local filesystem (development)

**Persistence**: State survives restarts via fido cache with Datastore backend. This includes:
- Thread/message IDs (so updates go to existing messages)
- Pending DM queue (delayed notifications survive restarts)
- Daily report tracking (prevents duplicate reports)
- Event deduplication (prevents duplicate processing across instances)
- Distributed claims (prevents duplicate threads during rolling deployments)

## Dependencies

- [sprinkler](https://github.com/codeGROOVE-dev/sprinkler) - WebSocket hub for GitHub webhooks
- [turnturnturn](https://github.com/codeGROOVE-dev/turnturnturn) - PR state analysis
- [discordgo](https://github.com/bwmarrin/discordgo) - Discord API library
- [fido](https://github.com/codeGROOVE-dev/fido) - High-performance cache with Datastore persistence

## Development

```bash
make fmt        # Format code
make lint       # Run linters
make test       # Run tests
make build      # Build binary
```

## Troubleshooting

### Bot not discovering orgs

1. Check that the GitHub App is installed on the target orgs
2. Look for "discovered GitHub installation" in logs
3. Verify `GITHUB_APP_ID` and `GITHUB_PRIVATE_KEY` are correct

### Sprinkler connection issues

1. Check `SPRINKLER_URL` is correct
2. Look for WebSocket connection errors in logs
3. Verify network connectivity to the Sprinkler service
