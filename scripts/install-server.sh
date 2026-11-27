#!/bin/sh
# Kotel server for a Linux computer without a screen, such as a VPS
# (T-036, D-138). One command installs or updates it:
#
#   curl -fsSL https://github.com/kolotiloff/agent-messenger/releases/latest/download/install-server.sh | sh
#
# It downloads the program of the same release, checks its SHA-256, runs it
# as the systemd service agent-messenger, turns the server on for the
# internet with this computer's public address and prints an invitation.
# Running it again updates the program and keeps the data and the devices.
#
# Optional settings: AM_PUBLIC (external address, when the network card has
# no public one), AM_PORT (default 47100), AM_VERSION (e.g. 0.0.0-preview.77).
set -eu

REPO="kolotiloff/agent-messenger"
DIR="/opt/agent-messenger"
DATA="/var/lib/agent-messenger"
BIN="$DIR/messenger"
CMD="/usr/local/bin/agent-messenger-server"
UNIT="/etc/systemd/system/agent-messenger.service"
PORT="${AM_PORT:-47100}"

say() { printf '%s\n' "$*"; }
fail() { printf 'Ошибка: %s\n' "$*" >&2; exit 1; }

[ "$(id -u)" = 0 ] || fail "запустите от root (sudo sh)."
[ "$(uname -s)" = Linux ] || fail "нужен Linux."
command -v systemctl >/dev/null 2>&1 || fail "нужен systemd."
command -v curl >/dev/null 2>&1 || fail "нужен curl (apt install curl)."
command -v sha256sum >/dev/null 2>&1 || fail "нужен sha256sum (coreutils)."
case "$PORT" in ''|*[!0-9]*) fail "AM_PORT должен быть числом." ;; esac

case "$(uname -m)" in
  x86_64|amd64) ASSET="AgentMessenger-Linux-x64" ;;
  aarch64|arm64) ASSET="AgentMessenger-Linux-ARM64" ;;
  *) fail "процессор $(uname -m) не поддерживается." ;;
esac
if [ -n "${AM_VERSION:-}" ]; then
  BASE="https://github.com/$REPO/releases/download/v$AM_VERSION"
else
  BASE="https://github.com/$REPO/releases/latest/download"
fi

# Another Kotel client of this user (started with `start`) holds the
# client lock; the service would not start next to it. It is not stopped
# here: its owner decides.
if ! systemctl is-active --quiet agent-messenger 2>/dev/null; then
  other=$(pgrep -af '_background-serve|messenger.* serve' 2>/dev/null | grep -v ' server run' || true)
  if [ -n "$other" ]; then
    say "На этом компьютере уже работает другой клиент Kotel:"
    say "$other"
    say "Остановите его (данные сохранятся), подставив его --root, и запустите установку снова:"
    say "  ./messenger --root КАТАЛОГ shutdown"
    exit 1
  fi
fi

say "Скачиваю $ASSET…"
mkdir -p "$DIR"
tmp=$(mktemp -d "$DIR/.download-XXXXXX")
trap 'rm -rf "$tmp"' EXIT
curl -fsSL --retry 3 -o "$tmp/$ASSET" "$BASE/$ASSET" || fail "не удалось скачать программу."
curl -fsSL --retry 3 -o "$tmp/sums" "$BASE/AgentMessenger-Windows-Linux-SHA256.txt" || fail "не удалось скачать контрольные суммы."
want=$(awk -v n="$ASSET" '{f=$2; sub(/^\*/,"",f)} f==n {print $1}' "$tmp/sums")
have=$(sha256sum "$tmp/$ASSET" | awk '{print $1}')
[ -n "$want" ] && [ "$want" = "$have" ] || fail "контрольная сумма не совпала; ничего не изменено. Повторите позже."
chmod 755 "$tmp/$ASSET"
"$tmp/$ASSET" version >/dev/null || fail "скачанная программа не запускается на этом компьютере."
mv -f "$tmp/$ASSET" "$BIN"

cat > "$CMD" <<EOF
#!/bin/sh
# Kotel server commands: invite, status, enable, disable, revoke.
exec $BIN --root $DATA server "\$@"
EOF
chmod 755 "$CMD"
if [ ! -e /usr/local/bin/kotel-server ] && [ ! -L /usr/local/bin/kotel-server ]; then
  ln -s agent-messenger-server /usr/local/bin/kotel-server
fi

cat > "$UNIT" <<EOF
[Unit]
Description=Kotel server
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=root
Environment=HOME=/root
ExecStart=$BIN --root $DATA server run
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
Alias=kotel.service
EOF
mkdir -p "$DATA"
chmod 700 "$DATA"
systemctl daemon-reload
systemctl enable agent-messenger >/dev/null 2>&1
if systemctl is-active --quiet agent-messenger; then
  say "Обновляю: перезапускаю службу…"
  systemctl restart agent-messenger
else
  systemctl start agent-messenger
fi

i=0
until "$CMD" status >"$tmp/status" 2>&1; do
  i=$((i + 1))
  if [ "$i" -ge 20 ]; then
    cat "$tmp/status" >&2
    journalctl -u agent-messenger -n 20 --no-pager >&2 || true
    fail "служба не отвечает. Подробности выше и в journalctl -u agent-messenger."
  fi
  sleep 1
done

if grep -q '^Сервер: работает' "$tmp/status"; then
  say "Сервер уже был включён, настройки и устройства сохранены."
else
  if [ -n "${AM_PUBLIC:-}" ]; then
    "$CMD" enable --internet --port "$PORT" --public "$AM_PUBLIC"
  else
    "$CMD" enable --internet --port "$PORT"
  fi
fi

if command -v ufw >/dev/null 2>&1 && ufw status 2>/dev/null | grep -q 'Status: active'; then
  ufw allow "$PORT/tcp" >/dev/null && say "Брандмауэр ufw: порт $PORT открыт."
fi
if command -v firewall-cmd >/dev/null 2>&1 && systemctl is-active --quiet firewalld; then
  firewall-cmd --quiet --permanent --add-port="$PORT/tcp" && firewall-cmd --quiet --reload && say "Брандмауэр firewalld: порт $PORT открыт."
fi

say ""
"$CMD" status
say ""
"$CMD" invite
say ""
say "Новое приглашение для следующего компьютера: kotel-server invite"
say "Состояние: kotel-server status. Обновить: запустите эту же команду установки ещё раз."
say "Если у хостинга есть свой брандмауэр в панели управления, откройте там порт $PORT (TCP)."
