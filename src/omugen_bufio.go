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

// bufReadSeeker adds a read buffer to a ReadSeekCloser while keeping Seek.
// SFF preloading walks sprite headers with many small Seek+Read calls; on an
// unbuffered *os.File each one is a system call (costly on compressed files).
// Reads inside the buffered window are served from memory; large reads
// bypass the buffer.
type bufReadSeeker struct {
	rs       io.ReadSeekCloser
	buf      []byte
	bufStart int64 // file offset of buf[0]
	n        int   // valid bytes in buf
	pos      int64 // logical read position
}

func newBufReadSeeker(rs io.ReadSeekCloser, size int) *bufReadSeeker {
	return &bufReadSeeker{rs: rs, buf: make([]byte, size), bufStart: -1}
}

func (b *bufReadSeeker) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if b.bufStart >= 0 && b.pos >= b.bufStart && b.pos < b.bufStart+int64(b.n) {
		c := copy(p, b.buf[b.pos-b.bufStart:b.n])
		b.pos += int64(c)
		return c, nil
	}
	if _, err := b.rs.Seek(b.pos, io.SeekStart); err != nil {
		return 0, err
	}
	if len(p) >= len(b.buf) {
		c, err := b.rs.Read(p)
		b.pos += int64(c)
		return c, err
	}
	n, err := io.ReadFull(b.rs, b.buf)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		b.bufStart = -1
		return 0, err
	}
	b.bufStart, b.n = b.pos, n
	if n == 0 {
		return 0, io.EOF
	}
	c := copy(p, b.buf[:n])
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
