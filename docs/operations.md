# Running Demitable

> The server may run 24/7 and be reachable over the internet, behind TLS
> termination you provide. This is the guide for doing that safely.
>
> It is written for a **GM who is not a sysadmin**. If something here is
> unclear or does not match your setup, that is a bug in this document.
>
> Governing decision: [ADR-0031](decisions/0031-internet-exposure.md).

---

## 1. What you are responsible for, and what the app is

| | Yours | The app's |
|---|---|---|
| TLS and certificates | ✅ | |
| DNS | ✅ | |
| Firewall and port exposure | ✅ | |
| Keeping the host patched | ✅ | |
| **Backing up the vault** | ✅ | |
| Rotating the GM secret, revoking accounts | ✅ | |
| Updating the app | ✅ | It may *notice* a new release; it will never install one |
| The vault format, the data, the projection, the access rules | | ✅ |

The split is deliberate. **The app is not the thing that owns your campaign —
the directory is.** The app is a process that reads and writes it.

## 2. The one thing to get right

> **Back up the vault. It is the only thing that cannot be regenerated.**

The app cannot lose your campaign, because it does not own it. But a
compromised host means the vault is gone, and there is no partial exposure — a
compromised server yields everything.

The good news is that the backup is trivial, because the vault is plain files:

```bash
# git — the option for most GMs, and the one that also gives history
cd ~/vaults/my-campaign
git init && git add . && git commit -m "campaign, day 1"
git remote add origin git@github.com:you/my-campaign.git
git push -u origin main

# or rsync, if you would rather not use git
rsync -a --delete ~/vaults/my-campaign/ backups:/vaults/my-campaign/
```

Two cautions:

- **`.demitable/` contains account hashes.** It is not a secret in the
  cryptographic sense — the hashes are argon2id — but you should not be
  committing it to a public repository. The `demitable init` scaffolder
  writes a `.gitignore` that excludes it.
- **The SQLite caches are not in the vault**, so there is nothing to back up
  for them. They rebuild.

Test the restore. A backup you have never restored is a hypothesis.

## 3. The deployment

```
        internet
            │  https, your certificate
            ▼
   ┌──────────────────┐
   │  Caddy / nginx / │   ← yours
   │  a tunnel        │
   └────────┬─────────┘
            │  http://127.0.0.1:8080
            ▼
   ┌──────────────────┐
   │  demitable       │   ← plain HTTP
   │  the vault       │
   └──────────────────┘
```

Start the app bound to loopback, behind your proxy:

```bash
demitable --vault ~/vaults/my-campaign \
          --public-url https://demitable.example.com
```

Behind a proxy, `--host 127.0.0.1` is correct and safest. If the app is
exposed directly, use `--expose`, which changes a set of defaults together
(see §5).

### Option A — Caddy, with a real certificate

The simplest good answer. Caddy gets and renews certificates automatically.

```caddyfile
demitable.example.com {
    reverse_proxy 127.0.0.1:8080
}
```

Caddy forwards `X-Forwarded-For` and `X-Forwarded-Proto`. Tell the app to
trust them, and only them:

```bash
demitable --vault ~/vaults/my-campaign \
          --public-url https://demitable.example.com \
          --trust-proxy 127.0.0.1
```

### Option B — a tunnel, no public port at all

Nothing is listening on your router, so there is nothing to scan.

```bash
# Cloudflare Tunnel
cloudflared tunnel run demitable

# Tailscale
tailscale funnel --bg 8080
```

With a tunnel there is no public address to rate-limit against, and no port
to misconfigure. This is the safest option and the least to go wrong.

### Option C — nginx, if you already have one

```nginx
server {
    server_name demitable.example.com;
    ssl_certificate     /etc/letsencrypt/live/demitable.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/demitable.example.com/privkey.pem;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host              $host;
        proxy_set_header X-Real-IP         $remote_addr;
        proxy_set_header X-Forwarded-For   $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header Upgrade           $http_upgrade;
        proxy_set_header Connection        "upgrade";
        proxy_read_timeout 3600s;   # the WebSocket
    }
}
```

**The `Upgrade`/`Connection` headers matter.** The live session state travels
over a WebSocket, and a proxy that does not forward the upgrade request will
leave the tabletop silently disconnected.

## 4. Keeping it running

