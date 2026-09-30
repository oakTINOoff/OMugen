//go:build !windows

package main

import "golang.org/x/text/encoding"

// systemCodePage: other systems have no ANSI code page; Windows-1252 and
// Shift-JIS are still tried.
func systemCodePage() encoding.Encoding { return nil }
