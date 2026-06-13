// Package rotate reproduces the retention logic of the Python script's
// rotate_directories and get_creation_date — including its quirks (the
// hardcoded max of 5 backups per calendar day, and the fact that a directory
// renamed to a weekly KW<week>_ backup is still referenced by its original
// name/path for the remainder of the run). Behaviour is deliberately preserved
// because altering backup deletion is risky, with one fix over the Python
// original: directories that are already weekly KW<week>_ backups are no longer
// re-prefixed on each run (the Python version produced KW20_KW20_... names).
package rotate

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"bodsch.me/mariadb-backup/internal/logging"
)

// maxDailyPerDay is the hardcoded per-calendar-day cap from the Python script
// (NOT the configurable rotation.daily, which only governs age-based deletion).
const maxDailyPerDay = 5

// Entry is a directory entry as seen by the FileSystem.
type Entry struct {
	Name  string
	IsDir bool
}

// FileSystem abstracts the directory operations rotation needs, so the logic is
// unit-testable and so dry-run can swap in a no-op implementation that only
// logs intended changes.
type FileSystem interface {
	ReadDir(dir string) ([]Entry, error)
	Rename(oldPath, newPath string) error
	RemoveAll(path string) error
	Exists(path string) bool
}

// creationDate extracts the backup date from a directory name in the format
// "%Y%m%d-%H%M", stripping an optional weekly "KW<week>_" prefix. The returned
// time is midnight in loc. ok is false if the name does not parse.
func creationDate(name string, loc *time.Location) (t time.Time, ok bool) {
	n := name
	if strings.HasPrefix(n, "KW") {
		if i := strings.Index(n, "_"); i >= 0 {
			n = n[i+1:]
		}
	}
	if len(n) < 13 {
		return time.Time{}, false
	}
	parsed, err := time.ParseInLocation("20060102-1504", n[:13], loc)
	if err != nil {
		return time.Time{}, false
	}
	// Reduce to the date (midnight) for age/weekday/week calculations.
	return time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 0, 0, 0, 0, loc), true
}

type dirEntry struct {
	name    string // original on-disk name (used throughout, even after rename)
	path    string // original path (deletes target this, matching Python)
	date    time.Time
	weekKey string // ISO year+week ("2025-07"), so weeks in different years never collide
}

// Rotate applies the retention policy to baseDir. now is injected for
// deterministic tests; fs performs (or, in dry-run, only logs) the mutations.
func Rotate(baseDir string, daily, weekly int, now time.Time, fs FileSystem, log logging.Logger) error {
	loc := now.Location()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)

	entries, err := fs.ReadDir(baseDir)
	if err != nil {
		return fmt.Errorf("read backup dir %s: %w", baseDir, err)
	}

	// Stable order by name, matching Python's sorted(..., key=name).
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })

	dailyBackups := map[time.Time][]dirEntry{} // keyed by date
	weeklyBackups := map[string][]dirEntry{}   // keyed by ISO year+week string

	for _, e := range entries {
		if !e.IsDir {
			continue
		}
		date, ok := creationDate(e.Name, loc)
		if !ok {
			continue
		}
		// Age in whole calendar days, rounded rather than truncated: between two
		// local midnights a DST transition makes the span 23h or 25h, which
		// truncating division by 24 would push to the wrong day. Matches
		// Python's date-based subtraction.
		ageDays := int(math.Round(today.Sub(date).Hours() / 24))
		isoYear, isoWeek := date.ISOWeek()
		week := fmt.Sprintf("%02d", isoWeek)
		// Bucket key carries the ISO year so week 52 of two different years (or
		// week 01 across a year boundary) never collide in weekly retention.
		weekKey := fmt.Sprintf("%04d-%02d", isoYear, isoWeek)
		path := joinPath(baseDir, e.Name)

		entry := dirEntry{name: e.Name, path: path, date: date, weekKey: weekKey}

		// 1. Group for the per-day cap (every dir, including KW ones).
		dailyBackups[date] = append(dailyBackups[date], entry)

		// 2. Rename Sunday backups to a weekly KW<week>_ name. Skip dirs that
		//    are already weekly backups, otherwise their existing KW<week>_
		//    prefix would be doubled (KW20_KW20_...) on every run.
		if date.Weekday() == time.Sunday && !strings.HasPrefix(e.Name, "KW") {
			newName := fmt.Sprintf("KW%s_%s", week, e.Name)
			newPath := joinPath(baseDir, newName)
			if !fs.Exists(newPath) {
				if err := fs.Rename(path, newPath); err != nil {
					log.Error("could not rename %s to %s: %v", e.Name, newName, err)
				} else {
					log.Info("rename backup %s to %s", e.Name, newName)
				}
			}
		}

		// 3. Delete non-KW dirs older than the daily age. (Operates on the
		//    original name/path, exactly like Python — so a dir just renamed in
		//    step 2 logs here but its old path no longer exists, a no-op.)
		if ageDays > daily && !strings.HasPrefix(e.Name, "KW") {
			deleteDir(fs, path, log)
			log.Info("remove older backup %s.", e.Name)
		}

		// 4. Collect dirs whose original name is already a weekly backup.
		if strings.HasPrefix(e.Name, "KW") {
			weeklyBackups[weekKey] = append(weeklyBackups[weekKey], entry)
		}
	}

	// 5. Keep at most 5 backups per calendar day (oldest by name deleted).
	for _, backups := range dailyBackups {
		if len(backups) > maxDailyPerDay {
			sort.Slice(backups, func(i, j int) bool { return backups[i].name < backups[j].name })
			toDelete := backups[:len(backups)-maxDailyPerDay]
			for _, old := range toDelete {
				deleteDir(fs, old.path, log)
				log.Info("delete excess daily backup %s", old.name)
			}
		}
	}

	// 6. Keep only the newest `weekly` weeks.
	weeks := make([]string, 0, len(weeklyBackups))
	for w := range weeklyBackups {
		weeks = append(weeks, w)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(weeks)))
	if weekly < len(weeks) {
		for _, w := range weeks[weekly:] {
			for _, entry := range weeklyBackups[w] {
				log.Info("delete weekly backup %s", entry.name)
				deleteDir(fs, entry.path, log)
			}
		}
	}
	return nil
}

func deleteDir(fs FileSystem, path string, log logging.Logger) {
	if !fs.Exists(path) {
		return
	}
	if err := fs.RemoveAll(path); err != nil {
		log.Error("could not delete %s: %v", path, err)
	}
}

func joinPath(base, name string) string {
	if strings.HasSuffix(base, "/") {
		return base + name
	}
	return base + "/" + name
}
