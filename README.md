# tsk

A terminal calendar client for people who'd rather not deal with calendars.

**tsk** — part "task", part the sound you make when yet another meeting invite lands in your inbox.

![UI Screenshot](assets/tsk_ui.png)

---

It's a CLI tool that pulls events from Google Calendar and Outlook Calendar and shows them in your terminal. Because sometimes you just want to see what's eating your day without opening a browser, signing into three accounts, and getting distracted by 47 unread emails.

> **Look, don't touch.** tsk is *read-only* — it can see your calendar but can't create, modify, or delete anything. Your meetings are safe. (Whether that's a bug or a feature is up to you.)

Works with your primary calendar, shared calendars, subscribed calendars (holidays, team schedules), and all those calendars you forgot you subscribed to.

## Install

```bash
go install github.com/theakshaypant/tsk/cmd/tsk@latest
```

Make sure `$GOPATH/bin` is in your `PATH` — then `tsk` is ready to go.

## Quick Start

Authenticate with your calendar provider, then run it:

```bash
tsk auth                   # Google Calendar (default)
tsk -p outlook auth        # Outlook / Office 365
```

Provider setup guides: [Google](docs/google_setup.md) | [Outlook](docs/outlook_setup.md)

### What's next?

```bash
tsk next
```

Shows your next upcoming event with a countdown. If you've double-booked yourself, it catches the conflict and shows all concurrent events.

### Interactive mode

```bash
tsk ui
```

A proper TUI with an event list and detail panel, day-by-day navigation, a "NOW" marker that auto-scrolls to where you are, meeting link shortcuts, and merged duplicates across shared calendars.

## Documentation

Commands, flags, profiles, configuration, TUI keybindings, environment variables — it's all in the [CLI & Configuration Reference](docs/usage.md).

---

Still early days. Half-baked ideas and optimistic plans live in the [repo issues](https://github.com/theakshaypant/tsk/issues) — no promises, but they're fun to think about.
