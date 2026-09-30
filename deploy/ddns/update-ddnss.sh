#!/bin/sh
# Updates the DNS record for jmiba.ddnss.de using the ddnss.de
# "benutzerdefiniert" (custom) update endpoint:
#
#   https://www.ddnss.de/upd.php?user=<user>&pwd=<pass>&host=<domain>&ip=<ip>&ip6=<ip6>
#
# Credentials live in ~/.config/ddnss.env (chmod 600), NOT in this script.
# Create it once on the Air:
#
#   mkdir -p ~/.config
#   cat > ~/.config/ddnss.env <<'EOF'
#   DDNSS_USER="your-ddnss-username"
#   DDNSS_PASS="your-ddnss-password"
#   EOF
#   chmod 600 ~/.config/ddnss.env

set -eu

HOSTNAME="jmiba.ddnss.de"
CRED_FILE="$HOME/.config/ddnss.env"

[ -f "$CRED_FILE" ] || { echo "ddnss: missing $CRED_FILE" >&2; exit 1; }
. "$CRED_FILE"

[ -n "${DDNSS_USER:-}" ] || { echo "ddnss: DDNSS_USER not set in $CRED_FILE" >&2; exit 1; }
[ -n "${DDNSS_PASS:-}" ] || { echo "ddnss: DDNSS_PASS not set in $CRED_FILE" >&2; exit 1; }

# Current public IPv4 (required) and IPv6 (optional, kept fresh too)
IP4="$(curl -4 -s --max-time 15 https://ifconfig.me || true)"
[ -n "$IP4" ] || { echo "ddnss: could not determine public IPv4" >&2; exit 1; }
IP6="$(curl -6 -s --max-time 15 https://ifconfig.me || true)"

RESPONSE="$(curl -s --max-time 20 \
	"https://www.ddnss.de/upd.php?user=${DDNSS_USER}&pwd=${DDNSS_PASS}&host=${HOSTNAME}&ip=${IP4}&ip6=${IP6}")"

echo "ddnss: $HOSTNAME ip=${IP4} ip6=${IP6:-<none>} -> ${RESPONSE}"
