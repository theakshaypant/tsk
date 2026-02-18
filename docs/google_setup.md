# Google Calendar API Setup Guide

To allow the `tsk` CLI to fetch your events, you must create a Google Cloud Project and generate OAuth 2.0 credentials. This process produces the `credentials.json` file required by the adapter.

> **tsk only needs read access.** It requests the `calendar.readonly` scope, it can look at your events but can't touch them. No surprise meetings will be created on your behalf.

### Phase 1: Create a Project

1. Go to the **[Google Cloud Console](https://console.cloud.google.com/)**.
2. Click the project dropdown in the top-left header (next to the Google Cloud logo).
3. Click **New Project**.
4. **Project Name:** `tsk-cli` (or any name you prefer).
5. Click **Create**.
6. **Important:** Wait for the notification, then click **Select Project** to switch to your new project.

### Phase 2: Enable the Calendar API

1. Open the sidebar (☰) and go to **APIs & Services** > **Library**.
2. Search for: `Google Calendar API`.
3. Click on the result card.
4. Click **Enable**.

### Phase 3: Configure the OAuth Consent Screen

*This defines what the user sees when logging in. Since this is a personal tool, we will use "External" mode with "Testing" status.*

1. Go to **APIs & Services** > **OAuth consent screen**.
2. **User Type:** Select **External**.
3. Click **Create**.
4. **App Information:**
* **App Name:** `tsk-cli`
* **User support email:** Select your email.
* **Developer contact info:** Enter your email.


5. Click **Save and Continue** (skip "Scopes" for now, we request them in code).
6. **Test Users (Crucial Step):**
* Click **+ Add Users**.
* Enter the exact Google email address you intend to use with the tool.
* *Note: While the app is in "Testing" mode, only emails added here can log in.*


7. Click **Save and Continue** until you finish.

### Phase 4: Generate Credentials

1. Go to **APIs & Services** > **Credentials**.
2. Click **+ Create Credentials** at the top.
3. Select **OAuth client ID**.
4. **Application type:** Select **Desktop app**.
* *Note: If you don't see this, ensure you are in the specific project, not the aggregate view.*


5. **Name:** `tsk-desktop-client` (default is fine).
6. Click **Create**.

### Phase 5: Download the Credentials

1. A popup will appear saying "OAuth client created".
2. Click the **Download JSON** button (looks like a downward arrow ⬇️).
3. Rename the downloaded file to **`credentials.json`**.
4. Move it somewhere safe — you'll tell tsk where to find it.

---

## Using with tsk

Now for the fun part.

### 1. Put your credentials somewhere

A common spot is `~/.config/tsk/`:

```bash
mkdir -p ~/.config/tsk
mv ~/Downloads/credentials.json ~/.config/tsk/
```

### 2. Create a config file

Create `~/.config/tsk/config.yaml`:

```yaml
default_profile: work

profiles:
  work:
    credentials_file: ~/.config/tsk/credentials.json
    token_file: ~/.config/tsk/token.json
```

### 3. Authenticate

```bash
tsk auth
```

This opens your browser for Google login. You'll see the "unverified app" warning — that's fine, it's your app. Click through to authorize.

Once done, tsk saves a token file and you won't need to do this again (unless the token expires or you revoke access).

### 4. You're in

```bash
tsk next          # what's coming up
tsk calendars     # see your calendars
tsk tui           # the full interface
```

---

### Adding more accounts

Want a personal calendar too? Add another profile:

```yaml
profiles:
  work:
    credentials_file: ~/.config/tsk/credentials.json
    token_file: ~/.config/tsk/token.json
  personal:
    credentials_file: ~/.config/tsk/personal_credentials.json
    token_file: ~/.config/tsk/personal_token.json
```

Then authenticate and use it:

```bash
tsk auth --profile personal
tsk next --profile personal
```

You'll need separate credentials for each Google account (repeat the Cloud Console setup above).

---

## Troubleshooting

* **Error 403: Access_denied:** This usually means you forgot to add your email to the **Test Users** list in Phase 3.
* **"Google hasn't verified this app":** This is normal. Since you wrote the app and it's in Testing mode, click **Continue** (or "Advanced" > "Go to tsk-cli (unsafe)") to proceed.