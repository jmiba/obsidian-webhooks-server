#!/bin/sh
# Updates the ddnss.de A record for jmiba.ddnss.de using the standard
# dyndns v2 protocol (HTTPS).
#
# Credentials live in ~/.config/ddnss.env (chmod 600), NOT in this script.
# Create it once on the Air:
#
#   mkdir -p ~/.config
#   cat > ~/.config/ddnss.env <<'EOF'
#   DDNSS_USER="your-ddnss-username"
#   DDNSS_PASS="your-ddnss-password"
#   # Copy the exact update host + path from the ddnss.de portal
#   # (ip4.ddnss.de -> your host entry shows the update URL):
#   DDNSS_UPDATE_HOST="update.ddnss.de"
#   DDNSS_UPDATE_PATH="/update"
#   EOF
#   chmod 600 ~/.config/ddnss.env

set -eu

HOSTNAME="jmiba.ddnss.de"
CRED_FILE="$HOME/.config/ddnss.env"

[ -f "$CRED_FILE" ] || { echo "ddnss: missing $CRED_FILE" >&2; exit 1; }
. "$CRED_FILE"

[ -n "${DDNSS_UPDATE_HOST:-}" ] || { echo "ddnss: DDNSS_UPDATE_HOST not set in $CRED_FILE" >&2; exit 1; }
[ -n "${DDNSS_USER:-}" ] || { echo "ddnss: DDNSS_USER not set in $CRED_FILE" >&2; exit 1; }

# Current public IPv4
IP="$(curl -4 -s --max-time 15 https://ifconfig.me || true)"
[ -n "$IP" ] || { echo "ddnss: could not determine public IP" >&2; exit 1; }

RESPONSE="$(curl -s --max-time 20 \
	--user "$DDNSS_USER:$DDNSS_PASS" \
	"https://${DDNSS_UPDATE_HOST}${DDNSS_UPDATE_PATH:-/}?hostname=${HOSTNAME}&myip=${IP}")"

case "$RESPONSE" in
_GOOD)
	echo "ddnss: updated $HOSTNAME -> $IP"
	;;
NOCHANGE)
	echo "ddnss: $HOSTNAME unchanged ($IP)"
	;;
*)
	echo "ddnss: unexpected response: $RESPONSE" >&2
	exit 1
	;;
esac
