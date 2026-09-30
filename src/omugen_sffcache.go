package main

// SFF header cache for select-screen preloading.
//
// An SFFv1 file stores each sprite's header right after the previous
// sprite's data, so finding a character's portraits walks the file with one
// disk read per sprite (thousands per character, and to the end of the file
// when a requested sprite doesn't exist). The cache remembers the small reads
// such a walk makes (sprite headers, palette bytes) and replays them from
// memory on the next start; only the portraits' own pixel data, and anything
// else not in the cache, is read from the file.
//
// Entries are keyed by file path, size and modification time. A read that the
// cache can't serve always comes from the file, so an incomplete entry costs
// time, never correctness. The cache is save/cache/sffheaders.bin, written
// when background loading finishes.

import (
	"bufio"
	"bytes"
	"compress/flate"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

const (
	sffCachePath    = "save/cache/sffheaders.bin"
	sffCacheMagic   = "OMSFFC\x00\x01"
	sffCacheMaxRead = 4096 // larger reads (sprite pixels) always come from the file
)

type sffCacheRange struct {
	off  int64
	data []byte
}

// sffCacheEntry is immutable once published: updates replace it.
type sffCacheEntry struct {
	size, mtime int64
	ranges      []sffCacheRange // sorted by offset, not overlapping
}

var sffCache struct {
	once    sync.Once
	mu      sync.Mutex
	path    string
	saveMu  sync.Mutex
	saved   int // entries in the file on disk
	entries map[string]*sffCacheEntry
	used    map[string]bool // entries needed this run (the others are dropped)
	dirty   bool
}

func sffCacheLoad() {
	sffCache.entries = map[string]*sffCacheEntry{}
	sffCache.used = map[string]bool{}
	if sffCache.path == "" {
		sffCache.path = sffCachePath
	}
	data, err := os.ReadFile(sffCache.path)
	if err != nil || !bytes.HasPrefix(data, []byte(sffCacheMagic)) {
		return
	}
	entries, err := sffCacheDecode(data[len(sffCacheMagic):])
	if err != nil {
		LogMessage("OMugen: ignoring the SFF cache (%v)", err)
		return
	}
	sffCache.entries = entries
	sffCache.saved = len(entries)
}

func sffCacheDecode(data []byte) (map[string]*sffCacheEntry, error) {
	r := bufio.NewReader(flate.NewReader(bytes.NewReader(data)))
	rd := func(x interface{}) error { return binary.Read(r, binary.LittleEndian, x) }
	var n uint32
	if err := rd(&n); err != nil {
		return nil, err
	}
	entries := make(map[string]*sffCacheEntry, n)
	for i := uint32(0); i < n; i++ {
		var plen uint16
		if err := rd(&plen); err != nil {
			return nil, err
		}
		p := make([]byte, plen)
		if _, err := io.ReadFull(r, p); err != nil {
			return nil, err
		}
		e := &sffCacheEntry{}
		var nr uint32
		if err := rd(&e.size); err != nil {
			return nil, err
		}
		if err := rd(&e.mtime); err != nil {
			return nil, err
		}
		if err := rd(&nr); err != nil {
			return nil, err
		}
		e.ranges = make([]sffCacheRange, nr)
		for j := range e.ranges {
			var l uint32
			if err := rd(&e.ranges[j].off); err != nil {
				return nil, err
			}
			if err := rd(&l); err != nil {
				return nil, err
			}
			if l > 1<<24 {
				return nil, errors.New("corrupt range")
			}
			e.ranges[j].data = make([]byte, l)
			if _, err := io.ReadFull(r, e.ranges[j].data); err != nil {
				return nil, err
			}
		}
		entries[string(p)] = e
	}
	return entries, nil
}

func sffCacheEncode(w io.Writer, entries map[string]*sffCacheEntry) error {
	if _, err := io.WriteString(w, sffCacheMagic); err != nil {
		return err
	}
	fw, err := flate.NewWriter(w, flate.BestSpeed)
	if err != nil {
		return err
	}
	bw := bufio.NewWriter(fw)
	wr := func(x interface{}) { binary.Write(bw, binary.LittleEndian, x) }
	names := make([]string, 0, len(entries))
	for k := range entries {
		names = append(names, k)
	}
	sort.Strings(names)
	wr(uint32(len(names)))
	for _, name := range names {
		e := entries[name]
		wr(uint16(len(name)))
		bw.WriteString(name)
		wr(e.size)
		wr(e.mtime)
		wr(uint32(len(e.ranges)))
		for _, r := range e.ranges {
			wr(r.off)
			wr(uint32(len(r.data)))
			bw.Write(r.data)
		}
	}
	if err := bw.Flush(); err != nil {
		return err
	}
	return fw.Close()
}

// sffCacheSave writes the entries used this run, if anything changed. Called
// when background loading finishes.
func sffCacheSave() {
	sffCache.saveMu.Lock()
	defer sffCache.saveMu.Unlock()
	sffCache.mu.Lock()
	if sffCache.entries == nil {
		sffCache.mu.Unlock()
		return
	}
	keep := make(map[string]*sffCacheEntry, len(sffCache.used))
	for k := range sffCache.used {
		if e := sffCache.entries[k]; e != nil {
			keep[k] = e
		}
	}
	changed := sffCache.dirty || len(keep) != sffCache.saved
	sffCache.dirty = false
	sffCache.saved = len(keep)
	path := sffCache.path
	sffCache.mu.Unlock()
	if !changed {
		return
	}
	if err := writeFileAtomic(path, func(w io.Writer) error { return sffCacheEncode(w, keep) }); err != nil {
		LogMessage("OMugen: could not save the SFF cache: %v", err)
	}
}

func writeFileAtomic(path string, write func(io.Writer) error) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	bw := bufio.NewWriterSize(f, 1<<20)
	if err := write(bw); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := bw.Flush(); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

// sffCacheOpen opens an SFF for preloading. commit(keep) must be called after
// a successful load; keep says whether the file is worth caching (SFFv1).
func sffCacheOpen(filename string) (f io.ReadSeekCloser, commit func(keep bool), err error) {
	plain := func() (io.ReadSeekCloser, func(bool), error) {
		raw, err := OpenFile(filename)
		if err != nil {
			return nil, nil, err
		}
		return newBufReadSeeker(raw, 256<<10), func(bool) {}, nil
	}
	if isZip, _, _ := IsZipPath(filename); isZip {
		return plain()
	}
	fi, err := os.Stat(filename)
	if err != nil {
		return plain()
	}
	sffCache.once.Do(sffCacheLoad)
	sffCache.mu.Lock()
	e := sffCache.entries[filename]
	sffCache.mu.Unlock()
	size, mtime := fi.Size(), fi.ModTime().UnixNano()
	if e != nil && (e.size != size || e.mtime != mtime) {
		e = nil
	}
	cr := &sffCachedReader{name: filename, size: size, entry: e}
	return cr, func(keep bool) { cr.commit(keep, size, mtime) }, nil
}

// sffCachedReader serves reads from a cache entry, falling back to the file
// (opened on first need), and records the small reads that missed.
type sffCachedReader struct {
	name  string
	size  int64
	pos   int64
	entry *sffCacheEntry
	file  io.ReadSeekCloser
	rec   []sffCacheRange
}

func (c *sffCachedReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if c.entry != nil {
		rs := c.entry.ranges
		i := sort.Search(len(rs), func(i int) bool { return rs[i].off+int64(len(rs[i].data)) > c.pos })
		if i < len(rs) && rs[i].off <= c.pos {
			n := copy(p, rs[i].data[c.pos-rs[i].off:])
			c.pos += int64(n)
			return n, nil
		}
	}
	if c.file == nil {
		raw, err := OpenFile(c.name)
		if err != nil {
			return 0, err
		}
		c.file = newBufReadSeeker(raw, 256<<10)
	}
	if _, err := c.file.Seek(c.pos, io.SeekStart); err != nil {
		return 0, err
	}
	n, err := c.file.Read(p)
	if n > 0 && len(p) <= sffCacheMaxRead {
		// Consecutive reads (the fields of one header) extend the last range.
		if k := len(c.rec) - 1; k >= 0 && c.rec[k].off+int64(len(c.rec[k].data)) == c.pos {
			c.rec[k].data = append(c.rec[k].data, p[:n]...)
		} else {
			c.rec = append(c.rec, sffCacheRange{off: c.pos, data: append([]byte(nil), p[:n]...)})
		}
	}
	c.pos += int64(n)
	return n, err
}

func (c *sffCachedReader) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		c.pos = offset
	case io.SeekCurrent:
		c.pos += offset
	case io.SeekEnd:
		c.pos = c.size + offset
	default:
		return 0, errors.New("sffCachedReader: invalid whence")
	}
	if c.pos < 0 {
		c.pos = 0
		return 0, errors.New("sffCachedReader: negative position")
	}
	return c.pos, nil
}

