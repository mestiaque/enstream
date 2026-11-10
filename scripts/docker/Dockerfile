# Go toolchain plus FFmpeg, so integration tests probe and thumbnail real (generated) clips.
FROM golang:1.25-bookworm
RUN apt-get update && apt-get install -y --no-install-recommends ffmpeg && rm -rf /var/lib/apt/lists/*
