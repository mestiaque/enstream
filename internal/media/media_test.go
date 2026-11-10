package media

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseProbe(t *testing.T) {
	js := `{"format":{"format_name":"mov,mp4,m4a,3gp,3g2,mj2","duration":"125.4","size":"1048576","bit_rate":"66900"},
	"streams":[{"codec_type":"video","codec_name":"png","disposition":{"attached_pic":1}},
	{"codec_type":"video","codec_name":"h264","profile":"High","width":1920,"height":1080,"pix_fmt":"yuv420p","avg_frame_rate":"30000/1001","disposition":{"attached_pic":0}},
	{"codec_type":"audio","codec_name":"aac","channels":2},{"codec_type":"subtitle","codec_name":"mov_text"}]}`
	m, err := ParseProbe([]byte(js))
	if err != nil {
		t.Fatal(err)
	}
	if m.VideoCodec != "h264" || m.Width != 1920 || m.FPS != 29.97 || m.AudioCodec != "aac" || m.AudioChannels != 2 || m.Subtitles != 1 || m.Duration != 125.4 {
		t.Fatalf("%+v", m)
	}
	if _, err := ParseProbe([]byte(`{"format":{},"streams":[{"codec_type":"audio","codec_name":"mp3"}]}`)); err == nil {
		t.Fatal("audio-only must be refused as a video")
	}
	if m, _ := ParseProbe([]byte(`{"format":{"duration":"N/A"},"streams":[{"codec_type":"video","codec_name":"mpeg4","avg_frame_rate":"0/0","r_frame_rate":"25/1"}]}`)); m.Duration != 0 || m.FPS != 25 {
		t.Fatalf("missing values: %+v", m)
	}
}

func TestRate(t *testing.T) {
	cases := []struct {
		ext  string
		m    Metadata
		want Compatibility
	}{
		{".mp4", Metadata{VideoCodec: "h264", AudioCodec: "aac", PixelFormat: "yuv420p"}, Likely},
		{".mp4", Metadata{VideoCodec: "h264"}, Likely}, // no audio is fine
		{".webm", Metadata{VideoCodec: "vp9", AudioCodec: "opus"}, Likely},
		{".mp4", Metadata{VideoCodec: "hevc", AudioCodec: "aac"}, Possibly},
		{".mkv", Metadata{VideoCodec: "h264", AudioCodec: "aac"}, Possibly},
		{".mp4", Metadata{VideoCodec: "h264", AudioCodec: "ac3"}, Possibly},
		{".mp4", Metadata{VideoCodec: "h264", VideoProfile: "High 10", AudioCodec: "aac"}, Possibly},
		{".avi", Metadata{VideoCodec: "mpeg4", AudioCodec: "mp3"}, Transcode},
		{".wmv", Metadata{VideoCodec: "wmv2"}, Transcode},
		{".mp4", Metadata{}, Unknown},
	}
	for _, c := range cases {
		if got := Rate(c.ext, c.m); got != c.want {
			t.Errorf("%s %+v: %s, want %s", c.ext, c.m, got, c.want)
		}
	}
}

func TestThumbnailTime(t *testing.T) {
	for d, want := range map[float64]float64{0: 0, 3: 1.5, 10: 2, 600: 60, 7200: 300} {
		if got := ThumbnailTime(d); got != want {
			t.Errorf("%v → %v, want %v", d, got, want)
		}
	}
}

func TestMatchSubtitles(t *testing.T) {
	names := []string{"Movie.mp4", "movie.srt", "Movie.en.srt", "Movie.Bangla.vtt", "Movie.English.forced.ass", "Movie 2.srt", "Movie.director.ssa", "other.srt", "Movie.txt"}
	subs := MatchSubtitles(filepath.Join("/v", "Movie.mp4"), names)
	got := map[string]string{}
	for _, s := range subs {
		got[filepath.Base(s.Path)] = s.Language + "|" + s.Label + "|" + s.Format
	}
	want := map[string]string{
		"movie.srt":                "|Subtitles|srt",
		"Movie.en.srt":             "en|English|srt",
		"Movie.Bangla.vtt":         "bn|Bangla|vtt",
		"Movie.English.forced.ass": "en|English (forced)|ass",
		"Movie.director.ssa":       "|director|ssa",
	}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: %q, want %q", k, got[k], v)
		}
	}
}

func TestWalk(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	mk := func(p string) {
		full := filepath.Join(root, p)
		os.MkdirAll(filepath.Dir(full), 0o755)
		os.WriteFile(full, []byte("x"), 0o644)
	}
	for _, p := range []string{"a.mp4", "B.MKV", "sub/c.avi", "sub/deep/d.webm", ".hidden/e.mp4", ".f.mp4", "skipme/g.mp4", "notes.txt", "dl.mp4.part", "~$tmp.mp4"} {
		mk(p)
	}
	os.WriteFile(filepath.Join(outside, "secret.mp4"), []byte("s"), 0o644)
	os.Symlink(filepath.Join(outside, "secret.mp4"), filepath.Join(root, "escape.mp4"))
	os.Symlink(filepath.Join(root, "a.mp4"), filepath.Join(root, "alias.mp4"))
	os.Symlink(outside, filepath.Join(root, "linkdir"))
	exts := map[string]bool{".mp4": true, ".mkv": true, ".avi": true, ".webm": true}
	files, _ := Walk(context.Background(), root, WalkOptions{Extensions: exts, Excluded: []string{"SkipMe"}})
	var rels []string
	for _, f := range files {
		rels = append(rels, f.RelPath)
	}
	got := strings.Join(rels, ",")
	if got != "B.MKV,a.mp4,alias.mp4,sub/c.avi,sub/deep/d.webm" {
		t.Fatalf("walk: %s", got)
	}
	if Title("the.big_movie  (2020).mkv") != "the big movie (2020)" {
		t.Fatal(Title("the.big_movie  (2020).mkv"))
	}
}

func TestSRTToVTT(t *testing.T) {
	in := "\xef\xbb\xbf1\r\n00:00:01,5 --> 0:00:03,250\r\nHello <i>there</i>\r\n\r\n2\r\n00:01:00,000 --> 00:01:02,000\r\nCaf\xe9\r\n"
	got := string(SRTToVTT([]byte(in)))
	want := "WEBVTT\n\n1\n00:00:01.500 --> 00:00:03.250\nHello <i>there</i>\n\n2\n00:01:00.000 --> 00:01:02.000\nCafé\n"
	if got != want {
		t.Fatalf("got %q", got)
	}
	if v := string(NormalizeVTT([]byte("00:01.000 --> 00:02.000\nx"))); v[:6] != "WEBVTT" {
		t.Fatal(v)
	}
}
