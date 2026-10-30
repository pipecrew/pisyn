package runner

import (
	"archive/tar"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// TestTarDirSymlink verifies that tarDir records symlinks as
// Typeflag=Symlink with the correct link target instead of writing
// a zero-content regular file (the bug reported in issue #8).
func TestTarDirSymlink(t *testing.T) {
	dir := t.TempDir()

	target := filepath.Join(dir, "real.txt")
	if err := os.WriteFile(target, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink("real.txt", link); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	r := tarDir(dir)

	var sawSymlink, sawRegular bool
	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("tar read: %v", err)
		}
		switch hdr.Name {
		case "link":
			if hdr.Typeflag != tar.TypeSymlink {
				t.Errorf("link: typeflag=%v want %v", hdr.Typeflag, tar.TypeSymlink)
			}
			if hdr.Linkname != "real.txt" {
				t.Errorf("link: linkname=%q want %q", hdr.Linkname, "real.txt")
			}
			if hdr.Size != 0 {
				t.Errorf("link: size=%d want 0", hdr.Size)
			}
			sawSymlink = true
		case "real.txt":
			if hdr.Typeflag != tar.TypeReg {
				t.Errorf("real.txt: typeflag=%v want %v", hdr.Typeflag, tar.TypeReg)
			}
			sawRegular = true
		}
	}
	if !sawSymlink {
		t.Error("symlink entry missing from tar")
	}
	if !sawRegular {
		t.Error("regular-file entry missing from tar")
	}
}

// TestTarDirBrokenSymlink verifies that tarDir surfaces a clear error
// rather than producing a zero-content regular file when a symlink
// target cannot be resolved.
func TestTarDirBrokenSymlink(t *testing.T) {
	dir := t.TempDir()

	// Symlink whose target does not exist.
	link := filepath.Join(dir, "broken")
	if err := os.Symlink("does-not-exist", link); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	r := tarDir(dir)

	var sawSymlink bool
	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("tar read: %v", err)
		}
		if hdr.Name == "broken" {
			sawSymlink = true
			if hdr.Typeflag != tar.TypeSymlink {
				t.Errorf("broken: typeflag=%v want %v", hdr.Typeflag, tar.TypeSymlink)
			}
			if hdr.Linkname != "does-not-exist" {
				t.Errorf("broken: linkname=%q want %q", hdr.Linkname, "does-not-exist")
			}
		}
	}
	if !sawSymlink {
		t.Error("broken symlink entry missing from tar")
	}
}

// TestTarDirSkipsEscapingSymlinks verifies that symlinks pointing outside the
// project tree (absolute or via "..") are left out of the archive.
func TestTarDirSkipsEscapingSymlinks(t *testing.T) {
	dir := t.TempDir()
	if err := os.Symlink("/etc/passwd", filepath.Join(dir, "abs")); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Symlink("../../outside", filepath.Join(dir, "sub", "up")); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	if err := os.Symlink("../sub", filepath.Join(dir, "sub", "inside")); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	names := map[string]bool{}
	tr := tar.NewReader(tarDir(dir))
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("tar read: %v", err)
		}
		names[hdr.Name] = true
	}
	if names["abs"] {
		t.Error("absolute symlink should be skipped")
	}
	if names[filepath.Join("sub", "up")] {
		t.Error("escaping relative symlink should be skipped")
	}
	if !names[filepath.Join("sub", "inside")] {
		t.Error("in-tree symlink should be kept")
	}
}
