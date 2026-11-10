package media

import (
	"os"
	"path/filepath"
	"strings"
)

// SubtitleFile is an external subtitle found next to a video.
type SubtitleFile struct {
	Path     string
	Format   string // srt | vtt | ass | ssa
	Language string // "en", "bn"… when the name says so
	Label    string
}

var subtitleExt = map[string]bool{".srt": true, ".vtt": true, ".ass": true, ".ssa": true}

var languageNames = map[string]string{
	"en": "English", "eng": "English", "english": "English",
	"bn": "Bangla", "ben": "Bangla", "bangla": "Bangla", "bengali": "Bangla",
	"hi": "Hindi", "hin": "Hindi", "hindi": "Hindi",
	"ar": "Arabic", "ara": "Arabic", "arabic": "Arabic",
	"es": "Spanish", "spa": "Spanish", "spanish": "Spanish",
	"fr": "French", "fre": "French", "fra": "French", "french": "French",
	"de": "German", "ger": "German", "deu": "German", "german": "German",
	"ja": "Japanese", "jpn": "Japanese", "japanese": "Japanese",
	"ko": "Korean", "kor": "Korean", "korean": "Korean",
	"zh": "Chinese", "chi": "Chinese", "zho": "Chinese", "chinese": "Chinese",
	"ur": "Urdu", "urd": "Urdu", "urdu": "Urdu",
	"pt": "Portuguese", "por": "Portuguese", "ru": "Russian", "rus": "Russian",
	"it": "Italian", "ita": "Italian", "tr": "Turkish", "tur": "Turkish", "id": "Indonesian", "ind": "Indonesian",
}

var languageCodes = map[string]string{"English": "en", "Bangla": "bn", "Hindi": "hi", "Arabic": "ar", "Spanish": "es", "French": "fr", "German": "de", "Japanese": "ja", "Korean": "ko", "Chinese": "zh", "Urdu": "ur", "Portuguese": "pt", "Russian": "ru", "Italian": "it", "Turkish": "tr", "Indonesian": "id"}

// MatchSubtitles finds subtitles for videoPath among the names in its directory:
// "movie.srt", "movie.en.srt", "movie.English.forced.vtt"… (case-insensitive base match).
func MatchSubtitles(videoPath string, dirNames []string) []SubtitleFile {
	dir := filepath.Dir(videoPath)
	base := strings.ToLower(strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath)))
	var out []SubtitleFile
	for _, name := range dirNames {
		ext := strings.ToLower(filepath.Ext(name))
		if !subtitleExt[ext] {
			continue
		}
		stem := strings.TrimSuffix(name, filepath.Ext(name))
		lower := strings.ToLower(stem)
		if lower != base && !strings.HasPrefix(lower, base+".") {
			continue
		}
		extra := strings.Trim(stem[len(base):], ".")
		lang, label := "", ""
		for _, part := range strings.Split(extra, ".") {
			if n, ok := languageNames[strings.ToLower(part)]; ok && lang == "" {
				lang, label = languageCodes[n], n
			}
		}
		if label == "" {
			label = extra
		}
		if label == "" {
			label = "Subtitles"
		}
		if strings.Contains(strings.ToLower(extra), "forced") && !strings.Contains(strings.ToLower(label), "forced") {
			label += " (forced)"
		}
		out = append(out, SubtitleFile{Path: filepath.Join(dir, name), Format: ext[1:], Language: lang, Label: label})
	}
	return out
}

// DirNames lists a directory's entry names (empty on error).
func DirNames(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names
}
