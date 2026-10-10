//go:build android

package main

/*
#cgo CFLAGS: -DSDL_MAIN_HANDLED
#cgo LDFLAGS: -lEGL -landroid -llog
#include <EGL/egl.h>
#include <jni.h>
#include <android/log.h>
#include <stdlib.h>
#include <unistd.h>
#include "SDL.h"

static const char* GetStringUTFChars_Wrapper(JNIEnv* env, jstring str) {
    if (!str) return NULL;
    return (*env)->GetStringUTFChars(env, str, NULL);
}

static void ReleaseStringUTFChars_Wrapper(JNIEnv* env, jstring str, const char* chars) {
    if (str && chars) {
        (*env)->ReleaseStringUTFChars(env, str, chars);
    }
}

// OMugen: calls a no-argument method of the app's activity (OMugenActivity).
static void omugen_notify_activity(const char* method) {
    JNIEnv* env = (JNIEnv*)SDL_AndroidGetJNIEnv();
    jobject activity = (jobject)SDL_AndroidGetActivity();
    if (!env || !activity) return;
    jclass cls = (*env)->GetObjectClass(env, activity);
    jmethodID mid = (*env)->GetMethodID(env, cls, method, "()V");
    if (mid) (*env)->CallVoidMethod(env, activity, mid);
    if ((*env)->ExceptionCheck(env)) (*env)->ExceptionClear(env);
    (*env)->DeleteLocalRef(env, cls);
    (*env)->DeleteLocalRef(env, activity);
}

// THIS IS THE KEY: A C constructor.
// This runs when the .so is loaded, BEFORE Go starts itself up.
__attribute__((constructor))
static void prepare_go_runtime() {
    // cgocheck=0: Stop Go from scanning memory pointers (prevents many A14 crashes)
    // scavenge=off: Stop the background memory reclaimer thread
    setenv("GODEBUG", "asyncpreemptoff=1,sigaltstack=0,cgocheck=0,scavenge=off,installgoroot=0,hardstacklimit=0", 1);
}
*/
import "C"
import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/veandco/go-sdl2/sdl"
)

var (
	extractionDone = make(chan bool, 1)
	baseDir        string
	appArgs        []string // the app's options as command-line flags (-framerate 60)
	logcatTag      = C.CString("omugen")
)

// runLogName is this run's log in OMugen's folder of the app's storage
// (LOCALAPPDATA/OMugen): the engine's messages and Go's own crash reports.
// After a crash the app shows it (OMugenActivity).
const runLogName = "last-run.log"

// openRunLog sends standard output and errors to the run log; Android drops
// them otherwise.
func openRunLog() {
	dir := engineCacheRoot()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		Logcat("run log: " + err.Error())
		return
	}
	f, err := os.Create(filepath.Join(dir, runLogName))
	if err != nil {
		Logcat("run log: " + err.Error())
		return
	}
	syscall.Dup3(int(f.Fd()), 1, 0)
	syscall.Dup3(int(f.Fd()), 2, 0)
	fmt.Fprintf(os.Stderr, "OMugen %s (%s) on Android\nGame: %s\nArgs: %v\nStarted: %s\n\n",
		Version, BuildTime, baseDir, appArgs, time.Now().Format("2006-01-02 15:04:05"))
}

// androidEngineExit tells the app the game was quit from its menu (no crash).
func androidEngineExit() {
	cs := C.CString("onEngineExit")
	C.omugen_notify_activity(cs)
	C.free(unsafe.Pointer(cs))
}

// androidArgs are the command-line flags the app passed with the game folder.
func androidArgs() []string { return appArgs }

// androidEngineReady tells the app the engine reads controllers now, so it
// can add its on-screen controller.
func androidEngineReady() {
	cs := C.CString("onEngineReady")
	C.omugen_notify_activity(cs)
	C.free(unsafe.Pointer(cs))
}

func init() {
}

// Log writer implementation
// On Android, writing to Stderr usually redirects to Logcat
func NewLogWriter() io.Writer {
	return os.Stderr
}

