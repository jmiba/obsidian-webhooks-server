# Default Settings & Smart Behaviour

This document describes all default settings and automatic behaviours in Obsidian Webhooks plugin v3.0.

## Overview

The plugin follows a **"Zero Configuration"** philosophy - it should work perfectly after entering just your Client Key. All other settings have smart defaults that cover 90% of use cases.

---

## Smart Defaults

### 🔌 Connection

| Setting | Default | Why |
|---------|---------|-----|
| **Server URL** | `https://obsidian-webhooks.khabaroff.studio` | Official hosted service |
| **Auto-reconnect** | Enabled | Automatically connect when plugin loads |
| **Reconnection delay** | Exponential backoff (1s → 60s) | Prevents server overload |
| **Max reconnect attempts** | 10 | Avoids infinite loops |

### 📝 File Operations

| Setting | Default | Why |
|---------|---------|-----|
| **Write mode** | Append | Most common use case (accumulating events) |
| **Auto-create folders** | Enabled | Prevents "parent folder not found" errors |
| **Newline separator** | Auto-detected | `\r\n` on Windows, `\n` on Unix/Mac |

**Examples:**

```typescript
// Newline behaviour
Platform.isWin → separator = "\r\n"
Platform.isMac → separator = "\n"
Platform.isLinux → separator = "\n"

// Folder creation
Webhook path: "notes/2024/january.md"
Creates: "notes/", "notes/2024/"
Then writes: "notes/2024/january.md"
```

### 🔄 Polling & Synchronization

| Setting | Default | Why |
|---------|---------|-----|
| **Enable polling** | Always enabled | Fallback when SSE unavailable |
| **Polling interval** | 5 seconds | Balance between latency and server load |
| **Offline sync** | Automatic | Catches missed events when reconnecting |

**How it works:**

```
1. Plugin attempts SSE connection (real-time)
   ↓
2. If SSE works → use SSE (instant delivery)
   ↓
3. If SSE fails → fallback to polling (5s intervals)
   ↓
4. When reconnecting → fetch missed events first
```

### 🔔 Notifications

| Setting | Default | Why |
|---------|---------|-----|
| **Show notifications** | Always enabled | Provides user feedback |
| **Notification duration** | 4 seconds (Obsidian default) | Standard Notice behaviour |

**Examples:**

```typescript
✅ "Webhook event processed: /notes/example.md"
❌ "Error processing webhook: File path invalid"
```

### 🐛 Debugging

| Setting | Default | Why |
|---------|---------|-----|
| **Debug logging** | Disabled | Reduces console noise for normal users |
| **Log prefix** | `[Obsidian Webhooks]` | Easy to filter in DevTools |

**When to enable:**
- Troubleshooting connection issues
- Investigating webhook processing errors
- Reporting bugs to support

---

## Behaviour Details

### Newline Separator Logic

**Auto-detection:**

```typescript
function getNewlineSeparator(): string {
  // Platform.isWin checks for Windows OS
  if (Platform.isWin) {
    return "\r\n"; // Windows-style (CRLF)
  }
  return "\n"; // Unix/Mac-style (LF)
}
```

**Effect on file content:**

```markdown
# Without separator (legacy behaviour):
Event 1Event 2Event 3

# With auto-detected separator:
Event 1
Event 2
Event 3
```

### Polling Fallback

**When activated:**
- SSE connection fails (network issue, firewall, etc.)
- Browser/Electron doesn't support EventSource
- Server returns non-200 status for SSE endpoint

**Polling flow:**

```
Every 5 seconds:
  GET /events/{client_key}?poll=true
  ↓
  Receive events array
  ↓
  Process each event
  ↓
  Send ACK for each processed event
  ↓
  Wait 5 seconds → repeat
```

### Folder Auto-Creation

**Path parsing:**

```typescript
// Input
path: "notes/work/2024/project.md"

// Creates folders
"notes/"
"notes/work/"
"notes/work/2024/"

// Then writes file
"notes/work/2024/project.md"
```

**Error handling:**

```typescript
// Without auto-create:
Error: "Parent folder 'notes/work/2024' does not exist"
User must manually create folders ❌

// With auto-create:
Folders created automatically
File written successfully ✅
```

### Payload Formatting

Webhook bodies remain raw in `create`, `append`, and `overwrite` modes. The
plugin parses YAML or JSON only in `frontmatter` mode.

---

## Configuration Override

While smart defaults work for most users, advanced users can override them:

### Option 1: Advanced Settings Panel

Expand "Advanced Settings" in plugin settings to access:
- Server URL (for self-hosted setups)
- Debug logging toggle
- Connection statistics

