package rotate

import (
	"sort"
	"testing"
	"time"

	"bodsch.me/mariadb-backup/internal/logging"
)

const baseDir = "/backup"

// fakeFS is an in-memory FileSystem recording mutations.
type fakeFS struct {
	entries []Entry
	exists  map[string]bool
	renames [][2]string
	removed []string
}

func newFakeFS(names ...string) *fakeFS {
	f := &fakeFS{exists: map[string]bool{}}
	for _, n := range names {
		f.entries = append(f.entries, Entry{Name: n, IsDir: true})
		f.exists[baseDir+"/"+n] = true
	}
	return f
}

func (f *fakeFS) ReadDir(string) ([]Entry, error) { return f.entries, nil }
func (f *fakeFS) Rename(o, n string) error {
	f.renames = append(f.renames, [2]string{o, n})
	f.exists[o] = false
	f.exists[n] = true
	return nil
}
func (f *fakeFS) RemoveAll(p string) error {
	f.removed = append(f.removed, p)
	f.exists[p] = false
	return nil
}
func (f *fakeFS) Exists(p string) bool { return f.exists[p] }

func mustDate(t *testing.T, s string) time.Time {
	t.Helper()
	tm, err := time.ParseInLocation("2006-01-02", s, time.UTC)
	if err != nil {
		t.Fatalf("bad date %q: %v", s, err)
	}
	return tm
}

func TestCreationDate(t *testing.T) {
	cases := []struct {
		name    string
		wantOK  bool
		wantYMD string
	}{
		{"20250216-2000", true, "2025-02-16"},
		{"KW07_20250216-2000", true, "2025-02-16"},
		{"KW7_20250216-2000", true, "2025-02-16"},
		{"garbage", false, ""},
		{"2025021", false, ""}, // too short
		{"app.log.txt0", false, ""},
	}
	for _, c := range cases {
		got, ok := creationDate(c.name, time.UTC)
		if ok != c.wantOK {
			t.Errorf("creationDate(%q) ok=%v want %v", c.name, ok, c.wantOK)
			continue
		}
		if ok && got.Format("2006-01-02") != c.wantYMD {
			t.Errorf("creationDate(%q)=%s want %s", c.name, got.Format("2006-01-02"), c.wantYMD)
		}
	}
}

func TestSundayRename(t *testing.T) {
	fs := newFakeFS("20250216-2000") // 2025-02-16 is a Sunday, ISO week 7
	now := mustDate(t, "2025-02-20")
	if err := Rotate(baseDir, 100, 100, now, fs, logging.NewCapture()); err != nil {
		t.Fatal(err)
	}
	if len(fs.renames) != 1 {
		t.Fatalf("want 1 rename, got %v", fs.renames)
	}
	want := [2]string{baseDir + "/20250216-2000", baseDir + "/KW07_20250216-2000"}
	if fs.renames[0] != want {
		t.Errorf("rename = %v want %v", fs.renames[0], want)
	}
	if len(fs.removed) != 0 {
		t.Errorf("unexpected removals: %v", fs.removed)
	}
}

func TestSundayRenameSkipsExistingWeeklyBackup(t *testing.T) {
	// A backup already named KW<week>_ whose date falls on a Sunday must not be
	// re-prefixed to KW<week>_KW<week>_... on subsequent runs.
	fs := newFakeFS("KW07_20250216-2000") // 2025-02-16 is a Sunday, ISO week 7
	now := mustDate(t, "2025-02-20")
	if err := Rotate(baseDir, 100, 100, now, fs, logging.NewCapture()); err != nil {
		t.Fatal(err)
	}
	if len(fs.renames) != 0 {
		t.Errorf("weekly backup must not be re-prefixed, got renames %v", fs.renames)
	}
}

func TestSundayRenameSkippedWhenTargetExists(t *testing.T) {
	fs := newFakeFS("20250216-2000")
	fs.exists[baseDir+"/KW07_20250216-2000"] = true // target already there
	now := mustDate(t, "2025-02-20")
	if err := Rotate(baseDir, 100, 100, now, fs, logging.NewCapture()); err != nil {
		t.Fatal(err)
	}
	if len(fs.renames) != 0 {
		t.Errorf("expected no rename, got %v", fs.renames)
	}
}

func TestAgeDelete(t *testing.T) {
	// daily=3, now=2025-02-20.
	fs := newFakeFS(
		"20250210-2000",      // Mon, age 10 -> delete
		"20250218-2000",      // Tue, age 2 -> keep
		"KW06_20250204-2000", // KW -> never age-deleted
	)
	now := mustDate(t, "2025-02-20")
	if err := Rotate(baseDir, 3, 100, now, fs, logging.NewCapture()); err != nil {
		t.Fatal(err)
	}
	if got := fs.removed; len(got) != 1 || got[0] != baseDir+"/20250210-2000" {
		t.Errorf("removed = %v, want only /20250210-2000", got)
	}
}