func (c *sffCachedReader) Close() error {
	if c.file != nil {
		return c.file.Close()
	}
	return nil
}

func (c *sffCachedReader) commit(keep bool, size, mtime int64) {
	sffCache.mu.Lock()
	defer sffCache.mu.Unlock()
	if !keep {
		return
	}
	sffCache.used[c.name] = true
	if len(c.rec) == 0 && c.entry != nil {
		return
	}
	var all []sffCacheRange
	if cur := sffCache.entries[c.name]; cur != nil && cur.size == size && cur.mtime == mtime {
		all = append(all, cur.ranges...)
	}
	all = append(all, c.rec...)
	sffCache.entries[c.name] = &sffCacheEntry{size: size, mtime: mtime, ranges: mergeSffCacheRanges(all)}
	sffCache.dirty = true
}

// mergeSffCacheRanges sorts ranges and joins overlapping or touching ones.
func mergeSffCacheRanges(rs []sffCacheRange) []sffCacheRange {
	sort.Slice(rs, func(i, j int) bool { return rs[i].off < rs[j].off })
	var out []sffCacheRange
	for _, r := range rs {
		if n := len(out); n > 0 {
			last := &out[n-1]
			end := last.off + int64(len(last.data))
			if r.off <= end {
				if rEnd := r.off + int64(len(r.data)); rEnd > end {
					last.data = append(last.data[:len(last.data):len(last.data)], r.data[end-r.off:]...)
				}
				continue
			}
		}
		out = append(out, sffCacheRange{off: r.off, data: r.data})
	}
	return out
}