### Option 2: Manual settings.json Edit

Located at: `.obsidian/plugins/obsidian-webhooks/data.json`

```json
{
  "serverUrl": "https://obsidian-webhooks.khabaroff.studio",
  "clientKey": "ck_...",
  "autoConnect": true,
  "debugLogging": false,

  // These are kept for backwards compatibility
  // but overridden by smart defaults:
  "enablePolling": true,        // Always true
  "pollingInterval": 5,         // Always 5
  "showNotifications": true,    // Always true
  "autoCreateFolders": true,    // Always true
  "defaultMode": "append",      // Always append
  "newlineType": "unix"         // Auto-detected
}
```

**Note:** Editing `enablePolling`, `pollingInterval`, `showNotifications`, `autoCreateFolders`, `defaultMode`, or `newlineType` will have no effect - they are overridden by smart defaults.

---

## Migration from v2.0

Users upgrading from v2.0 will notice:

### Settings Removed from UI

| Old Setting | v3.0 Behaviour |
|-------------|----------------|
| Enable polling | Always enabled (5s interval) |
| Newline type | Auto-detected by OS |
| Show notifications | Always shown |
| Auto-create folders | Always enabled |
| Default mode | Always "append" |
| Connect/Disconnect buttons | Replaced by "Test Connection" |

### Settings Preserved

| Setting | Location |
|---------|----------|
| Server URL | Advanced Settings |
| Auto-reconnect | Advanced Settings |
| Debug logging | Advanced Settings |

### Your Data

✅ **All saved settings are preserved** in `data.json`
✅ **No data loss** - old configuration still loads
⚠️ **Some settings are ignored** - smart defaults override them
ℹ️ **No action needed** - migration is automatic

---

## FAQ

**Q: Can I disable polling?**
A: No. Polling is essential as a fallback mechanism when SSE is unavailable. It's lightweight (5s interval) and doesn't impact performance.

**Q: Can I change newline type?**
A: No. The plugin auto-detects the correct newline for your OS. This prevents cross-platform issues.

**Q: Can I disable notifications?**
A: No. Notifications provide important feedback when webhooks are processed. They appear for 4 seconds and don't interrupt your workflow.

**Q: Can I change write mode to "overwrite"?**
A: Yes. Expand "Advanced Settings" and select "Overwrite file" from the Write mode dropdown. By default, the plugin uses "Append to end" mode, which is the most common use case for webhook logs.

**Q: Can I update only YAML frontmatter?**
A: Yes. Select "Update YAML frontmatter" from the Write mode dropdown. Send a complete frontmatter block, a YAML field mapping, or a JSON object. Matching fields are overwritten, missing fields are created, and the note body is preserved.

**Q: Can each webhook choose its own write mode?**
A: Yes. Add `mode=create`, `mode=append`, `mode=overwrite`, or `mode=frontmatter` to the webhook URL. This overrides the configured default for that event. Create mode fails if the note already exists.

**Q: Why can't I disable auto-create folders?**
A: Disabling it would cause "parent folder not found" errors. There's no benefit to manual folder creation - it's always safe to enable.

---

## Technical Details

### Platform Detection

```typescript
import { Platform } from 'obsidian';

Platform.isWin      // Windows
Platform.isMac      // macOS
Platform.isLinux    // Linux
Platform.isIos      // iOS (Obsidian Mobile)
Platform.isAndroidApp // Android (Obsidian Mobile)
```

### SSE vs Polling Decision Tree

```
Plugin starts
  ↓
Try SSE: EventSource(server/events/{key})
  ↓
  ├─ Success → Use SSE (real-time)
  │   └─ On disconnect → try SSE again
  │       └─ After 3 fails → switch to polling
  │
  └─ Failure → Use polling (5s interval)
      └─ Retry SSE every 60s
          └─ If SSE works → switch back to SSE
```

### Event Processing Pipeline

```
Webhook arrives at server
  ↓
Server stores in database
  ↓
Server sends via SSE (if connected)
  │
  └─ Plugin receives event
      ↓
      Check if already processed (duplicate detection)
      ↓
      Parse metadata only when mode is frontmatter
      ↓
      Auto-create parent folders
      ↓
      Write to file (append mode)
      ↓
      Send ACK to server
      ↓
      Show notification ✅
```

---

## Support

- 📖 Documentation: https://obsidian-webhooks.khabaroff.studio
- 🐛 Issues: https://github.com/jmiba/obsidian-webhooks-server/issues
- 📧 Email: support@khabaroff.studio

---

**Version:** 3.0.0
**Last Updated:** 2026-02-01
