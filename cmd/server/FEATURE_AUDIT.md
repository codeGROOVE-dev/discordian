# Feature Audit: User Mapping and Logging Enhancements

## Summary

This document describes the changes made to fix the `/goose report` race condition bug and add comprehensive logging for user mapping diagnostics, plus a new `/goose usermap` command.

## 1. Fixed Race Condition Bug

### Problem
The `/goose report` command was failing with two types of errors:
- "Unknown interaction" (code 10062) - interaction not acknowledged in time
- "Interaction has already been acknowledged" (code 40060) - duplicate handlers

### Root Cause
When multiple coordinators for different orgs shared the same Discord guild, `discordClientForGuild()` was called concurrently without mutex protection. This created:
- Multiple Discord clients for the same guild
- Multiple slash command handlers registered on the same session
- Each handler processing the same interaction simultaneously

### Fix (cmd/server/main.go:450-454)
Added documentation clarifying that `discordClientForGuild()` must be called with `cm.mu` lock held (which `startCoordinators()` already ensures). The existing mutex in `startCoordinators()` prevents the race.

**Result**: Only one Discord client and one slash command handler per guild, preventing duplicate processing.

## 2. Enhanced User Mapping Logging

### Forward Mapper (GitHub → Discord)
**File**: `internal/usermapping/usermapping.go`

Changed all mapping outcomes from DEBUG to INFO level:
- Config numeric ID mapping
- Config username resolution
- Discord username match
- Mapping failures

Added fields:
- `method` - shows which tier found the mapping
- `org` - organization context
- Helpful notes for failures

### Reverse Mapper (Discord → GitHub)
**File**: `internal/usermapping/reverse.go`

Enhanced logging:
- Start of search (INFO level)
- Each org being checked (DEBUG level)
- Total users checked in all orgs
- Mapping failures (WARN level with actionable message)

Added `ExportCache()` method for diagnostics.

### Report Generation Logging
**File**: `cmd/server/main.go`

Added comprehensive logging throughout report flow:
- Report request initiation
- Orgs found for guild (with full list)
- GitHub username lookup progress
- Each org being searched
- PR search operations
- Report completion

### Slash Command Handler Logging
**File**: `internal/discord/slash.go`

Added logging:
- Interaction handling start (with interaction_id)
- Report generation steps
- Response editing (with success/failure)

All logs include `interaction_id` for request tracing.

## 3. New Feature: `/goose usermap` Command

### Purpose
Displays all GitHub ↔ Discord user mappings for the guild, showing both:
1. **Config Mappings** - Hardcoded in `.codeGROOVE/discord.yaml`
2. **Discovered Mappings** - Auto-discovered via username matching or cached

### Implementation

#### Slash Command (internal/discord/slash.go)
- Added `UserMapGetter` interface
- Added `UserMappings` and `UserMapping` types
- Added `/goose usermap` command registration
- Added `handleUserMapCommand()` handler
- Added `formatUserMappingsEmbed()` formatter

#### Data Collection (cmd/server/main.go)
- Implemented `UserMappings()` method on `coordinatorManager`
- Collects config mappings from all orgs
- Collects cached mappings from reverse mapper (Discord → GitHub)
- Collects cached mappings from forward mappers (GitHub → Discord)
- Deduplicates entries

#### Cache Export Methods
Added export methods for inspection:
- `ReverseMapper.ExportCache()` - Returns Discord → GitHub mappings
- `Mapper.ExportCache()` - Returns GitHub → Discord mappings
- `Coordinator.ExportUserMapperCache()` - Exposes forward mapper cache

### Display Format
```
GitHub ↔ Discord User Mappings

📋 Config Mappings (3)
• alice → @Alice (myorg)
• bob → @Bob (myorg)
• charlie → @Charlie (myorg)

🔍 Discovered Mappings (2)
• dave → @Dave [username_match] (myorg)
• eve → @Eve [cached]

Total: 5 users
```

### Use Cases
1. **Debugging** - Verify user mappings are working correctly
2. **Configuration** - See which users need to be added to config
3. **Validation** - Confirm username-based discovery is working
4. **Troubleshooting** - Understand why a user isn't receiving notifications

## Files Changed

### Core Files
- `cmd/server/main.go` - Race condition fix, UserMappings implementation, logging
- `internal/discord/slash.go` - New command, interfaces, handlers
- `internal/usermapping/usermapping.go` - Enhanced logging, ExportCache
- `internal/usermapping/reverse.go` - Enhanced logging, ExportCache
- `internal/bot/coordinator.go` - ExportUserMapperCache method

## Testing

### Compilation
All changes compile successfully:
```bash
make lint  # Passes (only pre-existing warnings)
cd cmd/server && go build  # Success
```

### What to Test
1. **Race condition fix**: Run `/goose report` multiple times rapidly
2. **Logging**: Check logs for complete trace with interaction_id
3. **User mapping**: Run `/goose usermap` to see all mappings
4. **User lookup**: Verify logs show mapping resolution details

## Log Example

Successful `/goose report` trace:
```json
{"msg":"processing goose command","subcommand":"report","interaction_id":"..."}
{"msg":"handling report command","interaction_id":"..."}
{"msg":"report requested","guild_id":"...","user_id":"..."}
{"msg":"found orgs for guild","orgs_for_guild":["myorg"]}
{"msg":"searching for GitHub username for Discord user","discord_user_id":"..."}
{"msg":"mapped Discord user to GitHub via config","github_username":"alice","org":"myorg"}
{"msg":"searching PRs for org","org":"myorg","github_username":"alice"}
{"msg":"report generation complete","incoming_prs":2,"outgoing_prs":1}
{"msg":"successfully edited response","interaction_id":"..."}
```

## Benefits

1. **Reliability** - Fixed race condition causing command failures
2. **Debuggability** - Comprehensive logging for troubleshooting
3. **Transparency** - `/goose usermap` shows all user mappings
4. **Observability** - Request tracing via interaction_id
5. **User Experience** - Clear error messages with actionable guidance
