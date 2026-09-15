# Tailscale, in plain terms

Why this dashboard has no login screen, and what that does and does not protect.

---

## What it is

- **Tailscale** is software you install on your devices. It connects them to each other
  directly, over the internet, as if they were on the same home network.
- **Your tailnet** is the private network it creates — just your devices, nothing else.
- **A node** is one device on it: your Mac, your phone.
- Each node gets a permanent private address like `100.101.102.103`, and a name like
  `macbook.tail1234.ts.net`. These only work inside your tailnet.
- It is built on **WireGuard**, a well-reviewed encryption protocol. Tailscale handles the parts
  WireGuard leaves to you: finding devices, exchanging keys, getting through routers.

---

## How it works

- You **sign in with an existing account** — Google, GitHub, Apple. Tailscale stores no password
  of yours.
- Each device generates its own **private key that never leaves the device**. Only the matching
  public key is uploaded.
- Tailscale's **coordination server** hands out public keys and addresses so devices can find
  each other. It is a phone book, not a pipe.
- Devices then talk **directly to each other**, encrypted end to end.
- When a router blocks a direct connection, traffic falls back to a Tailscale **relay (DERP)**.
  The relay forwards encrypted bytes it cannot read.
- **Device keys expire**, by default after a few months. An expired device drops off the tailnet
  until you sign in again.

---

## What Tailscale can and cannot see

| Can see | Cannot see |
|---|---|
| Which devices you own, their names and addresses | The contents of your traffic |
| When devices connect, and to what | Your private keys — they never leave your devices |
| Metadata needed to route: public keys, endpoints | Anything inside the WireGuard tunnel |

- Encryption is **end to end between your devices**. A relay in the middle carries ciphertext.
- You are trusting Tailscale's coordination server to hand out the *right* public keys. If it
  handed out a wrong one, it could insert itself. This is the core trust assumption.
- If you do not want that assumption, **Headscale** is an open-source coordination server you
  run yourself. Same clients, your own phone book.

---

## What it protects you from

- **The public internet.** The dashboard is not reachable from it. There is no address to try,
  no port to scan, no login page to brute force.
- **Your home or café network.** Other devices on the same Wi-Fi cannot see it either.
- **Snooping in transit.** Traffic is encrypted device to device.
- **Weak passwords.** There is no password to this app, so there is none to steal.

---

## The real risks

Each one, and what to do about it.

- **Someone gets into your Google or GitHub account.** They can add a device to your tailnet and
  reach the dashboard. This is the single biggest risk.
  → Turn on 2FA for that account. It is now the key to your tailnet.

- **Your unlocked phone or laptop is taken.** It is already a trusted node. The dashboard opens.
  → Use a device passcode. Remove the device at `login.tailscale.com` if it is lost.

- **An old device stays on the tailnet.** A sold laptop or a reinstalled phone can linger.
  → Review the device list occasionally and remove anything you do not recognise.

- **You expose the app by accident.** Binding to `0.0.0.0` with a port forwarded, or turning on
  **Tailscale Funnel** — which is designed to publish a service to the whole internet.
  → Bind only to the `100.x` address. Never enable Funnel for this app.

- **Backups leak instead.** `~/dashboard-data/` holds your database in plain SQLite. Copying it
  to a synced folder or a USB stick takes the data outside the tailnet entirely.
  → Treat the data directory as the sensitive thing. Tailscale does not protect a copied file.

- **Anyone who can use your screen.** No login means no second gate on an unlocked machine.
  → This is a deliberate trade for a single-user app. Know that it is the trade.

**What Tailscale is not:** it does not hide your browsing, it is not anonymity, and it does not
protect a device that is already compromised.

---

## How this dashboard uses it

- The server **listens only on the tailnet address**, for example `-addr 100.101.102.103:8484`.
  It is not listening anywhere else.
- **The tailnet is the authentication.** If a request arrives, it came from one of your devices.
  This is written down as [ADR-003](adr/ADR-003-auth-by-network-perimeter-not-accounts.md).
- **`-token` is an optional second lock.** With it set, every `/api/` request must carry a
  matching `X-Token` header. It guards against a device on your tailnet you did not mean to
  trust — not against the internet, which cannot reach the app anyway.
- **There are no accounts.** Nothing to log into, nothing to reset, nothing to leak.
- The phone must **already be on the tailnet**. Nothing in the app can put it there.

---

## Where the QR code fits

The QR solves one small problem: typing a long address and a secret token on a phone keyboard.

- On the desktop, open the **Plan** screen and choose **Pair phone**.
- The server creates a **one-time code**, valid for **90 seconds**, and shows it as a QR.
- You scan it with the phone. The phone opens the dashboard and receives the token.
- The code is **deleted the moment it is used**. Scanning the same QR twice fails.
- Only a **hash of the code** is stored, so a copy of the database does not yield a usable code.

**What the QR does not do:**

- It does **not** put your phone on the tailnet. Install Tailscale on the phone and sign in
  first, or the scan opens an address that goes nowhere.
- It does **not** create a login or an account. It hands over a token you already had.

**Why one-time and 90 seconds:** a QR sits on a screen. Screens get photographed, shoulder-surfed
and shared in calls. A code that dies in a minute and works once makes a photograph worthless.
Putting the raw token in the QR would have made that photograph permanent access.

---

## Rules for this setup

- **Do** bind to the `100.x` tailnet address.
- **Do** turn on 2FA for the account you signed into Tailscale with.
- **Do** set `-token` if other people have devices on your tailnet.
- **Don't** bind to `0.0.0.0`.
- **Don't** enable Tailscale Funnel for this app.
- **Don't** put `~/dashboard-data/` in a cloud-synced folder.

---

## Checking it

```sh
tailscale ip -4      # this device's tailnet address, e.g. 100.101.102.103
tailscale status     # every device, and which are online
```

On macOS the command often is not on `PATH`. Use the bundled copy:

```sh
/Applications/Tailscale.app/Contents/MacOS/Tailscale ip -4
```

Or read it from the menu-bar icon. The full device list is at `login.tailscale.com`.

**Not installed yet?** Install Tailscale on the Mac and the phone, sign both into the same
account, then run the command above. Until both devices are on the tailnet, the phone-facing
parts of this app — pairing, add-to-home-screen, using it away from the desk — cannot be tested.
