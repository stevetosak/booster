package jsonfile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteAndRead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "new", "x.json")
	var v struct{ A int }
	if ok, err := Read(path, &v); ok || err != nil {
		t.Fatalf("Read of no file: %v, %v", ok, err)
	}
	if err := Write(path, map[string]int{"A": 7}); err != nil {
		t.Fatal(err)
	}
	if ok, err := Read(path, &v); !ok || err != nil || v.A != 7 {
		t.Fatalf("Read: %v, %v, %+v", ok, err, v)
	}
	info, _ := os.Stat(path)
	dir, _ := os.Stat(filepath.Dir(path))
	if info.Mode().Perm() != 0o600 || dir.Mode().Perm() != 0o700 {
		t.Errorf("modes: file %v, folder %v", info.Mode().Perm(), dir.Mode().Perm())
	}
	if left, _ := filepath.Glob(filepath.Join(filepath.Dir(path), ".*")); len(left) != 0 {
		t.Errorf("temporary files left: %v", left)
	}
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(path, &v); err == nil {
		t.Error("Read of a broken file: no error")
	}
}
