# CLI & Configuration Reference

Everything you need to know about running tsk and tweaking it to your liking.

---

## Commands

### `tsk`

The default command. Fetches and lists calendar events for the configured date range.

```bash
tsk
tsk --days 3
tsk --from monday --to friday
tsk --ooo=false --no-allday
```

### `tsk next`

Shows detailed information about the next upcoming event, with a countdown timer. If you've double-booked yourself (it happens), it detects the conflict and shows all concurrent events.

Supports all the same filter flags as the root command.

```bash
tsk next
tsk next --days 1
tsk next -c "Work Calendar"
```

### `tsk ui`

Launches the interactive TUI for browsing events. Event list and detail panel side by side (or stacked), with day-by-day navigation and a "NOW" marker.

```bash
tsk ui
tsk ui --split stack
tsk ui --list-percent 40
```

**TUI flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--split` | `side` | Panel layout: `side` (side-by-side) or `stack` (top/bottom) |
| `--list-percent` | `0` | List panel size as percentage (10-90). `0` = auto/responsive |

**Keyboard shortcuts:**

| Key | Action |
|-----|--------|
| `↑` / `↓` | Navigate events |
| `←` / `→` | Previous / next day |
| `t` | Jump to now (or jump to today if viewing another day) |
| `tab` | Switch focus between list and detail panels |
| `/` | Toggle split direction (side / stack) |
| `enter` | Open meeting link in browser |
| `a` | Quick accept event (single click accept) |
| `r` | Respond to event (full options modal) |
| `v` | Open event in calendar (browser) |
| `s` | Sync / refresh events |
| `ctrl+u` / `pgup` | Scroll detail panel up |
| `ctrl+d` / `pgdown` | Scroll detail panel down |
| `?` | Show help overlay |
| `q` / `ctrl+c` | Quit |

**Note:** The bottom help bar shows a simplified view with only the most common shortcuts. Press `?` to see the complete list of all available keyboard shortcuts.

### `tsk calendars`

Lists all calendars your account has access to — primary, shared, subscribed, and the ones you forgot about.

Aliases: `cal`, `cals`

```bash
tsk calendars
tsk -p work calendars
```

### `tsk auth`

Authenticates with your calendar provider via OAuth. Starts a local server on port 8085, opens a browser for sign-in, and saves the token locally. The provider is determined by the active profile's `provider` setting.

```bash
# Google (use a profile with provider: google)
tsk auth -p google_personal

# Outlook (use a profile with provider: outlook)
tsk -p outlook_work auth
```

If the browser doesn't open automatically, the URL is printed to the terminal for manual copy-paste.

### `tsk respond`

Respond to calendar event invitations with accept, decline, or tentative. Optionally include a message to the organizer and propose a new time.

**Event reference format:** `calendarID:eventID`

To find event IDs, enable `display.id: true` in your profile config and run `tsk` or `tsk next`.

```bash
# Accept an event
tsk respond primary:abc123xyz --accept

# Decline with a message
tsk respond primary:abc123xyz --decline --message "Sorry, I have a conflict"

# Tentatively accept
tsk respond primary:abc123xyz --tentative

# Accept with message (short flag)
tsk respond primary:abc123xyz --accept -m "Looking forward to it!"

# Tentatively accept with proposed new time (simplest - just times)
tsk respond primary:abc123xyz --tentative --propose "14:00/15:00"

# Propose time with date (uses local timezone)
tsk respond primary:abc123xyz --tentative --propose "2026-03-04T14:00/15:00"

# Propose time in UTC explicitly
tsk respond primary:abc123xyz --tentative --propose "2026-03-04T14:00Z/15:00Z"

# Propose time in Pacific Time (UTC-8)
tsk respond primary:abc123xyz --tentative --propose "2026-03-04T14:00-08:00/15:00-08:00"

