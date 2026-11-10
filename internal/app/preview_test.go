package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// A missing hover preview answers 404 and is made right away; then it is served as MP4.
// Clips of videos that leave the library are deleted.
func TestHoverPreviewClip(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	a, h := newTestApp(t)
	lib := t.TempDir()
	out, err := exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error", "-y", "-f", "lavfi", "-i", "testsrc2=size=320x240:rate=25:duration=30",
		"-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p", filepath.Join(lib, "Long.mp4")).CombinedOutput()
	if err != nil {
		t.Fatalf("%v %s", err, out)
	}
	js, _ := json.Marshal(map[string]string{"path": lib})
	code, e := call(t, h, "POST", "/api/v1/folders", string(js))
	if code != 201 {
		t.Fatalf("add folder: %d %+v", code, e)
	}
	var folder struct{ ID int64 }
	json.Unmarshal(e.Data, &folder)
	a.Scanner.Wait()
	_, e = call(t, h, "GET", "/api/v1/videos", "")
	var vids []struct{ ID int64 }
	json.Unmarshal(e.Data, &vids)
	if len(vids) != 1 {
		t.Fatalf("videos: %s", e.Data)
	}
	url := fmt.Sprintf("/api/v1/videos/%d/preview", vids[0].ID)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go a.Previews.Run(ctx)

	// Viewers (phones) can fetch previews too.
	const phone = "192.168.1.50:5000"
	if w := do(h, "GET", url, phone, nil); w.Code != 404 {
		t.Fatalf("before: %d", w.Code)
	}
	deadline := time.Now().Add(90 * time.Second)
	for {
		w := do(h, "GET", url, phone, nil)
		if w.Code == 200 {
			if ct := w.Header().Get("Content-Type"); ct != "video/mp4" || w.Body.Len() < 1000 {
				t.Fatalf("clip: %s %d bytes", ct, w.Body.Len())
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("preview never made: %d", w.Code)
		}
		time.Sleep(300 * time.Millisecond)
	}
	if w := do(h, "GET", "/api/v1/videos/999999/preview", phone, nil); w.Code != 404 {
		t.Fatalf("unknown video: %d", w.Code)
	}

	// Removing the folder removes its videos; the next pass deletes the clip.
	if code, _ := call(t, h, "DELETE", fmt.Sprintf("/api/v1/folders/%d", folder.ID), ""); code != 200 && code != 204 {
		t.Fatalf("remove folder: %d", code)
	}
	a.Previews.Wake()
	clipPath := a.Previews.Path(vids[0].ID)
	for deadline := time.Now().Add(10 * time.Second); ; time.Sleep(100 * time.Millisecond) {
		if _, err := os.Stat(clipPath); os.IsNotExist(err) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("orphan clip not removed")
		}
	}
}
