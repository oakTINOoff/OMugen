// dllsearch_windows.go
//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

func init() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	exeDir := filepath.Dir(exe)
	libDir := filepath.Join(exeDir, "lib")

	// OMugen: extract the embedded engine pack (runtime DLLs included) first.
	prepareEngineDir()
	var engineLibDir string
	if engineDir != "" {
		engineLibDir = filepath.Join(engineDir, "lib")
	}

	// Safe modern policy.
	_ = windows.SetDefaultDllDirectories(
		windows.LOAD_LIBRARY_SEARCH_DEFAULT_DIRS | windows.LOAD_LIBRARY_SEARCH_USER_DIRS,
	)

	// Our preferred user dirs (priority order: exeDir, libDir, then OMugen's engine lib).
	for _, d := range []string{exeDir, libDir, engineLibDir} {
		if d == "" {
			continue
		}
		if p, err := windows.UTF16PtrFromString(d); err == nil {
			_, _ = windows.AddDllDirectory(p)
		}
	}

	// Legacy fallback: ensure libDir is on PATH (exeDir is searched by default on old loaders).
	pathDirs := libDir
	if engineLibDir != "" {
		pathDirs += ";" + engineLibDir
	}
	_ = os.Setenv("PATH", pathDirs+";"+os.Getenv("PATH"))

	// Families we require.
	wantPatterns := []string{
		"avcodec-*.dll",
		"avdevice-*.dll",
		"avfilter-*.dll",
		"avformat-*.dll",
		"avutil-*.dll",
		"libwinpthread-*.dll",
		"swresample-*.dll",
		"swscale-*.dll",
		"libxmp*.dll",
		"SDL2*.dll",
	}

	// Search order: exe dir -> lib dir -> OMugen engine lib -> Windows default dirs & PATH.
	localOrder := []string{exeDir, libDir, engineLibDir}
	fallbackOrder := windowsDefaultAndPathDirs()

	// Pick a concrete file for each family using the specified order.
	chosen := make(map[string]string) // pattern -> full path
	var missing []string

	for _, pat := range wantPatterns {
		// 1) alongside exe
		if full := firstMatchAcross(localOrder[:1], pat); full != "" {
			chosen[pat] = full
			continue
		}
		// 2) in lib\
		if full := firstMatchAcross(localOrder[1:2], pat); full != "" {
			chosen[pat] = full
			continue
		}
		// 2b) in OMugen's extracted engine files
		if engineLibDir != "" {
			if full := firstMatchAcross(localOrder[2:3], pat); full != "" {
				chosen[pat] = full
				continue
			}
		}
		// 3) Windows default dirs & PATH
		if full := firstMatchAcross(fallbackOrder, pat); full != "" {
			chosen[pat] = full
			continue
		}

		// Not found anywhere.
		missing = append(missing, pat)
	}

	if len(missing) > 0 {
		var where strings.Builder
		where.WriteString("  " + exeDir + "  (exe directory)\n")
		where.WriteString("  " + libDir + "  (lib directory)\n")
		if engineLibDir != "" {
			where.WriteString("  " + engineLibDir + "  (OMugen engine files)\n")
		}
		if engineDirError != nil {
			where.WriteString("\nOMugen could not unpack its engine files:\n  " + engineDirError.Error() + "\n\n")
		}
		for _, d := range fallbackOrder {
			where.WriteString("  " + d + "\n")
		}
		ShowErrorDialog(
			"Required runtime DLLs are missing.\n\n" +
				"Searched locations (in priority order):\n" + where.String() + "\n" +
				"Missing families:\n  " + strings.Join(missing, "\n  "),
		)
		os.Exit(1)
	}

	// Try to load one DLL per family, in the chosen location (local beats global).
	var loadErrs []string
	for _, full := range chosen {
		_, err := windows.LoadLibraryEx(
			full, 0,
			windows.LOAD_LIBRARY_SEARCH_DLL_LOAD_DIR|
				windows.LOAD_LIBRARY_SEARCH_DEFAULT_DIRS|
				windows.LOAD_LIBRARY_SEARCH_USER_DIRS,
		)
		if err != nil {
			loadErrs = append(loadErrs, fmt.Sprintf("%s: %v", filepath.Base(full), err))
		}
	}
	if len(loadErrs) > 0 {
		ShowErrorDialog(
			"Failed to load required runtime libraries.\n\n" +
				"Errors:\n  " + strings.Join(loadErrs, "\n  ") + "\n\n" +
				"Ensure the DLLs match your app architecture and aren’t blocked by AV/SmartScreen.",
		)
		os.Exit(1)
	}
}

// firstMatchAcross returns the first match for a glob pattern across dirs.
func firstMatchAcross(dirs []string, pattern string) string {
	for _, d := range dirs {
		if matches, _ := filepath.Glob(filepath.Join(d, pattern)); len(matches) > 0 {
			return matches[0]
		}
	}
	return ""
}

// windowsDefaultAndPathDirs returns System32, SysWOW64 (if present), and each PATH dir.
func windowsDefaultAndPathDirs() []string {
	var dirs []string

	// SystemRoot-based directories (cover both 32- and 64-bit cases if they exist).
	if root := os.Getenv("SystemRoot"); root != "" {
		sys32 := filepath.Join(root, "System32")
		if fi, err := os.Stat(sys32); err == nil && fi.IsDir() {
			dirs = append(dirs, sys32)
		}
		sysWOW64 := filepath.Join(root, "SysWOW64")
		if fi, err := os.Stat(sysWOW64); err == nil && fi.IsDir() {
			dirs = append(dirs, sysWOW64)
		}
	}

	// Every folder on PATH.
	for _, p := range strings.Split(os.Getenv("PATH"), ";") {
		p = strings.TrimSpace(p)
		if p != "" {
			if fi, err := os.Stat(p); err == nil && fi.IsDir() {
				dirs = append(dirs, p)
			}
		}
	}
	return uniqueStrings(dirs)
}

func uniqueStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s == "" {
			continue
		}
		if _, ok := seen[s]; !ok {
			seen[s] = struct{}{}
			out = append(out, s)
		}
	}
	return out
}
