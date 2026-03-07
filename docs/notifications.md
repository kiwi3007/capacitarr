# Notifications

Capacitarr provides real-time notifications through Discord webhooks, Slack webhooks, and a built-in in-app notification center. Notifications keep you informed about engine activity, disk usage alerts, and system events without needing to check the dashboard.

## Notification Types

### Cycle Digests

A cycle digest is a single summary notification sent after each engine run completes. It includes the full picture of what happened during the cycle — how many items were evaluated, how many were flagged, what was deleted, and how much space was freed.

Digest content varies by execution mode:

| Mode | Title | Summary |
|------|-------|---------|
| **Auto** | 🧹 Cleanup Complete | `Deleted X of Y evaluated items in Z.Zs, freeing N.N GB` |
| **Dry-run** | 🔍 Dry-Run Complete | `Flagged X of Y items in Z.Zs — Would free N.N GB` |
| **Approval** | 📋 Items Queued for Approval | `Queued X of Y items in Z.Zs — Potential N.N GB` |
| *(no action needed)* | ✅ All Clear | `Evaluated X items — no action needed` |

Auto-mode digests also include a disk usage progress bar showing the before/after usage percentage and target. If a newer Capacitarr version is available, a version banner is appended to the digest.

Digests use a **two-gate flush** internally — the notification waits until both the evaluation phase and the deletion phase of the engine run are complete before sending. This ensures deletion results (freed bytes, failure count) are included in the summary.

### Instant Alerts

Instant alerts fire immediately when their trigger event occurs — they are not batched or delayed. Each alert type covers a specific operational event:

| Alert Type | Description |
|------------|-------------|
| **Engine Error** | The evaluation engine encountered an error during a run |
| **Mode Changed** | The execution mode was switched (e.g., dry-run → auto) |
| **Server Started** | Capacitarr has started and is ready to accept requests |
| **Threshold Breached** | Disk usage has exceeded the configured threshold for a disk group |
| **Update Available** | A newer Capacitarr release was detected on GitLab |
| **Approval Activity** | An item was approved or rejected in the approval queue |

## In-App Notifications

In-app notifications are always active — no configuration is needed. Every cycle digest and instant alert is automatically recorded to the in-app notification store, regardless of whether you have configured any Discord or Slack channels.

In-app notifications appear in the **notification bell** (🔔) in the UI header bar. Key features:

- **Always-on:** Works out of the box with no setup required
- **Notification badge:** Shows the unread count on the bell icon
- **Mark as read:** Click individual notifications or use "Mark all as read"
- **Clear all:** Remove all notifications from the panel
- **Auto-pruning:** Notifications are automatically cleaned up based on the audit log retention setting (default: 90 days)

In-app notifications mirror the same content sent to external channels — digests show the engine run summary, and alerts show the event title and message.

## Discord Setup

### Step 1: Create a Webhook

1. Open your Discord server and navigate to the channel where you want notifications
2. Click the **gear icon** (⚙️) next to the channel name to open Channel Settings
3. Select **Integrations** from the sidebar
4. Click **Webhooks** → **New Webhook**
5. Give the webhook a name (e.g., "Capacitarr") and optionally set an avatar
6. Click **Copy Webhook URL** — you'll need this in the next step
7. Click **Save Changes**

### Step 2: Add Channel in Capacitarr

1. Navigate to **Settings** → **Notifications**
2. Click **Add Channel**
3. Select **Discord** as the channel type
4. Paste the webhook URL you copied from Discord
5. Give the channel a descriptive name (e.g., "Media Alerts")
6. Click **Save**

### Step 3: Configure Subscriptions

After saving the channel, configure which notifications it receives using the subscription toggles. Each toggle controls a specific category of events — see the [Subscription Toggles](#subscription-toggles) section below.

Use the **Test** button to verify the webhook is working. A test notification will appear in your Discord channel.

## Slack Setup

### Step 1: Create an Incoming Webhook

1. Go to [api.slack.com/apps](https://api.slack.com/apps) and click **Create New App**
2. Choose **From scratch**, give it a name (e.g., "Capacitarr"), and select your workspace
3. In the app settings, navigate to **Incoming Webhooks** and toggle it **On**
4. Click **Add New Webhook to Workspace**
5. Select the channel where notifications should be posted
6. Click **Allow** — Slack will show you the webhook URL
7. Copy the webhook URL (starts with `https://hooks.slack.com/services/...`)

### Step 2: Add Channel in Capacitarr

1. Navigate to **Settings** → **Notifications**
2. Click **Add Channel**
3. Select **Slack** as the channel type
4. Paste the webhook URL you copied from Slack
5. Give the channel a descriptive name (e.g., "Disk Alerts")
6. Click **Save**

### Step 3: Configure Subscriptions

After saving the channel, configure which notifications it receives using the subscription toggles. Each toggle controls a specific category of events — see the [Subscription Toggles](#subscription-toggles) section below.

Use the **Test** button to verify the webhook is working. A test notification will appear in your Slack channel.

## Subscription Toggles

Each external notification channel (Discord or Slack) has independent subscription toggles. You can enable or disable each event category per channel, allowing you to route different notification types to different channels.

| Toggle | Events | Description |
|--------|--------|-------------|
| **Cycle Digest** | Engine complete + deletion batch | Summary of each engine run with stats and disk usage |
| **Error** | Engine errors | Fires when the evaluation engine fails during a run |
| **Mode Changed** | Execution mode switch | Fires when switching between dry-run, approval, and auto |
| **Server Started** | Application startup | Confirms Capacitarr is online after a restart |
| **Threshold Breach** | Disk usage exceeds threshold | Immediate alert when a disk group exceeds its threshold |
| **Update Available** | New version detected | Fires when a newer Capacitarr release exists on GitLab |
| **Approval Activity** | Items approved or rejected | Fires when approval queue items are approved or rejected |

> **Tip:** In-app notifications receive all event types automatically — these toggles only control external channel delivery.

## Digest Format

Cycle digest notifications are rendered as rich embeds in Discord and rich message blocks in Slack. Here's what a typical auto-mode digest looks like:

```
⚡ Capacitarr v1.0.0 • auto
─────────────────────────────
🧹 Cleanup Complete

Deleted 12 of 97 evaluated items
in 3.2s, freeing 48.3 GB

▓▓▓▓▓▓▓▓▓▓▓▓▓▓░░░░░░ 72% → 65%

📦 v1.1.0 available!
```

Digest components:

- **Author line:** Shows the Capacitarr version and current execution mode
- **Title:** Mode-specific title (🧹 Cleanup Complete, 🔍 Dry-Run Complete, 📋 Items Queued, or ✅ All Clear)
- **Description:** Item counts, duration, and freed/potential space
- **Progress bar:** Visual disk usage indicator (auto mode and all-clear only) showing current percentage and target
- **Version banner:** Appears when a newer release is available (optional)

Alert notifications use a similar embed format with a title, message, and color-coded severity (green for success, blue for info, amber for attention, red for errors).