```ini
# /etc/systemd/system/demitable.service
[Unit]
Description=Demitable
After=network-online.target

[Service]
Type=simple
User=demitable
WorkingDirectory=/home/demitable
ExecStart=/usr/local/bin/demitable \
    --vault /home/demitable/vaults/my-campaign \
    --public-url https://demitable.example.com \
    --trust-proxy 127.0.0.1
Restart=on-failure
RestartSec=5

# The app writes to the vault. Nothing else should.
ReadWritePaths=/home/demitable/vaults/my-campaign
ProtectSystem=strict
ProtectHome=read-only
NoNewPrivileges=true
PrivateTmp=true

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl enable --now demitable
sudo journalctl -u demitable -f      # structured logs, one JSON object per line
```

The app writes **structured logs to stdout** and manages no log files of its
own, so `journald` handles rotation and you never have to think about it.

It shuts down gracefully on `SIGTERM`: it stops accepting connections, finishes
in-flight writes, checkpoints the SQLite WAL, and exits. `systemctl restart`
is safe.

## 5. What `--expose` changes

One flag, and a set of defaults that only matter when the server is reachable
from outside. The point is that **you should not have to remember which
defaults are dangerous.**

| Behaviour | Default | `--expose` |
|---|---|---|
| Bind address | `127.0.0.1` | `0.0.0.0`, with a warning |
| Cookie `Secure` flag | follows the public scheme | always on |
| `X-Forwarded-*` | ignored | trusted only from `--trust-proxy` |
| GM authentication | implicit when local | **always required** |
| Auth rate limiting | on | on, stricter |
| Audit log | auth events | auth events, plus reads of GM-only notes |
| Public base URL | optional | **required** |
| Archive import | allowed | allowed, with SSRF defence always on |

**Use `--expose` only if the app is directly reachable from the internet.**
If it is behind a proxy or a tunnel, leave it off and use `--trust-proxy`.

## 6. Giving players access

1. The GM opens the app and creates a player. The app shows a **one-time join
   code** and a URL.
2. The player opens that URL on their phone, enters the code, and chooses a
   passphrase. The code is consumed and expires in 24 hours.
3. The player is now a seat. They can see their character, and the campaign as
   their character sees it, between sessions as well as during them.

The player never receives a file. They never receive the vault. Their view is
assembled per request by the projection layer, so a GM-only note is not
merely hidden — it is not sent.

**To remove a player**, delete the account. To change a passphrase, reset it
from the player's page.

## 7. Upgrading

```bash
systemctl stop demitable
# back up the vault — seriously
cp -a ~/vaults/my-campaign ~/vaults/my-campaign.bak
# replace the binary
systemctl start demitable
```

The app **never updates itself.** An update that changed the vault format
under a running campaign is not something an app should do to you, so it will
tell you a newer release exists and leave the decision with you.

## 8. If something goes wrong

| Symptom | Likely cause |
|---|---|
| Players cannot connect | The proxy is not forwarding the WebSocket upgrade — check §3 Option C |
| `Secure cookie` errors on the LAN | The app is being reached over plain HTTP while the public scheme is `https`; set `--public-url` to the scheme you are actually using |
| The tabletop disconnects every few minutes | A proxy timeout. Raise `proxy_read_timeout` (nginx) or Caddy's timeouts |
| Searches return nothing | The FTS index was lost or is stale. It rebuilds; check `~/.local/share/demitable/` is writable |
| "database is locked" | Two instances are running against the same vault, or the vault is on a network filesystem — the SQLite caches must be on a local disk |
| The app will not start | The vault path is wrong, or the port is taken. It prints both |
| Rate-limited and locked out | Wait out the backoff. The GM's own account is not rate-limited |

## 9. The check before you expose it

The app's own phase-1 exit criteria for exposure are a checklist. The ones
that matter most to you:

- [ ] The import endpoint refuses private addresses, and refuses to reconnect
      to a different address than it validated.
- [ ] The import is off unless you enabled it.
- [ ] Authentication is rate-limited.
- [ ] Absolute links use `--public-url`, not the `Host` header.
- [ ] Forwarded headers are trusted only from your proxy.
- [ ] The MCP endpoint is off.
- [ ] The app works with the network down.
- [ ] **You have restored a backup at least once.**

## 10. What this document does not cover

- **Multiple campaigns on one server.** The app serves one vault per process.
  Run two processes on two ports, or a reverse proxy that routes by path.
- **High availability.** One process, one machine. If it goes down, the table
  waits. A campaign is not a business.
- **A native mobile app.** The API is designed so one is possible; it does
  not exist yet.
