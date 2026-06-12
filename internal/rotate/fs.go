package rotate

import (
	"os"

	"bodsch.me/mariadb-backup/internal/logging"
)

// OSFileSystem is the production FileSystem backed by the os package.
type OSFileSystem struct{}

func (OSFileSystem) ReadDir(dir string) ([]Entry, error) {
	des, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := make([]Entry, 0, len(des))
	for _, de := range des {
		out = append(out, Entry{Name: de.Name(), IsDir: de.IsDir()})
	}
	return out, nil
}

func (OSFileSystem) Rename(oldPath, newPath string) error { return os.Rename(oldPath, newPath) }
func (OSFileSystem) RemoveAll(path string) error          { return os.RemoveAll(path) }
func (OSFileSystem) Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// DryRunFileSystem reads real directory contents but only logs the mutations it
// would perform, changing nothing on disk.
type DryRunFileSystem struct {
	Log logging.Logger
}

func (DryRunFileSystem) ReadDir(dir string) ([]Entry, error) {
	return OSFileSystem{}.ReadDir(dir)
}

func (d DryRunFileSystem) Rename(oldPath, newPath string) error {
	d.Log.Info("   would rename %s -> %s", oldPath, newPath)
	return nil
}

func (d DryRunFileSystem) RemoveAll(path string) error {
	d.Log.Info("   would delete %s", path)
	return nil
}

func (DryRunFileSystem) Exists(path string) bool {
	return OSFileSystem{}.Exists(path)
}
