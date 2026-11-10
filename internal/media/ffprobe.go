package media

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Metadata is what ENStream keeps about a video. Zero values mean "unknown".
type Metadata struct {
	Container     string
	Duration      float64 // seconds
	Size          int64
	Width         int
	Height        int
	FPS           float64
	VideoCodec    string
	VideoProfile  string
	PixelFormat   string
	AudioCodec    string
	AudioChannels int
	Bitrate       int64
	Subtitles     int // embedded subtitle streams
}

type probeOutput struct {
	Format struct {
		FormatName string `json:"format_name"`
		Duration   string `json:"duration"`
		Size       string `json:"size"`
		BitRate    string `json:"bit_rate"`
	} `json:"format"`
	Streams []struct {
		CodecType   string `json:"codec_type"`
		CodecName   string `json:"codec_name"`
		Profile     string `json:"profile"`
		Width       int    `json:"width"`
		Height      int    `json:"height"`
		PixFmt      string `json:"pix_fmt"`
		AvgFrame    string `json:"avg_frame_rate"`
		RFrame      string `json:"r_frame_rate"`
		Channels    int    `json:"channels"`
		Duration    string `json:"duration"`
		Disposition struct {
			AttachedPic int `json:"attached_pic"`
		} `json:"disposition"`
	} `json:"streams"`
}

// Probe runs ffprobe on a file (60 s limit; large files are only read at the start).
func Probe(ctx context.Context, ffprobe *Tool, path string) (Metadata, error) {
	out, err := ffprobe.Run(ctx, 60*time.Second, "-v", "error", "-print_format", "json", "-show_format", "-show_streams", "--", path)
	if err != nil {
		return Metadata{}, err
	}
	return ParseProbe(out)
}

// ParseProbe turns ffprobe JSON into Metadata. Missing fields stay zero.
func ParseProbe(b []byte) (Metadata, error) {
	var p probeOutput
	if err := json.Unmarshal(b, &p); err != nil {
		return Metadata{}, fmt.Errorf("unreadable ffprobe output: %w", err)
	}
	m := Metadata{Container: p.Format.FormatName}
	m.Duration = parseFloat(p.Format.Duration)
	m.Size, _ = strconv.ParseInt(p.Format.Size, 10, 64)
	m.Bitrate, _ = strconv.ParseInt(p.Format.BitRate, 10, 64)
	hasVideo := false
	for _, s := range p.Streams {
		switch s.CodecType {
		case "video":
			// Cover art in MP4/MKV is a "video" stream too; the first real one wins.
			if hasVideo || s.Disposition.AttachedPic == 1 {
				continue
			}
			hasVideo = true
			m.VideoCodec, m.VideoProfile, m.PixelFormat = s.CodecName, s.Profile, s.PixFmt
			m.Width, m.Height = s.Width, s.Height
			m.FPS = frameRate(s.AvgFrame)
			if m.FPS == 0 {
				m.FPS = frameRate(s.RFrame)
			}
			if m.Duration == 0 {
				m.Duration = parseFloat(s.Duration)
			}
		case "audio":
			if m.AudioCodec == "" {
				m.AudioCodec, m.AudioChannels = s.CodecName, s.Channels
			}
		case "subtitle":
			m.Subtitles++
		}
	}
	if !hasVideo {
		return m, fmt.Errorf("no video stream")
	}
	return m, nil
}

func parseFloat(s string) float64 {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil || f < 0 || f != f {
		return 0
	}
	return f
}

// frameRate parses "30000/1001" or "25"; absurd values (bad headers) count as unknown.
func frameRate(s string) float64 {
	var f float64
	if n, d, ok := strings.Cut(s, "/"); ok {
		num, den := parseFloat(n), parseFloat(d)
		if den == 0 {
			return 0
		}
		f = num / den
	} else {
		f = parseFloat(s)
	}
	if f <= 0 || f > 1000 {
		return 0
	}
	return float64(int(f*1000+0.5)) / 1000
}
