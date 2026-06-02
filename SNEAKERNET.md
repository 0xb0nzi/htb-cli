# Sneakernet install: htb-cli on Parrot OS

How to move this patched `htb-cli` source to an offline Parrot OS box via flash
drive, build it there, and run it. No GitHub or internet required on the
target machine.

The fix on branch `fix/htb-unified-vm-endpoints` migrates the broken machine
lifecycle calls (`/machine/play`, `/machine/stop`, `/arena/start`,
`/arena/stop`, `/arena/reset`) to HTB's new unified endpoints (`/vm/spawn`,
`/vm/terminate`, `/vm/reset`).

## Prerequisites

- **Source host (this Mac):** Go 1.20+ installed.
- **Target host (Parrot, x86_64):** Go 1.20+ installed (`apt install golang-go`
  or grab a tarball from <https://go.dev/dl/>). Skip this if you use the
  prebuilt binary option below.

## 1. Build a self-contained source tarball (on the Mac)

Vendoring bakes every dependency into the source tree so the target machine
never needs to fetch modules.

```bash
cd <path>/htb-cli
git checkout fix/htb-unified-vm-endpoints
go mod vendor
tar czf /tmp/htb-cli-src-vendored.tar.gz \
    --exclude='.git' --exclude='vendor/.git' \
    -C "$(dirname "$PWD")" "$(basename "$PWD")"
shasum -a 256 /tmp/htb-cli-src-vendored.tar.gz   # record this
rm -rf vendor                                    # keep working tree clean
```

Copy `htb-cli-src-vendored.tar.gz` (and the sha256) to the flash drive.

## 2. Build offline on Parrot

```bash
cp /media/$USER/<YOUR_USB>/htb-cli-src-vendored.tar.gz ~/
cd ~
sha256sum htb-cli-src-vendored.tar.gz             # must match what you recorded
tar xzf htb-cli-src-vendored.tar.gz
cd htb-cli

go version                                        # need 1.20+
GOFLAGS=-mod=vendor GOPROXY=off go build -o htb-cli .
sudo install -m 0755 htb-cli /usr/local/bin/htb-cli
htb-cli version
```

`-mod=vendor GOPROXY=off` forces the build to use the bundled `vendor/` dir
and never reach the network — the whole point of vendoring.

## 3. Configure the API token

`htb-cli` reads the JWT app token from the `HTB_TOKEN` env var. Generate one
at <https://app.hackthebox.com/profile/settings>.

```bash
echo 'export HTB_TOKEN="<your app token>"' >> ~/.bashrc   # or ~/.zshrc
source ~/.bashrc
```

## 4. Smoke test

```bash
htb-cli version
htb-cli start -m <machine>     # exercises POST /vm/spawn
htb-cli stop                   # exercises POST /vm/terminate
htb-cli reset                  # exercises POST /vm/reset
```

## Alternative: prebuilt static binary (no Go on Parrot)

If you don't want to install Go on the target, cross-compile a fully static
ELF on the Mac and sneakernet that instead:

```bash
GOOS=linux GOARCH=amd64 go build -o /tmp/htb-cli-linux-amd64 .
# (use GOARCH=arm64 for an ARM Parrot box)
shasum -a 256 /tmp/htb-cli-linux-amd64
```

On Parrot:

```bash
cp /media/$USER/<YOUR_USB>/htb-cli-linux-amd64 ~/htb-cli
chmod +x ~/htb-cli                                # FAT/exFAT strips exec bit
sudo install -m 0755 ~/htb-cli /usr/local/bin/htb-cli
```

## Gotchas

- **FAT32/exFAT flash drives** don't preserve Unix exec bits or symlinks.
  Tarballs are fine (permissions are stored inside the archive); a raw binary
  needs `chmod +x` after copying.
- **Go version**: `go.mod` requires `go 1.20`. Older Parrot apt packages may
  ship something older — check `go version` first.
- **Token hygiene**: an `HTB_TOKEN` you've ever pasted into a chat, log, or
  shared terminal should be rotated at
  <https://app.hackthebox.com/profile/settings>. Treat it like a password.
- **Cleaning up**: `vendor/` is large (~20 MB) and regenerable from `go.mod`,
  so don't commit it — note `vendor/` is *not* listed in `.gitignore` (the
  entry is commented out), so be careful with `git add -A`. Run
  `go mod vendor` only when you're about to build the tarball, and
  `rm -rf vendor` after.
