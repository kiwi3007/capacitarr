# Troubleshooting

This guide covers common issues you may encounter when running Capacitarr and how to resolve them.

## Integration Connection Failures

### Symptoms

- "Connection failed" when testing an integration
- Integration shows an error status on the dashboard
- Activity feed shows `integration_test_failed` events

### Common Causes

- **Wrong URL:** Ensure the URL includes the protocol (`http://` or `https://`) and port number. For example: `http://sonarr:8989`, not `sonarr:8989`.
- **Invalid API key:** Verify the API key matches the one in your \*arr app's Settings → General → API Key.
- **Network issues:** Ensure Capacitarr can reach the \*arr app. If both run in Docker, they must be on the same Docker network or use the correct container hostname.
- **SSL certificates:** If using HTTPS with self-signed certificates, the connection may fail due to certificate verification. Consider using HTTP for internal Docker networking.

### Diagnosis

1. Enable debug logging (Settings → Log Level → Debug)
2. Check the Activity Feed for `integration_test` and `integration_test_failed` events
3. Verify the URL is accessible from the Capacitarr container:

```bash
docker exec capacitarr wget -qO- http://sonarr:8989/api/v3/system/status?apikey=YOUR_KEY
```

4. On startup, Capacitarr runs a self-test against all enabled integrations — check the container logs for the results:

```bash
docker logs capacitarr | grep -i "integration"
```

## Disk Groups Not Appearing

### Symptoms

- Dashboard shows no disk groups
- Disk usage chart is empty
- No threshold configuration is available

### Common Causes

- **No integrations synced:** Disk groups are discovered from \*arr root folder paths during engine runs. If the engine hasn't run yet, no disk groups exist.
- **Integration not enabled:** Disabled integrations are skipped during sync — ensure at least one integration is enabled.
- **Root folder mismatch:** The \*arr app's root folder path must correspond to a real mount point that Capacitarr can detect via the \*arr disk space API.

### Resolution

1. Ensure at least one \*arr integration is enabled and connected (Settings → Integrations)
2. Trigger an engine run using the **Run Now** button on the dashboard
3. Check that your \*arr app has root folders configured (Settings → Media Management in Sonarr/Radarr)
4. Verify the Activity Feed shows `engine_complete` events — disk groups are populated as a side effect of engine runs

## Notifications Not Sending

### Symptoms

- Discord/Slack channels configured but no messages received
- Test notification works but cycle digests don't arrive
- In-app notifications appear but external channels are silent

### Common Causes

- **Channel not enabled:** Ensure the channel's "Enabled" toggle is on in Settings → Notifications
- **Subscription toggles off:** Each notification type has its own toggle per channel. Verify the relevant subscription is enabled (e.g., "Cycle Digest" for engine run summaries).
- **Invalid webhook URL:** The webhook URL may have been revoked or changed in Discord/Slack. Use the Test button to verify.
- **Rate limiting:** Discord and Slack may rate-limit rapid webhook calls. If many notifications fire in a short window, some may be delayed or dropped.

### Diagnosis

1. Use the **Test** button on the channel configuration to verify the webhook works
2. Check the Activity Feed for `notification_sent` or `notification_delivery_failed` events
3. Verify subscription toggles match the events you expect to receive
4. Check that in-app notifications are appearing (they are always-on) — if in-app shows the notification but the external channel doesn't, the issue is with the channel configuration or webhook

See [notifications.md](notifications.md) for the full notification setup guide.

## Engine Not Running

### Symptoms

- Dashboard shows no recent engine runs
- Items are not being evaluated or flagged
- Activity feed has no `engine_start` events

### Common Causes

- **No integrations:** The engine needs at least one enabled \*arr integration to run. Without integrations, there's nothing to evaluate.
- **Poll interval:** The engine runs on a schedule (default: every 5 minutes). If you just started Capacitarr, wait for the first poll cycle.
- **Previous run still active:** Only one engine run can execute at a time. If a previous run is still in progress, the next scheduled run will be skipped.

### Resolution

1. Verify at least one integration is connected (Settings → Integrations)
2. Use the **Run Now** button on the dashboard to trigger an immediate evaluation
3. Check the Activity Feed for `engine_start` and `engine_complete` events
4. Check container logs for errors:

```bash
docker logs capacitarr | grep -i "engine\|poller"
```

## Items Not Being Deleted

### Symptoms

- Engine runs show flagged items but nothing is deleted
- Audit log shows `dry_run` or `dry_delete` actions instead of `deleted`
- Freed bytes are always zero

### Common Causes

- **Dry-run mode:** The default execution mode is **dry-run**, which simulates deletions without actually removing files. This is a safety measure for new installations.
- **Deletions disabled:** The safety guard toggle ("Deletions Enabled") may be off. Even in auto mode, deletions won't execute if this toggle is disabled.
- **Approval mode:** In approval mode, flagged items are queued for manual review instead of being auto-deleted. Check the Approval Queue for pending items.

### Resolution

1. Check execution mode in Settings → Preferences → Execution Mode
2. If switching to auto mode, verify the "Deletions Enabled" safety guard is toggled on
3. If in approval mode, navigate to the dashboard Approval Queue and approve/reject pending items
4. After changing modes, trigger a **Run Now** to see the effect immediately

## SSE Connection Issues

### Symptoms

- Dashboard shows a "Disconnected" or "Reconnecting" banner
- Real-time updates are not appearing (activity feed stops updating)
- Manual page refreshes are required to see new data

### Common Causes

- **Reverse proxy buffering:** nginx and some other proxies buffer HTTP responses by default, which breaks Server-Sent Events streaming. See [deployment.md](deployment.md#sse-server-sent-events-proxy-configuration) for proxy configuration.
- **Proxy timeouts:** Long-lived SSE connections may be terminated by proxy read timeouts. Set `proxy_read_timeout` to a high value (86400s) for the SSE endpoint.
- **Cloudflare buffering:** Cloudflare's free plan buffers HTTP responses, causing SSE latency. Use DNS-only mode for the SSE path.

### Resolution

1. Check your reverse proxy configuration — ensure response buffering is disabled for `/api/v1/events`
2. The client automatically reconnects with exponential backoff — if the banner disappears on its own, the issue was transient
3. See the [SSE proxy configuration](deployment.md#sse-server-sent-events-proxy-configuration) section in the deployment guide

## Debug Logging

To enable verbose logging for diagnosis:

1. Navigate to **Settings** → **Log Level**
2. Set to **Debug**
3. Reproduce the issue
4. Check container logs:

```bash
docker logs capacitarr
```

5. For real-time log streaming:

```bash
docker logs -f capacitarr
```

6. Remember to set the log level back to **Info** after debugging — debug logs are verbose and will increase log volume significantly

Debug logging includes detailed information about:

- Integration API requests and responses
- Engine evaluation decisions and scoring
- Notification dispatch and delivery
- SSE connection lifecycle
- Database queries (when `DEBUG=true` environment variable is set)
