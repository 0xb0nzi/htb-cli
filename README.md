# htb-cli

> ## 🔧 Maintained fork
>
> This is a **maintained fork** of
> [`GoToolSharing/htb-cli`](https://github.com/GoToolSharing/htb-cli). The original project
> appears abandoned — no commits since February 2025 — and its machine commands broke when
> Hack The Box consolidated its API. This fork picks up maintenance and keeps the tool
> working.
>
> Original work © the GoToolSharing authors, licensed under **GPL-3.0**; this fork continues
> under the same license (see [`LICENSE`](./LICENSE)).
>
> **First fix:** the machine lifecycle (`start` / `stop` / `reset`) was migrated to HTB's
> unified `/vm/*` endpoints — `/vm/spawn`, `/vm/terminate`, `/vm/reset` — for every machine
> type (free, VIP, release arena), replacing the removed `/machine/play/{id}`, `/arena/*`,
> and `/machine/stop` routes. Also adds `SNEAKERNET.md`, an offline install guide for Parrot OS.
>
> ### 🗺️ Roadmap
>
> Goal: restore and maintain the **full** CLI against HTB's current API — auditing each
> command area, fixing any endpoints broken by the API consolidation, and verifying
> end-to-end against a live account.
>
> - [x] **Machine lifecycle** — `start` / `stop` / `reset` migrated to unified `/vm/*` endpoints
> - [x] **Machine info & listing** (`machines`, `info`) — migrated the removed `/machine/unreleased` and `/user/profile/activity` routes to the unified `/api/v5` endpoints
> - [ ] **Flag submission** (`submit`, `getflag`) — verify against current submit API
> - [ ] **Sherlocks** (`sherlocks`) — DFIR challenge support
> - [ ] **VPN** (`vpn`) — config download / region switching
> - [ ] **Pwnbox** (`pwnbox`) — spawn / terminate
> - [ ] **Challenges / Fortress / Pro Labs** — verify progress & info endpoints
> - [ ] **Shoutbox** (`shoutbox`) and **/etc/hosts helper** (`hosts`)
> - [ ] Full end-to-end pass against a live HTB account
> - [ ] Re-point module path, badges, and release CI to this fork
>
> Documentation below is inherited from upstream and is being updated as the fork progresses.

---

![Workflows (main)](https://github.com/0xb0nzi/htb-cli/actions/workflows/go.yml/badge.svg?branch=main)
![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/0xb0nzi/htb-cli/main)
![GitHub release](https://img.shields.io/github/v/release/0xb0nzi/htb-cli)
![GitHub Repo stars](https://img.shields.io/github/stars/0xb0nzi/htb-cli)

<a target="_blank" rel="noopener noreferrer" href="https://twitter.com/QU35T_TV" title="Follow"><img src="https://img.shields.io/twitter/follow/QU35T_TV?label=QU35T_TV&style=social" alt="Twitter QU35T_TV"></a>

<div>
  <img alt="current version" src="https://img.shields.io/badge/linux-supported-success">
  <img alt="current version" src="https://img.shields.io/badge/WSL-supported-success">
  <img alt="current version" src="https://img.shields.io/badge/mac-supported-success">
  <br>
  <img alt="amd64" src="https://img.shields.io/badge/amd64%20(x86__64)-supported-success">
  <img alt="arm64" src="https://img.shields.io/badge/arm64%20(aarch64)-supported-success">
</div>

<div align="center">
  <img src="./assets/logo.png" alt="Alt text" width="400">
</div></br>

# Documentation

> ℹ️ **Fork note:** The original upstream documentation site below is maintained by the
> original authors and predates this fork — it does **not** reflect changes made here (e.g.
> the unified `/vm/*` machine-lifecycle migration). Treat it as a general reference until
> this fork ships its own docs. Fork-specific behavior is tracked in the
> [Roadmap](#-roadmap) above.

Upstream documentation (general reference): <a href="https://htb-cli-documentation.qu35t.pw/" target="_blank">https://htb-cli-documentation.qu35t.pw/</a>
