# ENStream

**Your Personal Cinema.** ENStream turns your computer into a private video streaming server for
your home network. Run it on the PC that holds your videos; open its address in the browser of any
phone, tablet, TV or computer on the same Wi-Fi or Ethernet and watch. Nothing is uploaded and no
account or internet connection is needed.

Status: phase 1 (foundation) is done; see [IMPLEMENTATION_STATUS.md](IMPLEMENTATION_STATUS.md).

## Run

```bash
./dist/enstream-linux-amd64          # Ubuntu / Linux
dist\enstream-windows-amd64.exe      # Windows
```

It prints the addresses to open, for example `http://192.168.1.100:8080`. Options: `-port 9000`,
`-host 127.0.0.1`, `-data <folder>`, `-debug`, `-version`. FFmpeg is needed for thumbnails and
video details (`sudo apt install ffmpeg`, or `winget install Gyan.FFmpeg` on Windows).

## Build and test

Go 1.25+ is needed; without Go installed, the scripts use the official Go Docker image.

```bash
./scripts/build.sh        # dist/enstream-linux-amd64, -linux-arm64, -windows-amd64.exe
make test vet race
```

MIT License · © 2026 M. Estiaque Ahmed Khan
