package media

import "strings"

var mimeTypes = map[string]string{
	".mp4": "video/mp4", ".m4v": "video/mp4", ".mov": "video/quicktime", ".webm": "video/webm",
	".mkv": "video/x-matroska", ".avi": "video/x-msvideo", ".wmv": "video/x-ms-wmv", ".flv": "video/x-flv",
	".mpeg": "video/mpeg", ".mpg": "video/mpeg", ".vob": "video/mpeg", ".3gp": "video/3gpp",
	".ts": "video/mp2t", ".mts": "video/mp2t", ".m2ts": "video/mp2t", ".ogv": "video/ogg",
}

// MimeType for a video extension; application/octet-stream when unknown.
func MimeType(ext string) string {
	if m, ok := mimeTypes[strings.ToLower(ext)]; ok {
		return m
	}
	return "application/octet-stream"
}
