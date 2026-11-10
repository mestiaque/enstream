package media

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// MaxSubtitleSize keeps a broken or hostile file from using lots of memory.
const MaxSubtitleSize = 5 << 20

var srtTime = regexp.MustCompile(`(\d{1,2}):(\d{1,2}):(\d{1,2})[,.](\d{1,3})`)

// toUTF8 strips a BOM and treats invalid UTF-8 as Windows-1252/Latin-1, the usual
// encoding of older SRT files, so text never turns into replacement characters.
func toUTF8(b []byte) string {
	b = bytes.TrimPrefix(b, []byte("\xef\xbb\xbf"))
	if utf8.Valid(b) {
		return string(b)
	}
	var sb strings.Builder
	for _, c := range b {
		sb.WriteRune(rune(c))
	}
	return sb.String()
}

// SRTToVTT converts SubRip text to WebVTT. Cue numbers are kept as cue ids.
func SRTToVTT(src []byte) []byte {
	text := strings.ReplaceAll(strings.ReplaceAll(toUTF8(src), "\r\n", "\n"), "\r", "\n")
	text = strings.TrimRight(text, "\n")
	var out strings.Builder
	out.WriteString("WEBVTT\n\n")
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, "-->") {
			line = srtTime.ReplaceAllStringFunc(line, func(m string) string {
				p := srtTime.FindStringSubmatch(m)
				ms := p[4] + strings.Repeat("0", 3-len(p[4]))
				return fmt.Sprintf("%02s:%02s:%02s.%s", p[1], p[2], p[3], ms)
			})
		}
		out.WriteString(line)
		out.WriteByte('\n')
	}
	return []byte(out.String())
}

// NormalizeVTT makes sure a .vtt file is UTF-8 and has the WEBVTT header.
func NormalizeVTT(src []byte) []byte {
	text := strings.ReplaceAll(toUTF8(src), "\r\n", "\n")
	if !strings.HasPrefix(text, "WEBVTT") {
		text = "WEBVTT\n\n" + text
	}
	return []byte(text)
}

// ConvertWithFFmpeg turns ASS/SSA (styling dropped) into WebVTT.
func ConvertWithFFmpeg(ctx context.Context, ffmpeg *Tool, path string) ([]byte, error) {
	if !ffmpeg.Available() {
		return nil, fmt.Errorf("ffmpeg is not available")
	}
	return ffmpeg.Run(ctx, 30*time.Second, "-hide_banner", "-loglevel", "error", "-i", path, "-f", "webvtt", "-")
}