// TTF font loading
func LoadFntTtf(f *Fnt, fontfile string, filename string, height int32) {
	// 1. The game's folders, then OMugen's own font (phones have no Windows
	// fonts such as Arial, which games often name without shipping it).
	fileDir := omugenTtfPath(fontfile, filename)

	// 2. Set dimensions
	if height == -1 {
		height = int32(f.Size[1])
	} else {
		f.Size[1] = uint16(height)
	}

	// 3. Load the TTF
	ttf, err := gfxFont.LoadFont(fileDir, height, int(sys.gameWidth), int(sys.gameHeight))
	if err != nil {
		// Instead of a pure panic, log exactly where it looked
		Logcat(fmt.Sprintf("ERROR: Failed to load TTF from %s", fileDir))
		panic(fmt.Errorf("failed to load ttf font %v: %w", fileDir, err))
	}

	f.ttf = ttf.(Font)

	// 4. Create dummy palettes
	f.palettes = make([][256]uint32, 1)
	for i := 0; i < 256; i++ {
		f.palettes[0][i] = 0
	}
}

// Message boxes: SDL shows them as Android dialogs (and waits for them), even
// before its video starts.
func ShowInfoDialog(message, title string) {
	Logcat(fmt.Sprintf("INFO [%s]: %s", title, message))
	sdl.ShowSimpleMessageBox(sdl.MESSAGEBOX_INFORMATION, title, message, nil)
}

func ShowErrorDialog(message string) {
	Logcat(fmt.Sprintf("CRITICAL ERROR: %s", message))
	sdl.ShowSimpleMessageBox(sdl.MESSAGEBOX_ERROR, "OMugen", message, nil)
}

func ShowYesNoDialog(message, title string) bool {
	Logcat(fmt.Sprintf("QUESTION [%s]: %s", title, message))
	id, err := sdl.ShowMessageBox(&sdl.MessageBoxData{
		Flags:   sdl.MESSAGEBOX_INFORMATION,
		Title:   title,
		Message: message,
		Buttons: []sdl.MessageBoxButtonData{
			{Flags: sdl.MESSAGEBOX_BUTTON_ESCAPEKEY_DEFAULT, ButtonID: 0, Text: "No"},
			{Flags: sdl.MESSAGEBOX_BUTTON_RETURNKEY_DEFAULT, ButtonID: 1, Text: "Yes"},
		},
	})
	return err == nil && id == 1
}

//export SDL_main
func SDL_main(argc C.int, argv **C.char) C.int {
	runtime.LockOSThread()

	// Wait for JNI to tell us the path is ready
	<-extractionDone
	openRunLog()

	Logcat("SDL_main: Path ready, jumping to realMain")

	// Set the baseDir before starting
	sys.baseDir = baseDir

	// Call realMain NOW that we're on the main thread
	realMain()

	return 0
}

func eglGetProcAddress(name string) unsafe.Pointer {
	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))
	return unsafe.Pointer(C.eglGetProcAddress(cname))
}

func selectRenderer(cfgVal string) (Renderer, FontRenderer) {
	return &Renderer_GLES32{}, &FontRenderer_GLES32{}
}

func getAndroidFilesDir() string {
	path := C.SDL_AndroidGetExternalStoragePath()
	return C.GoString(path)
}

// OMugen's app (android/, OMugenActivity) calls this with the game folder the
// player chose, the app's own folders and its options as command-line flags.
//
//export Java_io_github_oaktinooff_omugen_OMugenActivity_nativeOnSDLReady
func Java_io_github_oaktinooff_omugen_OMugenActivity_nativeOnSDLReady(env *C.JNIEnv, clazz C.jclass, jPath, jFilesDir, jCacheDir, jArgs C.jstring) {
	baseDir = jniString(env, jPath)
	appArgs = strings.Fields(jniString(env, jArgs))
	// Go reads the environment once, as the process started (Android's
	// zygote), so the app's Os.setenv never reaches it: set it here.
	// LOCALAPPDATA holds the engine files and your pad layouts.
	if dir := jniString(env, jFilesDir); dir != "" {
		os.Setenv("LOCALAPPDATA", dir)
		os.Setenv("HOME", dir)
	}
	if dir := jniString(env, jCacheDir); dir != "" {
		os.Setenv("TMPDIR", dir)
	}

	// Signal the Go thread to wake up
	select {
	case extractionDone <- true:
		Logcat("JNI: Signal sent to Go")
	default:
		Logcat("JNI: Signal already sent")
	}
}

func jniString(env *C.JNIEnv, s C.jstring) string {
	c := C.GetStringUTFChars_Wrapper(env, s)
	if c == nil {
		return ""
	}
	defer C.ReleaseStringUTFChars_Wrapper(env, s, c)
	return C.GoString(c)
}

// Logcat writes to Android's log and to the run log.
func Logcat(s string) {
	cs := C.CString(s)
	C.__android_log_write(C.ANDROID_LOG_INFO, logcatTag, cs)
	C.free(unsafe.Pointer(cs))
	os.Stderr.WriteString(s + "\n")
}
