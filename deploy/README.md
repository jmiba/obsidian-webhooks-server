# Deploy: remote access via ddnss.de (Big Sur Air)

Exposes the native server (port 8070) at **https://jmiba.ddnss.de** using
Caddy (auto TLS via Let's Encrypt) + a ddnss.de IP updater. No Docker needed.

## 1. Router

Port-forward on the home router:

| External | Internal |
|---|---|
| 443/tcp | Air's LAN IP : 443 |
| 80/tcp  | Air's LAN IP : 80 (needed once for Let's Encrypt) |

## 2. Caddy (on the Air)

```bash
# Intel Mac (MacBook Air 2014); on Apple Silicon use mac_arm64
# NOTE: Big Sur requires v2.9.x — Caddy >= 2.10 is built for macOS 12+
# (dyld error: Symbol not found: _SecTrustCopyCertificateChain)
curl -fsSL -o /tmp/caddy.tar.gz "https://github.com/caddyserver/caddy/releases/download/v2.9.1/caddy_2.9.1_mac_amd64.tar.gz"
tar -xzf /tmp/caddy.tar.gz -C /tmp
sudo mv /tmp/caddy /usr/local/bin/caddy
sudo xattr -d com.apple.quarantine /usr/local/bin/caddy 2>/dev/null || true
caddy version

cp deploy/caddy/com.jmiba.caddy-webhooks.plist ~/Library/LaunchAgents/
launchctl load ~/Library/LaunchAgents/com.jmiba.caddy-webhooks.plist
tail -f /tmp/caddy-webhooks.err.log   # first run: watch the cert issuance
```

## 3. ddnss.de updater (on the Air)

Fill in your ddnss.de account credentials once. The script uses the
ddnss.de "benutzerdefiniert" (custom) update endpoint
(`https://www.ddnss.de/upd.php?user=...&pwd=...&host=...&ip=...`):

```bash
mkdir -p ~/.config
cat > ~/.config/ddnss.env <<'EOF'
DDNSS_USER="your-username"
DDNSS_PASS="your-password"
EOF
chmod 600 ~/.config/ddnss.env

sh deploy/ddns/update-ddnss.sh     # prints the ddnss.de response
nslookup jmiba.ddnss.de 1.1.1.1    # verify the A record changed

cp deploy/ddns/com.jmiba.ddnss-updater.plist ~/Library/LaunchAgents/
launchctl load ~/Library/LaunchAgents/com.jmiba.ddnss-updater.plist
```

## 4. App config (.env, on the Air)

```dotenv
EXTERNAL_HOST=https://jmiba.ddnss.de
MAGIC_LINK_BASE_URL=https://jmiba.ddnss.de
COOKIE_SECURE=true
```

Restart the app (`Ctrl+C`, then `set -a; source .env; set +a && ./bin/obsidian-webhooks-server`).

## 5. Verify from the remote Mac

```bash
curl -sI https://jmiba.ddnss.de/health
```

Point the Obsidian plugin at `https://jmiba.ddnss.de`.