# Decline all instances of a recurring event
tsk respond primary:abc123xyz --decline --all-instances
```

**Flags:**

| Flag | Short | Description |
|------|-------|-------------|
| `--accept` | | Accept the event invitation |
| `--decline` | | Decline the event invitation |
| `--tentative` | | Mark as tentatively accepted |
| `--message` | `-m` | Optional message to the organizer |
| `--propose` | | Propose new time (format: `HH:MM/HH:MM` or full timestamp) |
| `--all-instances` | | Respond to all instances of a recurring event |

**Recurring Events:**
- By default, responds to only the single instance you specify
- Use `--all-instances` to respond to all occurrences in the recurring series

**Time Format (Simple and Flexible):**

The `--propose` flag accepts multiple formats (simplest first):

1. **Time only** (recommended): `14:00/15:00`
   - Uses the event's date automatically
   - Uses your local timezone
   - No seconds required

2. **Date + time**: `2026-03-04T14:00/15:00`
   - Uses your local timezone
   - Useful for proposing a different date

3. **Full timestamp with timezone**: `2026-03-04T14:00Z/15:00Z`
   - Explicit UTC (Z suffix)
   - Or specify offset: `14:00-08:00` (PST), `14:00+05:30` (IST)

**Smart defaults:**
- No date? → Uses event's date
- No timezone? → Uses your local timezone
- No seconds? → Defaults to :00

The proposed time is sent to the organizer in two formats:
1. **Human-readable**: "Mon Jan 2, 2006 at 3:04 PM PST" (converted to event's timezone)
2. **RFC3339**: Full timestamp with timezone preserved for precision

**Notes:**
- Exactly one of `--accept`, `--decline`, or `--tentative` must be specified
- **Proposed times approach**:
  - **Sent to organizer**: Human-readable text in the attendee comment field
  - **Stored locally**: Structured data in `extendedProperties.private` (your calendar only)
  - Property names: `tsk:proposedStart`, `tsk:proposedEnd`, `tsk:proposedBy`
  - **API limitation**: Only the comment field propagates to the organizer; extended properties set by attendees remain on their calendar copy only
- You cannot respond to events where you are the organizer
- You can only respond to events where you are an attendee

**Provider support:**

| Provider | Support |
|----------|---------|
| Google Calendar | ✅ Supported |
| Outlook | 🚧 Coming soon |

### `tsk profile`

Manage configuration profiles.

```bash
# List all profiles (* marks the default)
tsk profile list

# Show a profile's settings
tsk profile show work

# Add a new Google profile
tsk profile add work \
  --provider google \
  --credentials-file ~/.config/tsk/work_creds.json \
  --token-file ~/.config/tsk/work_token.json \
  --days 14 \
  --smart-ooo

# Add a new Outlook profile
tsk profile add personal \
  --provider outlook \
  --client-id "your-azure-app-id" \
  --tenant-id consumers \
  --token-file ~/.config/tsk/outlook_token.json

# Edit an existing profile
tsk profile edit work --days 7 --no-allday=true

# Set the default profile
tsk profile default work
```

**Provider flags (add/edit):**

| Flag | Default | Description |
|------|---------|-------------|
| `--provider` | `google` | Calendar provider (`google` or `outlook`) |
| `--credentials-file` | | Path to Google OAuth credentials JSON |
| `--token-file` | | Path to saved OAuth token |
| `--client-id` | | Azure AD application client ID (Outlook) |
| `--tenant-id` | `common` | Azure AD tenant ID (Outlook) |

**Display flags (add/edit):**

| Flag | Default | Description |
|------|---------|-------------|
| `--show-calendar` | `true` | Show calendar name |
| `--show-time` | `true` | Show time and duration |
| `--show-location` | `true` | Show event location |
| `--show-meeting-link` | `true` | Show meeting join link |
| `--show-description` | `true` | Show event description |
| `--show-status` | `true` | Show response status |
| `--show-event-url` | `true` | Show link to event in calendar |
| `--show-attachments` | `false` | Show attachments |
| `--show-id` | `false` | Show event ID |
| `--show-in-progress` | `true` | Show in-progress indicator |

---

## Global Flags

These flags are available on every command and are inherited by subcommands.

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--config` | | `~/.config/tsk/config.yaml` | Path to config file |
| `--profile` | `-p` | | Profile to use (overrides `default_profile`) |

---

## Filter Flags

Available on `tsk`, `tsk next`, `tsk ui`, and any command that fetches events. Can also be set in profile config.

### Date Range

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--days` | `-d` | `7` | Number of days to fetch. Ignored if `--from`/`--to` are set |
| `--from` | | | Start date |
| `--to` | | | End date |

`--from` and `--to` accept several formats:

- `YYYY-MM-DD` — explicit date (`2026-03-15`)
- `MM-DD` or `MM/DD` — current year (`03-15`, `03/15`)
- `MM/DD/YYYY` — US format (`03/15/2026`)
- `today`, `tomorrow`, `yesterday`
- Weekday names — `monday`, `tue`, `friday` (next occurrence)
- `next monday`, `next friday` — explicitly next week

### Event Types

| Flag | Default | Description |
|------|---------|-------------|
| `--ooo` | `true` | Include out-of-office events |
| `--focus` | `false` | Include focus time events |
| `--workloc` | `false` | Include working location events |
| `--all-types` | `false` | Include all event types (overrides the above) |

### Event Filters

| Flag | Default | Description |
|------|---------|-------------|
| `--accepted` | `true` | Only show accepted events |
| `--subscribed` | `true` | Include events from subscribed calendars |
| `--no-allday` | `false` | Exclude all-day events |
| `--calendars` | `-c` | | Comma-separated calendar names to filter by |
| `--smart-ooo` | `false` | Hide all events on days you're out of office |
| `--primary-calendar` | | Primary calendar for smart OOO detection (auto-detected if not set) |

Smart OOO looks at your primary calendar for OOO events. On days where you're out of office, it hides everything except the OOO event itself. Useful if you don't want to see meetings you've already declined-by-absence.

---

## Configuration File

Location: `~/.config/tsk/config.yaml`

A starter config is included in the repo as `config.example.yaml`. Copy it and adjust:

```bash
mkdir -p ~/.config/tsk
cp config.example.yaml ~/.config/tsk/config.yaml
```

### Top-Level Settings

```yaml
# Which profile to use when none is specified
default_profile: work

