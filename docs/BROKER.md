# Broker: what is built (T-015, first part)

Status on 2026-11-07: the background client can host a broker and a computer can join a broker by invitation. **Tasks do not travel between computers yet**: workers still run only on the computer where the task was created. The window says so on the «Сервер» tab. The placement principles are in [DEPLOYMENT.md](DEPLOYMENT.md).

## What works

- **Hosting** (D-033). The background client runs the broker inside its own process when the owner turns it on in the window. Off by default. Access is "only this computer" (`127.0.0.1`) by default; "local network" listens on all interfaces and lists the computer's private IPv4 addresses (no loopback, no link-local). Port 47100 by default, 1024–65535 allowed. A busy port is refused with a clear message, and the previous settings and the running server are kept. The setting is remembered and the broker starts again with the client.
- **Identity and TLS.** On first start the broker creates a stable `broker_id` (`brk-` + 16 hex) and a self-signed ECDSA P-256 certificate. TLS 1.3 only. Clients trust exactly that certificate by its SHA-256 fingerprint; host names are not used for trust.
- **Invitations.** The owner picks one of the broker's addresses and gets a one-time code `am1:…` (base64url JSON with address, `broker_id`, fingerprint and a 256-bit secret). Valid for 15 minutes, used once, stored as a hash. The window hides the code once a new device has joined.
- **Joining** (D-034). The other computer pastes the code and a device name. It first connects with the pinned fingerprint, checks `broker_id` and protocol version (`GET /v1/hello`), and only then sends the secret (`POST /v1/register`). It receives a device ID and a 256-bit device key; the broker stores only a hash of the key. A computer belongs to one broker: joining another one requires "Отключить" first, so nothing is ever sent to a new broker silently.
- **Checking.** "Проверить связь" repeats the identity check and calls `GET /v1/ping` with the device key. A revoked or unknown key is reported as such; an unreachable server or a different certificate are reported separately.
- **Devices.** The host lists devices with their last contact and can revoke one. Revocation takes effect on the next request. Revoked devices are counted, not listed.
- **Disconnect** forgets the connection on this computer. The device stays listed on the broker until it is revoked there.

## Files

In the client's data directory:

| Path | What |
|---|---|
| `broker/settings.json` | hosted broker: on/off, scope, port |
| `broker/broker.sqlite` | `broker_id`, devices (key hashes), invitations (secret hashes) |
| `broker/tls-cert.pem`, `broker/tls-key.pem` | the broker's certificate and private key (0600) |
| `broker/serve.lock` | one running broker per data directory |
| `server-connection.json` | this computer's membership, including the device key (0600; Q-013) |

A damaged settings or connection file never stops the client: the window shows the problem and the file is left as it is.

## Interfaces

- Control command `server` with actions `status`, `enable`, `disable`, `invite`, `revoke`, `connect`, `check`, `disconnect`. The status never contains the device key.
- Window API: `GET /api/server`, `POST /api/server-action` (same rules as other window actions: token, Origin, JSON, bounded fields).
- `messenger status` shows `broker=not_configured|hosting|connected|hosting+connected`.

## Not built yet

- Carrying tasks, questions and reports between computers (the rest of T-015).
- A standalone broker for a VPS and the VPS setup wizard (D-035).
- Keeping the device key in the OS secure storage (Q-013).
- A reachability check from another computer. "Работает" means the broker listens; it does not prove that another computer can reach it (firewall, VPN, router).

## Verified

On Linux with tests (`internal/broker`, `internal/supervisor/server_test.go`, `internal/localui`) and in headless Chromium against a real client: enable, invite, join (the client joins its own broker, as a second computer would), check, revoke and the check after it, a wrong code, a wrong port, disable. Not verified: two real computers, the Mac app, a local network.
