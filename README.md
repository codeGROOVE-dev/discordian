# reviewGOOSE:Discord

The Discord integration for [reviewGOOSE](https://codegroove.dev/reviewgoose/) — know instantly when you're blocking a PR.

**reviewGOOSE:Discord** tracks GitHub pull requests and notifies reviewers when it's their turn. Works alongside [reviewGOOSE:Desktop](https://github.com/codeGROOVE-dev/goose) for a complete PR tracking experience.

## Features

- Creates Discord threads for new PRs (forum or text channels)
- Smart notifications: Delays DMs if user already notified in channel
- Channel auto-discovery: repos automatically map to same-named channels
- Configurable notification settings via YAML in your repos
- Daily report system
- Reliable delivery: Persistent state and deduplication

## Quick Start

### 1. Add the Bot to Your Server

[Add reviewGOOSE to your Discord server](https://discord.com/oauth2/authorize?client_id=1461368540190871831&permissions=2147485696&scope=bot%20applications.commands)

### 2. Install the GitHub App

Install the [reviewGOOSE GitHub App](https://github.com/apps/reviewgoose) on your organization.

### 3. Configure Your Organization

Create `.codeGROOVE/discord.yaml` in your organization's `.codeGROOVE` repository:

```yaml
global:
  guild_id: "YOUR_DISCORD_SERVER_ID"

users:
  github-username: "discord-user-id"
```

To find your Discord server ID: Enable Developer Mode in Discord settings, then right-click your server name and select "Copy Server ID".

That's it! The bot will start posting PR notifications to channels matching your repository names.

## Configuration

The full configuration options for `.codeGROOVE/discord.yaml`:

```yaml
global:
  guild_id: "1234567890123456789"
  reminder_dm_delay: 65  # Minutes before DM (default: 65, 0 = disabled)

channels:
  # Route all repos to a forum channel called #pull-requests
  pull-requests:
    repos:
      - "*"

  # Route specific repos to #backend channel with shorter DM delay
  backend:
    repos:
      - api
      - db
    reminder_dm_delay: 30

# GitHub username -> Discord user ID mapping
users:
  alice: "111111111111111111"
  bob: "222222222222222222"
  # Unmapped users: bot attempts username match in guild
```

## Channel Auto-Discovery

**By default, repos automatically map to channels with the same name:**
- Repository `goose` -> Channel `#goose`
- Repository `my-service` -> Channel `#my-service`

**Forum vs Text channels are auto-detected:**
- Forum channels: Each PR gets its own thread (Discord-native, ideal for PR tracking)
- Text channels: PR updates appear as messages (similar to Slack)

**Override auto-discovery:**
- **Explicit routing**: Add repo to a channel's `repos` list to route it elsewhere
- **Wildcard**: Use `repos: ["*"]` to route all repos to one channel
- **Disable notifications**: Add `mute: true` to suppress a repo entirely:
  ```yaml
  channels:
    noisy-repo:  # Repo named "noisy-repo" will have no notifications
      mute: true
  ```

## User Mapping

The bot maps GitHub users to Discord users in three ways:

1. **Explicit mapping**: `users:` section in discord.yaml
2. **Username match**: Searches guild members for matching Discord username
3. **Fallback**: Mentions GitHub username as plain text

For best results, add explicit mappings for users whose Discord and GitHub usernames differ.

## Slash Commands

- `/goose status` - Show bot connection status
- `/goose report` - Get your personal PR report
- `/goose dashboard` - Link to web dashboard
- `/goose help` - Show help

## Smart Notification Logic

- **Channel notifications**: If a user is tagged in a channel, DMs are delayed by `reminder_dm_delay` (default: 65 minutes)
- **Immediate DMs**: If a user isn't in the notification channel, they get immediate DMs
- **Daily reports**: Sent between 6-11:30am local time if user has pending PRs
- **Anti-spam**: Rate limited to prevent notification floods

## Troubleshooting

### Bot doesn't respond to commands

1. Ensure the bot has been added to your server with correct permissions
2. Try removing and re-adding the bot using the install link above

### No notifications for my org

1. Verify the GitHub App is installed on your org
2. Check that `.codeGROOVE/discord.yaml` exists in your org's `.codeGROOVE` repo
3. Ensure `guild_id` matches your Discord server ID

### Messages not appearing in channels

1. Verify the channel name matches the repo name (or is configured in discord.yaml)
2. Check bot has "Send Messages" permission in the channel
3. For forum channels, ensure bot has "Create Public Threads" permission

### DMs not being sent

1. User must share a server with the bot
2. User must have DMs enabled from server members
3. Check `reminder_dm_delay` isn't set to 0 (disabled)

## Self-Hosting

See [DEPLOYMENT.md](DEPLOYMENT.md) for instructions on deploying your own instance.