# Fallback settings (used when no profile is active)
credentials_file: credentials.json
token_file: token.json
days: 7
ooo: true
accepted: true
subscribed: true
```

### UI Settings

Controls the `tsk ui` layout. Can also be overridden with CLI flags.

```yaml
ui:
  split: side          # "side" or "stack"
  list_percent: 0      # 0 = auto, 10-90 = fixed percentage
```

### Profiles

Each profile is a self-contained configuration. Profiles can point to different providers, different accounts, different filters, and different display preferences. Use `tsk -p <name>` to activate one, or set `default_profile` to use one automatically.

```yaml
profiles:
  work:
    provider: google
    credentials_file: ~/.config/tsk/work_credentials.json
    token_file: ~/.config/tsk/work_token.json
    primary_calendar: "user@company.com"
    days: 14
    smart_ooo: true
    subscribed: true
    ooo: true
    focus: false
    workloc: false
    accepted: true
    no_allday: false

    display:
      calendar: true
      time: true
      location: true
      meeting_link: true
      description: true
      status: true
      event_url: true
      attachments: false
      id: false
      in_progress: true

  outlook_work:
    provider: outlook
    client_id: "your-azure-app-client-id"
    tenant_id: "common"
    token_file: ~/.config/tsk/outlook_token.json
    days: 14
    smart_ooo: true

  today:
    provider: google
    credentials_file: ~/.config/tsk/work_credentials.json
    token_file: ~/.config/tsk/work_token.json
    days: 1
    no_allday: true
    ooo: false
    subscribed: false
    display:
      calendar: false
      description: false
      status: false
      event_url: false
      meeting_link: true
      in_progress: true
```

### Profile Settings Reference

All profile settings are optional. Anything not specified falls back to the global default.

**Provider & Auth:**

| Key | Default | Description |
|-----|---------|-------------|
| `provider` | `google` | `google` or `outlook` |
| `credentials_file` | `credentials.json` | Google OAuth credentials file path |
| `token_file` | `token.json` | Saved OAuth token file path |
| `client_id` | | Azure AD application client ID (Outlook) |
| `tenant_id` | `common` | Azure AD tenant ID (Outlook). Use `consumers` for personal Microsoft accounts |

**Filters:**

| Key | Default | Description |
|-----|---------|-------------|
| `days` | `7` | Number of days to fetch |
| `from` | | Start date (same formats as CLI) |
| `to` | | End date |
| `calendars` | | Comma-separated calendar name filter |
| `primary_calendar` | auto | Primary calendar for smart OOO |
| `ooo` | `true` | Include OOO events |
| `focus` | `false` | Include focus time |
| `workloc` | `false` | Include working location |
| `all_types` | `false` | Include all event types |
| `accepted` | `true` | Only accepted events |
| `subscribed` | `true` | Include subscribed calendars |
| `smart_ooo` | `false` | Hide events on OOO days |
| `no_allday` | `false` | Exclude all-day events |

**Display:**

Nested under `display:` in the profile. Controls which fields appear in event output.

| Key | Default | Description |
|-----|---------|-------------|
| `calendar` | `true` | Calendar name |
| `time` | `true` | Time and duration |
| `location` | `true` | Location |
| `meeting_link` | `true` | Meeting join link |
| `description` | `true` | Event description |
| `status` | `true` | Response status (accepted, declined, etc.) |
| `event_url` | `true` | Link to event in calendar app |
| `attachments` | `false` | Attached files |
| `id` | `false` | Event ID |
| `in_progress` | `true` | In-progress indicator with remaining time |

---

## Environment Variables

All config settings can be set via environment variables with the `TSK_` prefix. Underscores replace dots and hyphens.

```bash
TSK_DAYS=3 tsk
TSK_PROVIDER=outlook tsk
TSK_SMART_OOO=true tsk
```

---

## Precedence

Settings are resolved in this order (highest wins):

1. **CLI flags** — `tsk --days 3`
2. **Profile settings** — from the active profile in config
3. **Environment variables** — `TSK_DAYS=3`
4. **Global config** — top-level keys in `config.yaml`
5. **Built-in defaults** — `days: 7`, `provider: google`, etc.

---

## Provider Setup

Each provider has its own authentication setup. Once configured, the day-to-day usage is the same.

- **Google Calendar** — [Setup guide](google_setup.md)
- **Outlook / Office 365** — [Setup guide](outlook_setup.md)
