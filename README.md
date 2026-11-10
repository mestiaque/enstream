# ENStream

**Your personal cinema.** ENStream turns one computer at home into a private streaming service:
pick the folders with your videos and watch them on phones, tablets, smart TVs and other
computers on the same Wi-Fi, in any browser. No cloud, no account, no tracking.

- Netflix-style library with thumbnails, search, filters and *Continue watching* on every device
- Custom player: resume, subtitles (SRT/VTT/ASS), speed, picture-in-picture, keyboard and touch controls
- Desktop app with a tray icon: install it on the computer that has the videos, and you're done
- Optional PIN, automatic scans, daily backups, in-app updates
- Same family as ENViewer, ENSave, ENPlayer and ENPost

## Install (on the computer with your videos)

Install ENStream once, on the computer that stores the videos (the "host"). Phones and TVs need
nothing: they just open an address in their browser.

The commands below always fetch the **latest** release, so they work on any computer with an
internet connection. Installed copies update themselves (Settings → Desktop → *Check for updates*).

### Ubuntu / Debian (22.04 or newer), .deb (recommended)

Open a terminal (**Ctrl+Alt+T**) and paste:

```bash
sudo apt update && sudo apt install -y curl
URL=$(curl -fsSL https://api.github.com/repos/mestiaque/enstream/releases/latest | grep -o 'https://[^"]*_amd64\.deb' | head -1)
curl -fL -o /tmp/enstream.deb "$URL"
sudo apt install -y /tmp/enstream.deb
```

`apt` also installs FFmpeg (thumbnails and video details). Start ENStream from the app grid
(search “ENStream”) or run `enstream`. Running the same commands again upgrades it.

### Any Linux, AppImage (no installation)

```bash
mkdir -p ~/Applications
URL=$(curl -fsSL https://api.github.com/repos/mestiaque/enstream/releases/latest | grep -o 'https://[^"]*_amd64\.AppImage' | head -1)
curl -fL -o ~/Applications/ENStream.AppImage "$URL"
chmod +x ~/Applications/ENStream.AppImage
~/Applications/ENStream.AppImage
```

The AppImage uses the system FFmpeg: `sudo apt install ffmpeg` (or your distribution's equivalent).

### Windows 10 / 11, installer

Open **PowerShell** (Start → type “PowerShell”) and paste:

```powershell
$r = Invoke-RestMethod https://api.github.com/repos/mestiaque/enstream/releases/latest
$a = $r.assets | Where-Object name -like '*-setup.exe' | Select-Object -First 1
Invoke-WebRequest $a.browser_download_url -OutFile "$env:TEMP\$($a.name)"
Start-Process "$env:TEMP\$($a.name)" -ArgumentList '/S' -Wait
```

Windows asks once for administrator approval: the installer lets ENStream through Windows
Firewall on private (home) networks, so phones can connect. FFmpeg is downloaded automatically on
first start. ENStream appears in the Start menu.

## First start

1. **Choose your video folders** (*Choose a folder…* or one of the suggestions). Files are never
   moved, renamed or changed; ENStream only reads them.
2. **Optional PIN** if other people use your Wi-Fi.
3. **Scan the QR code** with your phone, or type the address shown (for example
   `http://192.168.1.20:8080`) in any browser on the same network.

ENStream starts with the computer and keeps running in the tray when you close the window, so
the TV can always reach it. Both are switches in **Settings → Desktop**; quit from the tray menu.

## Uninstall

**Ubuntu / Debian:**

```bash
sudo apt remove -y en-stream
```

**AppImage:** quit it from the tray, then `rm ~/Applications/ENStream.AppImage`

**Windows:** Settings → Apps → ENStream → Uninstall, or in PowerShell:

```powershell
$u = Get-ItemProperty 'HKLM:\Software\Microsoft\Windows\CurrentVersion\Uninstall\*','HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\*' -ErrorAction SilentlyContinue |
  Where-Object DisplayName -like 'ENStream*' | Select-Object -First 1
Start-Process $u.UninstallString.Trim('"') -ArgumentList '/S' -Wait
```

**Also remove the library data** (optional): the index, thumbnails, history, settings and backups.
Your video files are not in here and are never deleted.

```bash
rm -rf ~/.local/share/enstream ~/.config/com.mestiaque.enstream ~/.local/share/com.mestiaque.enstream ~/.config/autostart/ENStream.desktop   # Linux
```

```powershell
Remove-Item -Recurse -Force "$env:LOCALAPPDATA\ENStream","$env:APPDATA\com.mestiaque.enstream","$env:LOCALAPPDATA\com.mestiaque.enstream" -ErrorAction SilentlyContinue   # Windows
```

## Headless server (NAS, mini PC, no screen)

The release also contains the bare server: `enstream-server-linux-amd64`,
`enstream-server-linux-arm64` (Raspberry Pi 4/5) and `enstream-server-windows-amd64.exe`.

```bash
URL=$(curl -fsSL https://api.github.com/repos/mestiaque/enstream/releases/latest | grep -o 'https://[^"]*enstream-server-linux-amd64"' | tr -d '"' | head -1)
curl -fL -o ~/enstream "$URL" && chmod +x ~/enstream
sudo apt install -y ffmpeg
~/enstream            # prints the addresses to open; add folders from any browser
```

Options: `-port 8080`, `-host 0.0.0.0`, `-data <folder>`, `-reset-pin` (forgotten PIN), `-debug`, `-version`.

## Good to know

- **Who can connect:** only devices on your local network (home Wi-Fi, Ethernet, VPNs such as
  Tailscale). Requests from the internet are refused. Add a PIN for more privacy.
- **Forgot the PIN?** Quit ENStream and start the server once with `-reset-pin`
  (desktop: `enstream-server -reset-pin` from the install folder), then start ENStream again.
- **A video won't play in the browser?** Some formats (HEVC, old AVI) aren't supported by every
  browser. The details page says so and offers *Copy stream link* for VLC.
- **Where is my data?** Linux `~/.local/share/enstream`, Windows `%LOCALAPPDATA%\ENStream`.
  Backups are in its `backups` folder (Settings → Backups).

## Development

```bash
make test                         # Go tests (Docker with FFmpeg when Go isn't installed)
make run                          # the server on http://localhost:8080
cd desktop
./scripts/build-sidecar.sh linux  # bundle the server for the desktop app
./scripts/docker.sh "pnpm install && pnpm tauri build --bundles deb,appimage"
```

Layout: `cmd/enstream` and `internal/` (Go server), `web/` (interface, embedded in the server),
`desktop/` (Tauri app around the server). Progress notes: [IMPLEMENTATION_STATUS.md](IMPLEMENTATION_STATUS.md).

## License

MIT © 2026 M. Estiaque Ahmed Khan. Icons: Lucide (ISC).
