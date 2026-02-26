# tsk

A terminal calendar client for people who'd rather not deal with calendars.

**tsk** — part "task", part the sound you make when yet another meeting invite lands in your inbox.

---

It's a CLI tool that pulls events from Google Calendar and Outlook Calendar and shows them in your terminal. Because sometimes you just want to see what's eating your day without opening a browser, signing into three accounts, and getting distracted by 47 unread emails.

> **Look, don't touch.** tsk is *read-only*, it can see your calendar but can't create, modify, or delete anything. Your meetings are safe. (Whether that's a bug or a feature is up to you.)

Works with your primary calendar, shared calendars, subscribed calendars (holidays, team schedules), and all those calendars you forgot you subscribed to.

## Install

```bash
go install github.com/theakshaypant/tsk/cmd/tsk@latest
```

Make sure `$GOPATH/bin` is in your `PATH` — then `tsk` is ready to go.

## Calendar Providers

tsk supports multiple calendar providers. Set the `provider` field in your profile config (`google` or `outlook`) and authenticate.

### Google Calendar

Set up credentials via the Google Cloud Console, then:

```bash
tsk auth
```

This opens a browser, you click through Google's OAuth flow, and you're done. Tokens are saved locally so you don't have to do this dance every time.

If you're running this on a headless server or just hate browsers, there's a `--manual` flag that gives you a URL to paste instead.

[Google setup guide](docs/google_setup.md)

### Outlook Calendar

Register an app in Azure Portal, add your `client_id` and `tenant_id` to the profile config, then:

```bash
tsk -p outlook auth
```

Same browser OAuth flow, same local token storage. Works with personal Microsoft accounts, work/school accounts, or both — depending on how you registered the app.

[Outlook setup guide](docs/outlook_setup.md)

## Usage

```bash
# What's on the calendar?
tsk

# What's next? (countdown + conflict detection if you've double-booked)
tsk next

# Just the next 3 days
tsk --days 3

# This week
tsk --from monday --to friday

# Hide the OOO noise
tsk --ooo=false

# Skip all-day events
tsk --no-allday

# Smart mode: if you're OOO, hide everything else
tsk --smart-ooo

# What calendars do I even have?
tsk calendars
```

## Interactive Mode

For when you want to actually browse your calendar like a normal person:

```bash
tsk ui
```

This gives you a proper TUI with:
- Event list on the left, details on the right (collapses to single-panel on narrow terminals)
- Navigate with arrow keys, `←`/`→` to switch days
- A "NOW" marker so you know where you are in your day — auto-scrolls to it on load
- Jump to now with `t`, switch panels with `tab`
- Open meeting links with `Enter`, view event in calendar with `v`
- Past events dimmed out so you can focus on what's ahead
- Duplicate events across shared calendars merged into one, with per-calendar responses
- HTML descriptions rendered cleanly — links stay clickable
- Press `?` for the full shortcut list

![UI Screenshot](assets/tsk_ui.png)

## Profiles

Different views for different moods:

```bash
# Work account, all the details
tsk -p work

# Just today's meetings, minimal output
tsk -p today

# Only actual meetings, no focus time or OOO
tsk -p meetings
```

Set up profiles in `~/.config/tsk/config.yaml`. Each profile can have its own provider, credentials (for multiple accounts), filters, and display preferences. Mix and match Google and Outlook accounts.

```bash
# List profiles
tsk profile list

# Add a Google profile
tsk profile add work --provider google --credentials-file ~/.config/tsk/work_creds.json

# Add an Outlook profile
tsk profile add personal --provider outlook --client-id "your-azure-app-id" --tenant-id "your-tenant-id"

# See what a profile looks like
tsk profile show work
```

## Config

Copy `config.example.yaml` to `~/.config/tsk/config.yaml` and tweak to taste. Everything's optional — sensible defaults if you don't bother.

---

Still early days. More providers might show up eventually. iCloud, maybe. We'll see.
