package main

import (
	"bytes"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type nopCloseReadSeeker struct{ *bytes.Reader }

func (nopCloseReadSeeker) Close() error { return nil }

// loadFirstIniSection must yield the same [Size] values as parsing the whole file.
func TestLoadFirstIniSection(t *testing.T) {
	text := "\uFEFF; comment\r\n[Data]\r\nlife = 1000\r\n[ Size ]\r\n[Size] ; first\r\n  xscale = 0.5 ;x\r\nyscale=.75\r\n[Size]\r\nxscale = 9\r\n[Statedef 0]\r\ntype = S\r\n"
	path := t.TempDir() + "/test.cns"
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	lines, err := loadFirstIniSection(path, "size")
	if err != nil {
		t.Fatal(err)
	}
	// Reference: the old whole-file parse.
	full := SplitAndTrim(strings.TrimPrefix(text, "\uFEFF"), "\n")
	var want IniSection
	for i := 0; i < len(full); {
		is, name, _ := ReadIniSection(full, &i)
		if name == "size" {
			want = is
			break
		}
		if name == "" {
			break
		}
	}
	i := 0
	got, name, _ := ReadIniSection(lines, &i)
	if name != "size" || got["xscale"] != want["xscale"] || got["yscale"] != want["yscale"] || got["xscale"] != "0.5" {
		t.Fatalf("got %v (%q), want %v", got, name, want)
	}
}

// On real character files, the streamed [Size] section must parse to the same
// xscale/yscale as the old whole-file parse.
func TestLoadFirstIniSectionRealCns(t *testing.T) {
	files, _ := filepath.Glob("D:/Jump Force Mugen/chars/*/*.cns")
	if len(files) == 0 {
		t.Skip("Jump Force MUGEN not present")
	}
	if len(files) > 500 {
		files = files[:500]
	}
	sizeOf := func(lines []string) (IniSection, bool) {
		for i := 0; i < len(lines); {
			is, name, _ := ReadIniSection(lines, &i)
			if name == "size" {
				return is, true
			}
			if name == "" {
				break
			}
		}
		return nil, false
	}
	checked := 0
	for _, f := range files {
		str, err := LoadText(f)
		if err != nil {
			continue
		}
		want, wantOK := sizeOf(SplitAndTrim(str, "\n"))
		lines, err := loadFirstIniSection(f, "size")
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		got, gotOK := sizeOf(lines)
		if wantOK != gotOK || got["xscale"] != want["xscale"] || got["yscale"] != want["yscale"] {
			t.Errorf("%s: streamed %v/%v, whole-file %v/%v", f, got, gotOK, want, wantOK)
		}
		checked++
	}
	t.Logf("compared [Size] in %d .cns files", checked)
}

// The buffered reader must return exactly what the plain reader returns for
// any mix of seeks and reads (including across buffer and file boundaries).
func TestBufReadSeekerMatchesPlainReader(t *testing.T) {
	data := make([]byte, 100_003)
	rng := rand.New(rand.NewSource(1))
	rng.Read(data)
	plain := bytes.NewReader(data)
	buf := newBufReadSeeker(nopCloseReadSeeker{bytes.NewReader(data)}, 4096)

	for i := 0; i < 20000; i++ {
		switch rng.Intn(4) {
		case 0:
			off := rng.Int63n(int64(len(data)) + 50)
			a, _ := plain.Seek(off, io.SeekStart)
			b, _ := buf.Seek(off, io.SeekStart)
			if a != b {
				t.Fatalf("seek start: %d vs %d", a, b)
			}
		case 1:
			off := rng.Int63n(200) - 100
			cur, _ := plain.Seek(0, io.SeekCurrent)
			if cur+off < 0 {
				off = -cur
			}
			a, _ := plain.Seek(off, io.SeekCurrent)
			b, _ := buf.Seek(off, io.SeekCurrent)
			if a != b {
				t.Fatalf("seek cur: %d vs %d", a, b)
			}
		default:
			n := rng.Intn(10000)
			if rng.Intn(10) == 0 {
				n = 1
			}
			p1, p2 := make([]byte, n), make([]byte, n)
			n1, e1 := io.ReadFull(plain, p1)
			n2, e2 := io.ReadFull(buf, p2)
			if n1 != n2 || (e1 == nil) != (e2 == nil) || !bytes.Equal(p1[:n1], p2[:n2]) {
				t.Fatalf("read %d at step %d: plain %d/%v, buffered %d/%v", n, i, n1, e1, n2, e2)
			}
		}
	}
	a, _ := plain.Seek(-10, io.SeekEnd)
	b, _ := buf.Seek(-10, io.SeekEnd)
	if a != b {
		t.Fatalf("seek end: %d vs %d", a, b)
	}
}
