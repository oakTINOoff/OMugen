package main

import (
	"bufio"
	"errors"
	"io"
	"strings"
)

// loadFirstIniSection streams a text file and returns the trimmed lines of
// the first section called name (lowercase, as SectionName returns it), from
// its header up to the next line starting with '['. It stops reading there,
// so finding [Size] near the top of a multi-megabyte .cns reads only the top.
// The result parses with ReadIniSection exactly like the whole file would.
func loadFirstIniSection(filename, name string) ([]string, error) {
	rc, err := OpenFile(filename)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	sc := bufio.NewScanner(rc)
	sc.Buffer(make([]byte, 64<<10), 64<<20)
	var out []string
	in, first := false, true
	for sc.Scan() {
		line := sc.Text()
		if first {
			line = strings.TrimPrefix(line, "\uFEFF")
			first = false
		}
		line = strings.TrimSpace(line)
		if in {
			if len(line) > 0 && line[0] == '[' {
				break
			}
			out = append(out, line)
			continue
		}
		if n, _ := SectionName(line); n == name {
			in = true
			out = append(out, line)
		}
	}
	return out, sc.Err()
}

// bufReadSeeker adds a read cache to a ReadSeekCloser while keeping Seek.
// SFF preloading makes many small Seek+Read calls; on an unbuffered *os.File
// each one is a system call (costly on compressed files). The cache keeps a
// few blocks aligned to the block size, so the access patterns of the SFF
// loader stay cheap: jumping between palette headers and palette data, and
// scanning backwards byte by byte. Reads of a block or more bypass it.
type bufReadSeeker struct {
	rs        io.ReadSeekCloser
	blocks    []bufBlock
	blockSize int64
	tick      uint64
	pos       int64 // logical read position
}

type bufBlock struct {
	start int64 // file offset of data[0], -1 if unused
	n     int   // valid bytes in data
	used  uint64
	data  []byte
}

// newBufReadSeeker caches up to size bytes, in blocks of at most 16 KB.
func newBufReadSeeker(rs io.ReadSeekCloser, size int) *bufReadSeeker {
	bs := 16 << 10
	if size/4 < bs {
		bs = size / 4
	}
	if bs < 64 {
		bs = 64
	}
	b := &bufReadSeeker{rs: rs, blockSize: int64(bs)}
	for i := 0; i < size/bs || i < 1; i++ {
		b.blocks = append(b.blocks, bufBlock{start: -1, data: make([]byte, bs)})
	}
	return b
}

func (b *bufReadSeeker) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	start := b.pos - b.pos%b.blockSize
	var blk *bufBlock
	for i := range b.blocks {
		if b.blocks[i].start == start {
			blk = &b.blocks[i]
			break
		}
	}
	if blk == nil {
		if int64(len(p)) >= b.blockSize {
			if _, err := b.rs.Seek(b.pos, io.SeekStart); err != nil {
				return 0, err
			}
			c, err := b.rs.Read(p)
			b.pos += int64(c)
			return c, err
		}
		// Replace the least recently used block.
		blk = &b.blocks[0]
		for i := range b.blocks {
			if b.blocks[i].used < blk.used {
				blk = &b.blocks[i]
			}
		}
		blk.start = -1
		if _, err := b.rs.Seek(start, io.SeekStart); err != nil {
			return 0, err
		}
		n, err := io.ReadFull(b.rs, blk.data)
		if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
			return 0, err
		}
		blk.start, blk.n = start, n
	}
	b.tick++
	blk.used = b.tick
	off := int(b.pos - start)
	if off >= blk.n {
		return 0, io.EOF
	}
	c := copy(p, blk.data[off:blk.n])
	b.pos += int64(c)
	return c, nil
}

func (b *bufReadSeeker) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		b.pos = offset
	case io.SeekCurrent:
		b.pos += offset
	case io.SeekEnd:
		size, err := b.rs.Seek(0, io.SeekEnd)
		if err != nil {
			return 0, err
		}
		b.pos = size + offset
	default:
		return 0, errors.New("bufReadSeeker: invalid whence")
	}
	if b.pos < 0 {
		b.pos = 0
		return 0, errors.New("bufReadSeeker: negative position")
	}
	return b.pos, nil
}

func (b *bufReadSeeker) Close() error { return b.rs.Close() }
