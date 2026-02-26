# Outlook / Office 365 Calendar Setup Guide

To allow the `tsk` CLI to fetch your Outlook calendar events, you must register an application in Azure AD (Microsoft Entra ID) and generate a client ID. This is the Outlook equivalent of the Google `credentials.json` flow.

> **tsk only needs read access.** It requests the `Calendars.Read` scope — it can look at your events but can't create, modify, or delete anything.

### Prerequisites: You Need an Azure AD Tenant

Azure requires a **directory (tenant)** to register apps. If you have a **work or school** Microsoft account, your organization already has one — you're all set, skip to Phase 1.

If you have a **personal** Microsoft account (@outlook.com, @hotmail.com, @live.com), you'll see this error in the Azure Portal:

> *"The ability to create applications outside of a directory has been deprecated."*

You need to get a directory first. Pick **one** of these options:

#### Option A: Sign Up for a Free Azure Account (Fastest)

1. Go to **[azure.microsoft.com/free](https://azure.microsoft.com/en-us/pricing/purchase-options/azure-account)** and click **Start free** / **Try Azure for free**.
2. Sign in with your personal Microsoft account.
3. Complete the signup (requires a credit card for verification, but you won't be charged).
4. This **automatically creates an Azure AD tenant** tied to your account.
5. You're now ready — go to Phase 1.

#### Option B: M365 Developer Program Sandbox

1. Go to the **[M365 Developer Program dashboard](https://developer.microsoft.com/en-us/microsoft-365/dev-program)**.
2. Sign in and click **Set up E5 subscription** (or **Go to dashboard** if you already joined).
3. Follow the prompts to provision a sandbox tenant — you'll get a new admin account like `admin@yourname.onmicrosoft.com`.
4. **Important:** The sandbox can take up to a few hours to fully provision. If it says "We're setting things up", wait and check back.
5. Once provisioned, sign in to the **[Azure Portal](https://portal.azure.com/)** with the **sandbox admin account** (not your personal account).
6. You're now ready — go to Phase 1.

> **Note:** If your M365 Developer Program admin email is the same as your personal @outlook.com email, use `tenant_id: "consumers"` in your config (see Phase 2).

#### Option C: Create an Azure AD Tenant Directly

1. Go to **[Azure Portal — Create a tenant](https://portal.azure.com/#create/Microsoft.AzureActiveDirectory)**.
2. Select **Azure Active Directory** and click **Next**.
3. Fill in an organization name (e.g., `tsk-dev`) and initial domain (e.g., `tskdev.onmicrosoft.com`).
4. Click **Review + create**, then **Create**.
5. Switch to the new tenant (click your profile icon in the top-right > **Switch directory**).
6. You're now ready — go to Phase 1.

---

### Phase 1: Register an Application

1. Go to the **[Azure Portal — App registrations](https://portal.azure.com/#view/Microsoft_AAD_RegisteredApps/ApplicationsListBlade)**.
   - Sign in with the account that has an Azure AD tenant (see Prerequisites above).
2. Click **+ New registration**.
3. Fill in the details:
   - **Name:** `tsk-cli` (or whatever you prefer).
   - **Supported account types:**
     - Choose **"Accounts in any organizational directory and personal Microsoft accounts"** if you want it to work with any Microsoft account (recommended).
     - Choose **"Accounts in this organizational directory only"** if you only need your work/school account.
   - **Redirect URI:** Select **"Public client/native (mobile & desktop)"** from the dropdown, then enter:
     ```
     http://localhost:8085/callback
     ```

   > **⚠️ Important:** You **cannot change** the "Supported account types" after creation. If you need to switch (e.g., to add personal account support), you must create a new app registration.

4. Click **Register**.

### Phase 2: Copy Your Client ID and Tenant ID

1. After registration, you'll land on the app's **Overview** page.
2. Copy the **Application (client) ID** — this is your `client_id`.
   - It looks like: `a1b2c3d4-e5f6-7890-abcd-ef1234567890`
3. Copy the **Directory (tenant) ID** — this is your `tenant_id`.
   - It looks like: `f1e2d3c4-b5a6-7890-abcd-ef1234567890`

> **That's it for credentials.** Unlike Google, Outlook doesn't require a separate credentials file or client secret for public (desktop) apps. You just need the client ID and tenant ID.

### Phase 3: Verify API Permissions (Usually Automatic)

The app should already have the right permissions, but let's confirm:

1. In your app registration, go to **API permissions** in the left sidebar.
2. You should see **Microsoft Graph** with **User.Read** listed.
3. If `Calendars.Read` isn't listed, that's fine — tsk requests it dynamically during login. But if you want to add it explicitly:
   - Click **+ Add a permission**.
   - Select **Microsoft Graph** > **Delegated permissions**.
   - Search for `Calendars.Read` and check it.
   - Click **Add permissions**.

> **No admin consent required.** `Calendars.Read` and `User.Read` are user-consentable permissions — no tenant admin needs to approve anything.

---

## Using with tsk

### 1. Create a config file

Add an Outlook profile to `~/.config/tsk/config.yaml`:

```yaml
default_profile: outlook_work

profiles:
  outlook_work:
    provider: outlook
    client_id: "a1b2c3d4-e5f6-7890-abcd-ef1234567890"   # from Phase 2
    tenant_id: "f1e2d3c4-b5a6-7890-abcd-ef1234567890"   # from Phase 2 — see tenant_id cheat sheet below
    token_file: ~/.config/tsk/outlook_token.json
```

Or use the CLI:

```bash
tsk profile add outlook_work \
  --provider outlook \
  --client-id "a1b2c3d4-e5f6-7890-abcd-ef1234567890" \
  --tenant-id "f1e2d3c4-b5a6-7890-abcd-ef1234567890" \
  --token-file ~/.config/tsk/outlook_token.json
```

### 2. Authenticate

```bash
tsk -p outlook_work auth
```

This opens your browser for Microsoft login. Sign in and grant `tsk-cli` permission to read your calendar. Once done, tsk saves a token and you won't need to do this again unless the token expires or you revoke access.

### 3. You're in

```bash
tsk -p outlook_work next          # what's coming up
tsk -p outlook_work calendars     # see your calendars
tsk -p outlook_work ui            # the full interface
```

If you set `outlook_work` as your default profile, you can drop the `-p` flag:

```bash
tsk profile default outlook_work
tsk ui
```

---

### Tenant ID Cheat Sheet

| Scenario | `tenant_id` value |
|---|---|
| **Work/school account** | Your Directory (tenant) ID — a UUID from the Azure Portal app overview page |
| **Personal account (@outlook.com, @hotmail.com, etc.)** | `consumers` |
| **M365 Developer Program (admin email = personal email)** | `consumers` |
| Works with any Microsoft account (personal + work/school) | `common` — only works if app is registered as multi-tenant |
| Only work/school accounts from any organization | `organizations` |

> **Rule of thumb:**
> - **Personal Microsoft account?** Use `consumers`.
> - **Work/school account?** Use the tenant UUID from the Azure Portal.
> - **Not sure?** Try `consumers` first. If that doesn't work, use the UUID.

---

### Adding More Accounts

Want both a work and personal Outlook account? Add separate profiles — each gets its own token:

```yaml
profiles:
  outlook_work:
    provider: outlook
    client_id: "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
    tenant_id: "f1e2d3c4-b5a6-7890-abcd-ef1234567890"   # your org's tenant ID
    token_file: ~/.config/tsk/outlook_work_token.json

  outlook_personal:
    provider: outlook
    client_id: "a1b2c3d4-e5f6-7890-abcd-ef1234567890"   # same app works
    tenant_id: "consumers"
    token_file: ~/.config/tsk/outlook_personal_token.json
```

Then authenticate each:

```bash
tsk -p outlook_work auth
tsk -p outlook_personal auth
```

You can reuse the same Azure AD app registration for multiple accounts — just use different token files.

---

### Mixing Google and Outlook

tsk supports both providers side by side. Each profile is independent:

```yaml
profiles:
  google_work:
    provider: google
    credentials_file: ~/.config/tsk/google_credentials.json
    token_file: ~/.config/tsk/google_token.json

  outlook_work:
    provider: outlook
    client_id: "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
    tenant_id: "consumers"
    token_file: ~/.config/tsk/outlook_token.json
```

```bash
tsk -p google_work ui      # Google calendar
tsk -p outlook_work ui     # Outlook calendar
```

---

## Troubleshooting

- **"unauthorized_client: The client does not exist or is not enabled for consumers":**
  This is almost always a **wrong `tenant_id`** in your config. The fix:
  1. If you're using a **personal Microsoft account** (@outlook.com, @hotmail.com, etc.), set `tenant_id: "consumers"`.
  2. If you're using a **work/school account**, set `tenant_id` to the actual Directory (tenant) ID UUID from your app's Overview page in the Azure Portal.
  3. Delete your token file and re-authenticate:
     ```bash
     rm ~/.config/tsk/outlook_token.json
     tsk -p outlook_work auth
     ```

- **401 Unauthorized with empty response body:**
  This usually means the account you authenticated with **doesn't have an Exchange Online mailbox** (calendar API calls require one). Common causes:
  - M365 Developer Program sandbox not fully provisioned — go to [admin.microsoft.com → Users](https://admin.microsoft.com/Adminportal/Home#/users), click your user, and ensure **Exchange Online** is enabled under Licenses.
  - You authenticated with the developer tenant admin account instead of your personal account — delete the token and re-authenticate, making sure to pick the right account.
  - Your `tenant_id` doesn't match the account you signed in with.

- **"The ability to create applications outside of a directory has been deprecated":**
  Your Microsoft account doesn't have an Azure AD tenant. See the **Prerequisites** section at the top — the fastest fix is signing up for a [free Azure account](https://azure.microsoft.com/en-us/pricing/purchase-options/azure-account), which creates a tenant automatically.

- **"Property api.requestedAccessTokenVersion is invalid":**
  This happens when you try to change "Supported account types" on an existing app registration — Azure doesn't allow it after creation. **Create a new app registration** from scratch, selecting the correct account type during initial setup.

- **"AADSTS7000218: The request body must contain the following parameter: client_assertion or client_secret":**
  Your app is registered as a **confidential client** instead of a public client. Go to your app registration > **Authentication** > ensure **"Allow public client flows"** is set to **Yes** at the bottom.

- **"AADSTS50011: The redirect URI specified in the request does not match":**
  Make sure the redirect URI in your app registration is exactly `http://localhost:8085/callback` and the platform is **"Public client/native (mobile & desktop)"**.

- **"AADSTS65001: The user or administrator has not consented to use the application":**
  This can happen with strict tenant policies. Ask your IT admin to grant consent, or try with `tenant_id: "common"`.

- **"Insufficient privileges to complete the operation":**
  Your organization may block third-party apps from reading calendar data. Contact your IT admin.

- **Token expired / login loop:**
  Delete the token file and re-authenticate:
  ```bash
  rm ~/.config/tsk/outlook_token.json
  tsk -p outlook_work auth
  ```
