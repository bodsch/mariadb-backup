package dump

import (
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"bodsch.me/mariadb-backup/internal/logging"
)

type fakeRunner struct {
	rc      int
	stderr  string
	err     error
	content string
	calls   [][]string
}

func (f *fakeRunner) Run(name string, args []string, out io.Writer) (int, string, error) {
	f.calls = append(f.calls, args)
	if f.content != "" {
		io.WriteString(out, f.content)
	}
	return f.rc, f.stderr, f.err
}

func mkdirReal(dir string) error { return os.MkdirAll(dir, 0o755) }

func TestBuildArgsSchema(t *testing.T) {
	args := buildArgs(schemaOnly, "mydb", "/tmp/x.cnf", true, []string{"mydb.logs"})
	joined := strings.Join(args, " ")
	if args[0] != "--defaults-extra-file=/tmp/x.cnf" {
		t.Errorf("defaults-extra-file must be first: %v", args)
	}
	for _, want := range []string{"--quote-names", "--opt", "--events", "--routines", "--single-transaction", "--no-data", "--skip-ssl", "--ignore-table=mydb.logs", "--databases mydb"} {
		if !strings.Contains(joined, want) {
			t.Errorf("schema args missing %q: %s", want, joined)
		}
	}
	if strings.Contains(joined, "--no-create-info") {
		t.Errorf("schema must not contain --no-create-info: %s", joined)
	}
}

func TestBuildArgsData(t *testing.T) {
	args := buildArgs(dataOnly, "mydb", "", false, nil)
	joined := strings.Join(args, " ")
	if strings.Contains(joined, "--defaults-extra-file") {
		t.Errorf("no defaults file expected: %s", joined)
	}
	if !strings.Contains(joined, "--no-create-info") || strings.Contains(joined, "--no-data") {
		t.Errorf("data args wrong: %s", joined)
	}
	if strings.Contains(joined, "--skip-ssl") {
		t.Errorf("skip-ssl should be absent: %s", joined)
	}
}

func TestDumpDatabaseSuccess(t *testing.T) {
	out := t.TempDir()
	d := &Dumper{Log: logging.NewCapture(), Runner: &fakeRunner{content: "-- sql\n"}}
	res, err := d.DumpDatabase(out, "mydb", mkdirReal)
	if err != nil {
		t.Fatal(err)
	}
	if res.Failed {
		t.Errorf("rc 0 should not fail")
	}
	for _, name := range []string{"schema.sql", "data.sql"} {
		p := filepath.Join(out, "mydb", name)
		if _, err := os.Stat(p); err != nil {
			t.Errorf("expected %s: %v", p, err)
		}
	}
}

func TestDumpDatabaseExitCodeOneIsTolerated(t *testing.T) {
	d := &Dumper{Log: logging.NewCapture(), Runner: &fakeRunner{rc: 1}}
	res, err := d.DumpDatabase(t.TempDir(), "mydb", mkdirReal)
	if err != nil {
		t.Fatal(err)
	}
	if res.Failed {
		t.Errorf("rc 1 must be tolerated (not Failed)")
	}
}

func TestDumpDatabaseExitCodeTwoFails(t *testing.T) {
	d := &Dumper{Log: logging.NewCapture(), Runner: &fakeRunner{rc: 2, stderr: "boom"}}
	res, err := d.DumpDatabase(t.TempDir(), "mydb", mkdirReal)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Failed {
		t.Errorf("rc 2 must mark Failed")
	}
}

func TestDumpDatabaseBinaryNotFoundIsFatal(t *testing.T) {
	d := &Dumper{Log: logging.NewCapture(), Runner: &fakeRunner{err: ErrBinaryNotFound}}
	_, err := d.DumpDatabase(t.TempDir(), "mydb", mkdirReal)
	if err == nil {
		t.Fatal("expected fatal error when binary not found")
	}
}

func TestDumpDatabaseBinaryNotFoundLeavesNoFile(t *testing.T) {
	out := t.TempDir()
	d := &Dumper{Log: logging.NewCapture(), Runner: &fakeRunner{err: ErrBinaryNotFound}}
	if _, err := d.DumpDatabase(out, "mydb", mkdirReal); err == nil {
		t.Fatal("expected fatal error when binary not found")
	}
	// The empty file created before the binary failed must not be left behind.
	p := filepath.Join(out, "mydb", "schema.sql")
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Errorf("incomplete dump file %s should have been removed (err=%v)", p, err)
	}
}

func TestDumpDatabaseGzip(t *testing.T) {
	out := t.TempDir()
	d := &Dumper{Compress: true, Log: logging.NewCapture(), Runner: &fakeRunner{content: "-- sql data\n"}}
	if _, err := d.DumpDatabase(out, "mydb", mkdirReal); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(out, "mydb", "schema.sql.gz")
	f, err := os.Open(p)
	if err != nil {
		t.Fatalf("expected gz file: %v", err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("not a valid gzip: %v", err)
	}
	data, _ := io.ReadAll(gz)
	if !strings.Contains(string(data), "sql data") {
		t.Errorf("gzip content wrong: %q", data)
	}
}

func TestDumpDatabaseDryRun(t *testing.T) {
	out := t.TempDir()
	log := logging.NewCapture()
	d := &Dumper{DryRun: true, Log: log, Runner: &fakeRunner{}}
	res, err := d.DumpDatabase(out, "mydb", func(string) error { t.Fatal("mkdir must not be called in dry-run"); return nil })
	if err != nil || res.Failed {
		t.Fatalf("dry-run unexpected err/fail: %v %v", err, res.Failed)
	}
	if _, err := os.Stat(filepath.Join(out, "mydb")); !os.IsNotExist(err) {
		t.Errorf("dry-run must not create directories")
	}
	if !log.Contains("would dump") {
		t.Errorf("dry-run should log intentions: %v", log.Messages())
	}
}
