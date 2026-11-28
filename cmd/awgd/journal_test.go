package main

import (
	"os"
	"path/filepath"
	"testing"

	"awg31/internal/journal"
)

func TestImportEntrypointLog(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "entrypoint.log"), []byte("1795700000 architecture: x86_64\nmusor\n1795700001 ip rule 'lookup local' was missing, restored\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	j := journal.New(10)
	importEntrypointLog(j, dir)
	got, _ := j.Since(0, 0)
	if len(got) != 2 || got[0].Text != "[entrypoint] architecture: x86_64" || got[0].Time.Unix() != 1795700000 || got[1].Level != journal.Warn {
		t.Fatalf("%+v", got)
	}
	importEntrypointLog(j, filepath.Join(dir, "none")) // нет файла — тихо
}
