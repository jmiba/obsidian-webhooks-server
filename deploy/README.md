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
curl -o /tmp/caddy "https://cdn.caddyserver.com/api/download?os=darwin&arch=amd64"
chmod +x /tmp/caddy
sudo mv /tmp/caddy /usr/local/bin/caddy
caddy version

cp deploy/caddy/com.jmiba.caddy-webhooks.plist ~/Library/LaunchAgents/
launchctl load ~/Library/LaunchAgents/com.jmiba.caddy-webhooks.plist
tail -f /tmp/caddy-webhooks.err.log   # first run: watch the cert issuance
```

## 3. ddnss.de updater (on the Air)

Fill in credentials once (update host/path are shown in the ddnss.de
portal for your host):

```bash
mkdir -p ~/.config
cat > ~/.config/ddnss.env <<'EOF'
DDNSS_USER="your-username"
DDNSS_PASS="your-password"
DDNSS_UPDATE_HOST="update.ddnss.de"
DDNSS_UPDATE_PATH="/update"
EOF
chmod 600 ~/.config/ddnss.env

sh deploy/ddns/update-ddnss.sh     # must print GOOD or NOCHANGE

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
