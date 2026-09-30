package main

import (
	"bytes"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// sffCacheReset starts a fresh cache stored at path (tests only).
func sffCacheReset(path string) {
	sffCache.once = sync.Once{}
	sffCache.path = path
	sffCache.entries, sffCache.used = nil, nil
	sffCache.dirty, sffCache.saved = false, 0
}

func TestSffCacheRecordReplay(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "char.sff")
	data := make([]byte, 300_000)
	rand.New(rand.NewSource(7)).Read(data)
	if err := os.WriteFile(file, data, 0o644); err != nil {
		t.Fatal(err)
	}
	cachePath := filepath.Join(dir, "cache", "sffheaders.bin")
	saved := sffCache.path
	t.Cleanup(func() { sffCacheReset(saved) })

	// A walk-like pattern: small reads spread over the file, a backward
	// byte scan, and one large (pixel data) read.
	type op struct{ off, n int }
	var ops []op
	for off := 0; off+33 < len(data); off += 7919 {
		ops = append(ops, op{off, 32}, op{off + 32, 1})
	}
	for off := 150_000; off > 149_000; off-- {
		ops = append(ops, op{off, 1})
	}
	large := op{200_000, 50_000}
	run := func() (*sffCachedReader, func(bool)) {
		f, commit, err := sffCacheOpen(file)
		if err != nil {
			t.Fatal(err)
		}
		cr, ok := f.(*sffCachedReader)
		if !ok {
			t.Fatalf("got %T, want the cached reader", f)
		}
		for _, o := range ops {
			p := make([]byte, o.n)
			if _, err := cr.Seek(int64(o.off), io.SeekStart); err != nil {
				t.Fatal(err)
			}
			if _, err := io.ReadFull(cr, p); err != nil || !bytes.Equal(p, data[o.off:o.off+o.n]) {
				t.Fatalf("read %d@%d: %v", o.n, o.off, err)
			}
		}
		return cr, commit
	}

	sffCacheReset(cachePath)
	cr, commit := run()
	if cr.entry != nil || cr.file == nil {
		t.Fatalf("first run must read the file")
	}
	cr.Close()
	commit(true)
	sffCacheSave()
	if _, err := os.Stat(cachePath); err != nil {
		t.Fatalf("cache not written: %v", err)
	}

	// Next start: every small read comes from the cache, the file stays closed.
	sffCacheReset(cachePath)
	cr, commit = run()
	if cr.entry == nil || cr.file != nil {
		t.Fatalf("replay must not open the file (entry %v, file %v)", cr.entry != nil, cr.file != nil)
	}
	p := make([]byte, large.n)
	cr.Seek(int64(large.off), io.SeekStart)
	if _, err := io.ReadFull(cr, p); err != nil || !bytes.Equal(p, data[large.off:large.off+large.n]) {
		t.Fatalf("large read: %v", err)
	}
	cr.Close()
	commit(true)
	if sffCache.dirty {
		t.Errorf("large reads must not be cached")
	}

	// A changed file is read again.
	later := time.Now().Add(time.Hour)
	os.Chtimes(file, later, later)
	sffCacheReset(cachePath)
	cr, _ = run()
	if cr.entry != nil {
		t.Errorf("an entry for a modified file must be ignored")
	}
	cr.Close()
}

func TestMergeSffCacheRanges(t *testing.T) {
	got := mergeSffCacheRanges([]sffCacheRange{
		{10, []byte("klm")}, {0, []byte("abc")}, {3, []byte("de")}, {12, []byte("mnop")}, {20, []byte("x")},
	})
	want := []sffCacheRange{{0, []byte("abcde")}, {10, []byte("klmnop")}, {20, []byte("x")}}
	if len(got) != len(want) {
		t.Fatalf("got %d ranges, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].off != want[i].off || !bytes.Equal(got[i].data, want[i].data) {
			t.Errorf("range %d: %d %q, want %d %q", i, got[i].off, got[i].data, want[i].off, want[i].data)
		}
	}
}