func TestAgeDeleteBoundaryIsStrict(t *testing.T) {
	// age exactly == daily must NOT delete (Python uses strict >).
	fs := newFakeFS("20250217-2000") // Mon, age 3
	now := mustDate(t, "2025-02-20")
	if err := Rotate(baseDir, 3, 100, now, fs, logging.NewCapture()); err != nil {
		t.Fatal(err)
	}
	if len(fs.removed) != 0 {
		t.Errorf("age==daily should not delete, removed=%v", fs.removed)
	}
}

func TestMaxFivePerDayIndependentOfDaily(t *testing.T) {
	names := []string{
		"20250218-0100", "20250218-0200", "20250218-0300", "20250218-0400",
		"20250218-0500", "20250218-0600", "20250218-0700",
	}
	for _, daily := range []int{2, 100} {
		fs := newFakeFS(names...)
		now := mustDate(t, "2025-02-18") // age 0, so no age-based deletion
		if err := Rotate(baseDir, daily, 100, now, fs, logging.NewCapture()); err != nil {
			t.Fatal(err)
		}
		sort.Strings(fs.removed)
		want := []string{baseDir + "/20250218-0100", baseDir + "/20250218-0200"}
		if len(fs.removed) != 2 || fs.removed[0] != want[0] || fs.removed[1] != want[1] {
			t.Errorf("daily=%d: removed=%v want %v (cap must be 5, not daily)", daily, fs.removed, want)
		}
	}
}

func TestWeeklyRetention(t *testing.T) {
	// Use Tuesday-dated KW dirs so the Sunday-rename step does not re-prefix
	// them; this isolates the weekly-retention path. Weeks 6,7,8; keep newest 2.
	fs := newFakeFS(
		"KW06_20250204-2000", // ISO week 6
		"KW07_20250211-2000", // ISO week 7
		"KW08_20250218-2000", // ISO week 8
	)
	now := mustDate(t, "2025-02-20")
	if err := Rotate(baseDir, 100, 2, now, fs, logging.NewCapture()); err != nil {
		t.Fatal(err)
	}
	if len(fs.removed) != 1 || fs.removed[0] != baseDir+"/KW06_20250204-2000" {
		t.Errorf("weekly retention removed=%v, want only oldest week KW06", fs.removed)
	}
	if len(fs.renames) != 0 {
		t.Errorf("unexpected renames: %v", fs.renames)
	}
}

func TestWeeklyRetentionDistinguishesYears(t *testing.T) {
	// Same ISO week number (01) in two different years must be treated as two
	// distinct weeks, not merged into one bucket. With weekly=1 the older year
	// is deleted and the newer kept. Tuesday/Thursday dates avoid the Sunday
	// rename step.
	fs := newFakeFS(
		"KW01_20240102-2000", // ISO 2024-W01 (Tue)
		"KW01_20250102-2000", // ISO 2025-W01 (Thu)
	)
	now := mustDate(t, "2025-02-20")
	if err := Rotate(baseDir, 100, 1, now, fs, logging.NewCapture()); err != nil {
		t.Fatal(err)
	}
	if len(fs.removed) != 1 || fs.removed[0] != baseDir+"/KW01_20240102-2000" {
		t.Errorf("removed=%v, want only the older year KW01_20240102-2000", fs.removed)
	}
}

func TestAgeDeleteAcrossDSTBoundary(t *testing.T) {
	// Between two local midnights that straddle a spring-forward transition the
	// span is 47h; truncating /24 would yield age 1 (a day short) and wrongly
	// keep a backup that is 2 calendar days old. Rounding matches Python's
	// date subtraction.
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Skipf("no tzdata for Europe/Berlin: %v", err)
	}
	fs := newFakeFS("20250329-2000") // Sat, day before DST switch (2025-03-30)
	now := time.Date(2025, 3, 31, 12, 0, 0, 0, loc)
	if err := Rotate(baseDir, 1, 100, now, fs, logging.NewCapture()); err != nil {
		t.Fatal(err)
	}
	if len(fs.removed) != 1 || fs.removed[0] != baseDir+"/20250329-2000" {
		t.Errorf("age across DST: removed=%v, want /20250329-2000 (age 2 > daily 1)", fs.removed)
	}
}

func TestNonDirEntriesIgnored(t *testing.T) {
	fs := &fakeFS{exists: map[string]bool{}}
	fs.entries = []Entry{{Name: "20250210-2000", IsDir: false}} // a file, not a dir
	now := mustDate(t, "2025-02-20")
	if err := Rotate(baseDir, 1, 1, now, fs, logging.NewCapture()); err != nil {
		t.Fatal(err)
	}
	if len(fs.removed) != 0 || len(fs.renames) != 0 {
		t.Errorf("non-dir entry should be ignored; removed=%v renames=%v", fs.removed, fs.renames)
	}
}
