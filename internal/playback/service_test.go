package playback

import (
	"context"
	"path/filepath"
	"testing"

	"enstream/internal/database"
	repo "enstream/internal/database/repositories"
)

func TestRules(t *testing.T) {
	if !IsComplete(95, 100) || IsComplete(50, 100) || !IsComplete(3580, 3600) || IsComplete(0, 0) {
		t.Fatal("IsComplete")
	}
	for _, c := range []struct{ pos, dur, want float64 }{{3, 100, 0}, {42.7, 100, 42}, {98, 100, 0}, {500, 100, 0}} {
		if got := ResumeFrom(c.pos, c.dur, false); got != c.want {
			t.Errorf("ResumeFrom(%v,%v)=%v", c.pos, c.dur, got)
		}
	}
	if ResumeFrom(50, 100, true) != 0 {
		t.Fatal("completed videos start over")
	}
	if ValidToken("short") || ValidToken("bad token with spaces!!") || !ValidToken("abcDEF0123456789_-xy") {
		t.Fatal("token")
	}
}

func TestProgressIsPerDevice(t *testing.T) {
	ctx := context.Background()
	db, _ := database.Open(filepath.Join(t.TempDir(), "p.db"))
	defer db.Close()
	st := repo.New(db)
	fid, _ := st.AddFolder(ctx, "/m", "m")
	vid, _, _ := st.UpsertVideo(ctx, repo.VideoRecord{FolderID: fid, Title: "Film", FileName: "f.mp4", FilePath: "/m/f.mp4", RelativePath: "f.mp4", Extension: ".mp4", MimeType: "video/mp4", FileSize: 1, Duration: 3600})
	svc := &Service{Store: st}
	phone, laptop := "phonephonephonephone01", "laptoplaptoplaptop0002"
	if _, err := svc.Update(ctx, vid, phone, "iPhone", 1234, 3600); err != nil {
		t.Fatal(err)
	}
	phoneID, laptopID := st.DeviceID(ctx, phone), st.DeviceID(ctx, laptop)
	cw, total, _ := st.History(ctx, phoneID, true, 10, 0)
	if total != 1 || cw[0].Position != 1234 {
		t.Fatalf("continue watching %+v", cw)
	}
	// Another device has its own history: nothing yet, and no position for the film.
	if _, total, _ := st.History(ctx, laptopID, true, 10, 0); total != 0 {
		t.Fatal("the laptop sees the phone's Continue Watching")
	}
	if _, err := svc.Update(ctx, vid, laptop, "Firefox", 10, 3600); err != nil {
		t.Fatal(err)
	}
	laptopID = st.DeviceID(ctx, laptop)
	if v, _ := st.Video(ctx, vid, phoneID); v.Position != 1234 {
		t.Fatalf("phone position %v", v.Position)
	}
	if v, _ := st.Video(ctx, vid, laptopID); v.Position != 10 {
		t.Fatalf("laptop position %v", v.Position)
	}
	done, err := svc.Update(ctx, vid, laptop, "Firefox", 3590, 3600)
	if err != nil || !done {
		t.Fatalf("completion %v %v", done, err)
	}
	if cw, _, _ := st.History(ctx, laptopID, true, 10, 0); len(cw) != 0 {
		t.Fatal("finished videos leave Continue Watching")
	}
	if cw, _, _ := st.History(ctx, phoneID, true, 10, 0); len(cw) != 1 {
		t.Fatal("finishing on the laptop changed the phone's Continue Watching")
	}
	if h, total, _ := st.History(ctx, laptopID, false, 10, 0); total != 1 || !h[0].Completed {
		t.Fatal("history keeps finished videos")
	}
	// Clearing is per device too.
	if err := st.ClearHistory(ctx, laptopID); err != nil {
		t.Fatal(err)
	}
	if _, total, _ := st.History(ctx, phoneID, false, 10, 0); total != 1 {
		t.Fatal("clearing the laptop's history cleared the phone's")
	}
	// Without a valid device token nothing is recorded.
	if _, err := svc.Update(ctx, vid, "bad", "", 5, 3600); err != nil {
		t.Fatal(err)
	}
	for _, bad := range [][2]float64{{-1, 100}, {500, 100}, {1, -5}} {
		if _, err := svc.Update(ctx, vid, phone, "", bad[0], bad[1]); err != ErrInvalid {
			t.Errorf("%v accepted", bad)
		}
	}
	if _, err := svc.Update(ctx, 999, phone, "", 1, 10); err != ErrNotFound {
		t.Fatal("unknown video")
	}
	// Without a valid token playback still records (temporary identity).
	if _, err := svc.Update(ctx, vid, "", "", 10, 3600); err != nil {
		t.Fatal(err)
	}
	devs, _ := st.Devices(ctx, laptop)
	if len(devs) != 2 || !devs[0].Current {
		t.Fatalf("devices %+v", devs)
	}
}
