package media

import "strings"

// Compatibility is how likely a browser plays the original file directly.
type Compatibility string

const (
	Likely    Compatibility = "likely"    // MP4/WebM with codecs every current browser decodes
	Possibly  Compatibility = "possibly"  // plays in some browsers or devices (e.g. HEVC, MKV)
	Transcode Compatibility = "transcode" // needs conversion to play in a browser
	Unknown   Compatibility = "unknown"   // not probed (FFprobe missing or failed)
)

func isOneOf(v string, set ...string) bool {
	for _, s := range set {
		if v == s {
			return true
		}
	}
	return false
}

// Rate judges a file from its extension and probed codecs. It is a prediction, shown as a
// hint in the UI; the browser has the final word.
func Rate(ext string, m Metadata) Compatibility {
	if m.VideoCodec == "" {
		return Unknown
	}
	ext = strings.ToLower(ext)
	v, a := m.VideoCodec, m.AudioCodec
	webAudio := a == "" || isOneOf(a, "aac", "mp3", "opus", "vorbis", "flac")
	h264Plain := v == "h264" && (m.PixelFormat == "" || m.PixelFormat == "yuv420p" || m.PixelFormat == "yuvj420p") &&
		!strings.Contains(strings.ToLower(m.VideoProfile), "high 10") && !strings.Contains(strings.ToLower(m.VideoProfile), "4:2:2") &&
		!strings.Contains(strings.ToLower(m.VideoProfile), "4:4:4")

	switch {
	case isOneOf(ext, ".mp4", ".m4v", ".mov") && h264Plain && webAudio:
		return Likely
	case ext == ".webm" && isOneOf(v, "vp8", "vp9", "av1") && (a == "" || isOneOf(a, "opus", "vorbis")):
		return Likely
	case isOneOf(ext, ".mp4", ".m4v", ".mov", ".mkv", ".webm") && isOneOf(v, "h264", "hevc", "vp9", "av1"):
		// Plays in Chrome/Edge/Safari depending on codec, profile and hardware.
		return Possibly
	case ext == ".ogv" && v == "theora":
		return Possibly
	default:
		return Transcode
	}
}
