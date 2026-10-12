package io.github.oaktinooff.omugen;

import android.Manifest;
import android.app.AlertDialog;
import android.content.BroadcastReceiver;
import android.content.Context;
import android.content.Intent;
import android.content.IntentFilter;
import android.content.SharedPreferences;
import android.content.pm.PackageManager;
import android.graphics.Typeface;
import android.graphics.drawable.GradientDrawable;
import android.net.Uri;
import android.os.Build;
import android.os.Bundle;
import android.os.Environment;
import android.provider.Settings;
import android.util.TypedValue;
import android.view.Display;
import android.view.Gravity;
import android.view.KeyEvent;
import android.view.View;
import android.view.ViewGroup;
import android.view.WindowManager;
import android.widget.Button;
import android.widget.FrameLayout;
import android.widget.LinearLayout;
import android.widget.ScrollView;
import android.widget.TextView;
import android.widget.Toast;

import org.libsdl.app.SDLActivity;

import java.io.BufferedReader;
import java.io.File;
import java.io.FileInputStream;
import java.io.FileOutputStream;
import java.io.InputStreamReader;
import java.io.OutputStreamWriter;
import java.io.Writer;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.Date;
import java.util.HashSet;
import java.util.List;
import java.util.Locale;
import java.util.Set;

/**
 * OMugen on Android: a game picker over SDL's activity. The engine
 * (libmain.so) waits until a game is chosen, then runs it with the chosen
 * options; the on-screen controller (TouchPadView) is added once the engine
 * reads controllers.
 *
 * Games are MUGEN folders (data, chars, stages): in Internal storage/OMugen,
 * on USB drives and SD cards, or in folders the player added (GameLibrary).
 */
public class OMugenActivity extends SDLActivity {
    private static final String PREFS = "OMugen";
    // The game being played, kept apart so phone backups leave it out (res/xml).
    private static final String RUN_PREFS = "OMugenRun";
    private static final int[] FRAME_RATES = { 60, 90, 120, 144 };

    /**
     * Starts the engine on a game folder, with the app's folders (its files:
     * the engine's, your pad layouts) and command-line flags (util_android.go).
     */
    private static native void nativeOnSDLReady(String gameDir, String filesDir, String cacheDir, String args);

    private SharedPreferences prefs, runPrefs;
    private GameLibrary library;
    private View picker;
    private LinearLayout gameList; // filled when the scan finishes
    private int scans;             // the latest scan; older ones are dropped
    private TouchPadView touchPad;
    private boolean started;

    // A drive plugged in or out: look for games again.
    private final BroadcastReceiver drives = new BroadcastReceiver() {
        @Override
        public void onReceive(Context context, Intent intent) {
            if (!started && picker != null) {
                showPicker();
            }
        }
    };
    private boolean drivesWatched;

    @Override
    protected String[] getLibraries() {
        return new String[] { "SDL2", "main" };
    }

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        prefs = getSharedPreferences(PREFS, MODE_PRIVATE);
        runPrefs = getSharedPreferences(RUN_PREFS, MODE_PRIVATE);
        library = new GameLibrary(this);
        prefs.edit().remove("running").remove("runningPid").remove("runningSince").apply(); // v0.1.3 kept them here
        showPicker();
        checkLastRun();
    }

    @Override
    public boolean dispatchKeyEvent(KeyEvent event) {
        // Back on the game list closes the app (SDL's view would swallow it).
        if (!started && event.getKeyCode() == KeyEvent.KEYCODE_BACK) {
            if (event.getAction() == KeyEvent.ACTION_UP) {
                finish();
            }
            return true;
        }
        return super.dispatchKeyEvent(event);
    }

    @Override
    protected void onResume() {
        super.onResume();
        if (!started && picker != null) {
            showPicker(); // back from the file access settings
        }
        if (!started && !drivesWatched) {
            IntentFilter f = new IntentFilter();
            f.addAction(Intent.ACTION_MEDIA_MOUNTED);
            f.addAction(Intent.ACTION_MEDIA_UNMOUNTED);
            f.addAction(Intent.ACTION_MEDIA_REMOVED);
            f.addAction(Intent.ACTION_MEDIA_EJECT);
            f.addDataScheme("file");
            try {
                if (Build.VERSION.SDK_INT >= 33) {
                    registerReceiver(drives, f, Context.RECEIVER_NOT_EXPORTED);
                } else {
                    registerReceiver(drives, f);
                }
                drivesWatched = true;
            } catch (Exception ignored) {
            }
        }
    }

    @Override
    protected void onPause() {
        if (drivesWatched) {
            try {
                unregisterReceiver(drives);
            } catch (Exception ignored) {
            }
            drivesWatched = false;
        }
        super.onPause();
    }

    @Override
    public void onRequestPermissionsResult(int requestCode, String[] permissions, int[] grantResults) {
        super.onRequestPermissionsResult(requestCode, permissions, grantResults);
        if (started) {
            return;
        }
        showPicker();
        // After "Don't ask again" (Android 8-10) only the app's settings can grant it.
        if (requestCode == 1 && !hasFileAccess()
                && !shouldShowRequestPermissionRationale(Manifest.permission.READ_EXTERNAL_STORAGE)) {
            Toast.makeText(this, "Allow Storage in OMugen's permissions, then come back.", Toast.LENGTH_LONG).show();
            try {
                startActivity(new Intent(Settings.ACTION_APPLICATION_DETAILS_SETTINGS,
                        Uri.parse("package:" + getPackageName())));
            } catch (Exception ignored) {
            }
        }
    }

    @Override
    protected void onDestroy() {
        if (touchPad != null) {
            touchPad.detach();
        }
        super.onDestroy();
        // The engine runs once per process: start fresh next time.
        android.os.Process.killProcess(android.os.Process.myPid());
    }

    /** Called by the engine when the game is quit from its menu. */
    public void onEngineExit() {
        runPrefs.edit().clear().commit();
    }

    /** Called by the engine once it reads controllers. */
    public void onEngineReady() {
        runOnUiThread(new Runnable() {
            @Override
            public void run() {
                if (touchPad != null) {
                    touchPad.attach();
                }
            }
        });
    }

    // ------------------------------------------------------------- picker

    private void showPicker() {
        if (picker != null && picker.getParent() != null) {
            ((ViewGroup) picker.getParent()).removeView(picker);
        }
        LinearLayout col = new LinearLayout(this);
        col.setOrientation(LinearLayout.VERTICAL);
        int pad = dp(28);
        col.setPadding(pad, dp(20), pad, dp(20));

        TextView title = text("OMugen", 32, true);
        col.addView(title);

        if (!hasFileAccess()) {
            col.addView(text("OMugen plays MUGEN games from your phone's storage, so it needs access to your files.", 16, false));
            col.addView(button("Allow file access", new View.OnClickListener() {
                @Override
                public void onClick(View v) {
                    requestFileAccess();
                }
            }));
        } else {
            col.addView(text("Your games", 20, true));
            gameList = new LinearLayout(this);
            gameList.setOrientation(LinearLayout.VERTICAL);
            gameList.addView(text("Looking for games...", 15, false));
            col.addView(gameList);
            scanGames();
            col.addView(button("Refresh", new View.OnClickListener() {
                @Override
                public void onClick(View v) {
                    showPicker();
                }
            }));
            col.addView(button("Add a game folder (USB drive, SD card...)", new View.OnClickListener() {
                @Override
                public void onClick(View v) {
                    browse(null);
                }
            }));
            for (final String folder : addedFolders()) {
                col.addView(button("✕  " + folder, new View.OnClickListener() {
                    @Override
                    public void onClick(View v) {
                        confirmRemoveFolder(folder);
                    }
                }));
            }
            String where = "Games go in the OMugen folder of your phone's storage or of a USB drive or SD card, "
                    + "at the top of a drive, or in a folder you add. They're played where they are.";
            if (!GameLibrary.drivesSupported()) {
                where += " Games on USB drives and SD cards need Android 11 or newer.";
            }
            col.addView(text(where, 13, false));
        }

        col.addView(text("Settings", 20, true));
        final Button controls = button(controlsLabel(), null);
        controls.setOnClickListener(new View.OnClickListener() {
            @Override
            public void onClick(View v) {
                prefs.edit().putBoolean("touch", !onScreenController()).apply();
                controls.setText(controlsLabel());
            }
        });
        col.addView(controls);
        final Button fps = button(frameRateLabel(), null);
        fps.setOnClickListener(new View.OnClickListener() {
            @Override
            public void onClick(View v) {
                List<Integer> rates = frameRates();
                int i = rates.indexOf(frameRate());
                prefs.edit().putInt("fps", rates.get((i + 1) % rates.size())).apply();
                fps.setText(frameRateLabel());
            }
        });
        col.addView(fps);
        col.addView(text("The game itself always runs at its own speed; a higher frame rate only looks smoother "
                + "and uses more battery.", 13, false));
        col.addView(button("Send the last game's log", new View.OnClickListener() {
            @Override
            public void onClick(View v) {
                shareLastLog();
            }
        }));
        col.addView(text("OMugen " + appVersion(), 12, false));

        ScrollView scroll = new ScrollView(this);
        scroll.setBackgroundColor(0xF2101016);
        scroll.addView(col);
        picker = scroll;
        addContentView(picker, new ViewGroup.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT,
                ViewGroup.LayoutParams.MATCH_PARENT));
    }

    private void startGame(File game) {
        started = true;
        if (drivesWatched) {
            try {
                unregisterReceiver(drives);
            } catch (Exception ignored) {
            }
            drivesWatched = false;
        }
        // "running" stays until the game is quit from its menu: if it is still
        // there next time, OMugen looks for what went wrong (checkLastRun).
        prefs.edit().putString("last", game.getAbsolutePath()).apply();
        runPrefs.edit().putString("running", game.getAbsolutePath())
                .putInt("runningPid", android.os.Process.myPid())
                .putLong("runningSince", System.currentTimeMillis())
                .commit();
        requestRefreshRate(frameRate());
        if (picker != null && picker.getParent() != null) {
            ((ViewGroup) picker.getParent()).removeView(picker);
        }
        picker = null;
        if (onScreenController()) {
            touchPad = new TouchPadView(this);
            addContentView(touchPad, new FrameLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT,
                    ViewGroup.LayoutParams.MATCH_PARENT));
        }
        nativeOnSDLReady(game.getAbsolutePath(), getFilesDir().getAbsolutePath(),
                getCacheDir().getAbsolutePath(), "-omugenfps " + frameRate());
    }

    /** Asks for the screen mode closest to the frame rate (less battery at 60). */
    private void requestRefreshRate(int fps) {
        try {
            Display display = getWindowManager().getDefaultDisplay();
            Display.Mode current = display.getMode();
            Display.Mode best = null;
            for (Display.Mode m : display.getSupportedModes()) {
                if (m.getPhysicalWidth() != current.getPhysicalWidth()
                        || m.getPhysicalHeight() != current.getPhysicalHeight()) {
                    continue;
                }
                if (best == null || Math.abs(m.getRefreshRate() - fps) < Math.abs(best.getRefreshRate() - fps)) {
                    best = m;
                }
            }
            WindowManager.LayoutParams lp = getWindow().getAttributes();
            if (best != null) {
                lp.preferredDisplayModeId = best.getModeId();
            }
            lp.preferredRefreshRate = fps;
            getWindow().setAttributes(lp);
        } catch (Exception ignored) {
        }
    }

    // ------------------------------------------------------------- game list

    /** Looks for games off the UI thread (drives can be slow), then lists them. */
    private void scanGames() {
        final int scan = ++scans;
        final List<String> added = addedFolders();
        new Thread(new Runnable() {
            @Override
            public void run() {
                final List<GameLibrary.Game> games = library.scan(added);
                runOnUiThread(new Runnable() {
                    @Override
                    public void run() {
                        if (scan == scans && !started) {
                            listGames(games);
                        }
                    }
                });
            }
        }).start();
    }

    private void listGames(List<GameLibrary.Game> games) {
        if (gameList == null) {
            return;
        }
        gameList.removeAllViews();
        if (games.isEmpty()) {
            gameList.addView(text("No games yet. Copy a MUGEN game folder (the one with data, chars and stages inside) "
                    + "into the OMugen folder of your phone's storage, or plug in the drive that has your games, "
                    + "then tap Refresh.", 15, false));
        }
        String last = prefs.getString("last", "");
        for (final GameLibrary.Game game : games) {
            String name = game.dir.getName();
            if (!game.where.equals("Phone")) {
                name += "   ·  " + game.where;
            }
            if (game.dir.getAbsolutePath().equals(last)) {
                name = "▶  " + name;
            }
            gameList.addView(button(name, new View.OnClickListener() {
                @Override
                public void onClick(View v) {
                    startGame(game.dir);
                }
            }));
        }
    }

    private List<String> addedFolders() {
        List<String> l = new ArrayList<>(prefs.getStringSet("folders", new HashSet<String>()));
        l.sort(String.CASE_INSENSITIVE_ORDER);
        return l;
    }

    private void setAddedFolders(Set<String> folders) {
        prefs.edit().putStringSet("folders", folders).apply();
        showPicker();
    }

    /**
     * A folder browser over the phone's storage and the drives (dir null: the
     * list of them). "Use this folder" adds a game, or a folder of games.
     */
    private void browse(final File dir) {
        final List<File> dirs = new ArrayList<>();
        final List<String> names = new ArrayList<>();
        String title;
        if (dir == null) {
            for (GameLibrary.Place p : library.places()) {
                dirs.add(p.dir);
                names.add(p.name);
            }
            title = "Choose a folder";
        } else {
            for (File d : GameLibrary.subFolders(dir)) {
                dirs.add(d);
                names.add(GameLibrary.isGame(d) ? d.getName() + "   (a game)" : d.getName());
            }
            title = dir.getAbsolutePath() + (GameLibrary.isGame(dir) ? "   (a game)" : "");
        }
        AlertDialog.Builder b = new AlertDialog.Builder(this)
                .setTitle(title)
                .setItems(names.toArray(new String[0]), (d, which) -> browse(dirs.get(which)))
                .setNegativeButton("Cancel", null);
        if (dir != null) {
            b.setPositiveButton("Use this folder", (d, w) -> {
                Set<String> folders = new HashSet<>(addedFolders());
                folders.add(dir.getAbsolutePath());
                setAddedFolders(folders);
            });
            b.setNeutralButton("Up", (d, w) -> browse(isPlace(dir) ? null : dir.getParentFile()));
        }
        b.show();
    }

    private boolean isPlace(File dir) {
        for (GameLibrary.Place p : library.places()) {
            if (p.dir.getAbsolutePath().equals(dir.getAbsolutePath())) {
                return true;
            }
        }
        return dir.getParentFile() == null;
    }

    private void confirmRemoveFolder(final String folder) {
        new AlertDialog.Builder(this)
                .setTitle("Remove this folder from the list?")
                .setMessage(folder + "\n\nNothing is deleted: its games just aren't listed anymore.")
                .setPositiveButton("Remove", (d, w) -> {
                    Set<String> folders = new HashSet<>(addedFolders());
                    folders.remove(folder);
                    setAddedFolders(folders);
                })
                .setNegativeButton("Cancel", null)
                .show();
    }

    private boolean hasFileAccess() {
        if (Build.VERSION.SDK_INT >= 30) {
            return Environment.isExternalStorageManager();
        }
        return checkSelfPermission(Manifest.permission.READ_EXTERNAL_STORAGE) == PackageManager.PERMISSION_GRANTED
                && checkSelfPermission(Manifest.permission.WRITE_EXTERNAL_STORAGE) == PackageManager.PERMISSION_GRANTED;
    }

    private void requestFileAccess() {
        if (Build.VERSION.SDK_INT >= 30) {
            try {
                startActivity(new Intent(Settings.ACTION_MANAGE_APP_ALL_FILES_ACCESS_PERMISSION,
                        Uri.parse("package:" + getPackageName())));
            } catch (Exception e) {
                startActivity(new Intent(Settings.ACTION_MANAGE_ALL_FILES_ACCESS_PERMISSION));
            }
        } else {
            requestPermissions(new String[] { Manifest.permission.READ_EXTERNAL_STORAGE,
                    Manifest.permission.WRITE_EXTERNAL_STORAGE }, 1);
        }
    }

    // ------------------------------------------------------------- error report

    /**
     * When the last game didn't end from its menu, looks in the engine's log
     * (last-run.log, util_android.go) and Android's log for an error, and
     * shows it, saved as OMugen-error-report.txt in the OMugen folder.
     */
    private void checkLastRun() {
        final String game = runPrefs.getString("running", null);
        if (game == null) {
            return;
        }
        final int pid = runPrefs.getInt("runningPid", -1);
        final long since = runPrefs.getLong("runningSince", 0);
        runPrefs.edit().clear().commit();
        new Thread(new Runnable() {
            @Override
            public void run() {
                List<String> engineLog = readLines(new File(new File(getFilesDir(), "OMugen"), "last-run.log"), 400);
                List<String> systemLog = systemLog(pid, since);
                List<String> found = new ArrayList<>();
                for (String l : engineLog) {
                    if (isEngineError(l)) {
                        found.add(l);
                    }
                }
                for (String l : systemLog) {
                    if (isSystemError(l)) {
                        found.add(l);
                    }
                }
                if (found.isEmpty()) {
                    return; // closed some other way (swiped away, say)
                }
                StringBuilder r = new StringBuilder();
                r.append("OMugen error report\n").append(aboutApp());
                r.append("Game: ").append(game).append('\n');
                r.append("Started: ").append(new Date(since)).append("\n\n");
                r.append("== What went wrong ==\n");
                for (int i = 0; i < Math.min(found.size(), 40); i++) {
                    r.append(found.get(i)).append('\n');
                }
                r.append("\n== OMugen log (end) ==\n");
                for (String l : engineLog) {
                    r.append(l).append('\n');
                }
                r.append("\n== Android log ==\n");
                for (String l : systemLog) {
                    r.append(l).append('\n');
                }
                final String report = r.toString();
                final File saved = saveReport(report);
                final StringBuilder summary = new StringBuilder();
                for (int i = 0; i < Math.min(found.size(), 12); i++) {
                    summary.append(found.get(i)).append("\n\n");
                }
                runOnUiThread(new Runnable() {
                    @Override
                    public void run() {
                        showReport(summary.toString().trim(), report, saved);
                    }
                });
            }
        }).start();
    }

    // How the engine's log starts the lines that report an error: its own
    // messages (util_android.go, main.go handlePanic) and Go's crash reports.
    private static final String[] ENGINE_ERRORS = { "CRITICAL ERROR", "Panic:", "panic:", "fatal error:",
            "SIGSEGV", "SIGABRT", "SIGBUS", "SIGILL", "unexpected signal" };

    private static boolean isEngineError(String l) {
        String s = l.trim();
        for (String m : ENGINE_ERRORS) {
            if (s.startsWith(m)) {
                return true;
            }
        }
        return false;
    }

    /**
     * A line of Android's log (threadtime: date time pid tid level tag: message)
     * that reports an error: fatal ones, Java crashes, the Go runtime's and
     * native crash reports. Only the level and tag count, so a game named
     * "Fatal Fury" doesn't look like an error.
     */
    private static boolean isSystemError(String l) {
        String[] f = l.trim().split("\\s+", 6);
        if (f.length < 6) {
            return false;
        }
        int colon = f[5].indexOf(':');
        String tag = (colon >= 0 ? f[5].substring(0, colon) : f[5]).trim();
        switch (f[4]) {
            case "F":
                return true;
            case "E":
                return tag.equals("AndroidRuntime") || tag.equals("Go") || tag.equals("DEBUG") || tag.equals("libc");
            default:
                return false;
        }
    }

    /** The last lines of a text file (none if it is missing). */
    private static List<String> readLines(File f, int max) {
        List<String> lines = new ArrayList<>();
        try (BufferedReader r = new BufferedReader(new InputStreamReader(new FileInputStream(f), StandardCharsets.UTF_8))) {
            String l;
            while ((l = r.readLine()) != null) {
                lines.add(l);
                if (lines.size() > max) {
                    lines.remove(0);
                }
            }
        } catch (Exception ignored) {
        }
        return lines;
    }

    /**
     * This app's lines of Android's log since the last game started (apps only
     * see their own): the game's process, and native crash reports, which come
     * from Android's crash_dump (tag DEBUG).
     */
    private static List<String> systemLog(int pid, long since) {
        List<String> lines = new ArrayList<>();
        // logcat -t takes a time as epoch seconds (Android 7+), or a line count.
        String from = since > 0 ? String.format(Locale.ROOT, "%d.%03d", since / 1000, since % 1000) : "5000";
        try {
            Process proc = new ProcessBuilder("logcat", "-d", "-v", "threadtime", "-t", from)
                    .redirectErrorStream(true).start();
            String pidField = String.valueOf(pid);
            try (BufferedReader r = new BufferedReader(new InputStreamReader(proc.getInputStream(), StandardCharsets.UTF_8))) {
                String l;
                while ((l = r.readLine()) != null) {
                    String[] f = l.trim().split("\\s+", 7);
                    boolean ours = f.length >= 6 && pid > 0 && f[2].equals(pidField);
                    boolean crashReport = f.length >= 6 && (f[5].startsWith("DEBUG") || f[5].startsWith("crash_dump"));
                    if (ours || crashReport) {
                        lines.add(l);
                        if (lines.size() > 1500) {
                            lines.remove(0);
                        }
                    }
                }
            }
            proc.waitFor();
        } catch (Exception e) {
            lines.add("(no Android log: " + e + ")");
        }
        return lines;
    }

    private File saveReport(String report) {
        File f = new File(GameLibrary.phoneFolder(), "OMugen-error-report.txt");
        try (Writer w = new OutputStreamWriter(new FileOutputStream(f), StandardCharsets.UTF_8)) {
            w.write(report);
            return f;
        } catch (Exception e) {
            return null;
        }
    }

    private void showReport(String summary, final String report, File saved) {
        if (isFinishing()) {
            return;
        }
        TextView t = new TextView(this);
        t.setText(summary + (saved != null
                ? "\n\nThe full report is in your phone's storage: OMugen/" + saved.getName() : ""));
        t.setTextIsSelectable(true);
        t.setTypeface(Typeface.MONOSPACE);
        t.setTextSize(TypedValue.COMPLEX_UNIT_SP, 12);
        t.setPadding(dp(20), dp(12), dp(20), dp(12));
        ScrollView scroll = new ScrollView(this);
        scroll.addView(t);
        new AlertDialog.Builder(this)
                .setTitle("OMugen closed because of an error")
                .setView(scroll)
                .setPositiveButton("Share report", (d, w) -> shareText("OMugen error report", report))
                .setNegativeButton("Close", null)
                .show();
    }

    /** Shares the engine's log of the last game (last-run.log), for a problem that didn't close it. */
    private void shareLastLog() {
        List<String> lines = readLines(new File(new File(getFilesDir(), "OMugen"), "last-run.log"), 4000);
        if (lines.isEmpty()) {
            Toast.makeText(this, "No log yet: it is written while a game runs.", Toast.LENGTH_LONG).show();
            return;
        }
        StringBuilder b = new StringBuilder("OMugen log\n").append(aboutApp()).append('\n');
        for (String l : lines) {
            b.append(l).append('\n');
        }
        shareText("OMugen log", b.toString());
    }

    /** Sends text with an app of your choice (messages, mail, Drive...). */
    private void shareText(String subject, String text) {
        // Binder transactions are limited: the start and end are what matter.
        if (text.length() > 90000) {
            text = text.substring(0, 45000) + "\n...\n" + text.substring(text.length() - 45000);
        }
        Intent send = new Intent(Intent.ACTION_SEND);
        send.setType("text/plain");
        send.putExtra(Intent.EXTRA_SUBJECT, subject);
        send.putExtra(Intent.EXTRA_TEXT, text);
        try {
            startActivity(Intent.createChooser(send, "Share the " + subject));
        } catch (Exception ignored) {
        }
    }

    /** The app's version and the phone, for reports. */
    private String aboutApp() {
        return "App: " + appVersion() + "\nDevice: " + Build.MANUFACTURER + " " + Build.MODEL + " (" + Build.DEVICE
                + "), Android " + Build.VERSION.RELEASE + " (API " + Build.VERSION.SDK_INT + ")\n";
    }

    @SuppressWarnings("deprecation")
    private String appVersion() {
        try {
            android.content.pm.PackageInfo info = getPackageManager().getPackageInfo(getPackageName(), 0);
            return info.versionName + " (" + info.versionCode + ")";
        } catch (Exception e) {
            return "?";
        }
    }

    // ------------------------------------------------------------- settings

    private boolean onScreenController() {
        return prefs.getBoolean("touch", true);
    }

    private String controlsLabel() {
        return onScreenController() ? "Controls: on-screen controller" : "Controls: Bluetooth controllers only";
    }

    /** The frame rates the screen has a mode for at its current resolution (60 always). */
    private List<Integer> frameRates() {
        List<Integer> rates = new ArrayList<>();
        for (int r : FRAME_RATES) {
            if (r == 60 || hasRefreshRate(r)) {
                rates.add(r);
            }
        }
        return rates;
    }

    private boolean hasRefreshRate(int rate) {
        try {
            Display display = getWindowManager().getDefaultDisplay();
            Display.Mode current = display.getMode();
            for (Display.Mode m : display.getSupportedModes()) {
                if (m.getPhysicalWidth() == current.getPhysicalWidth()
                        && m.getPhysicalHeight() == current.getPhysicalHeight()
                        && Math.abs(m.getRefreshRate() - rate) < 1.5f) {
                    return true;
                }
            }
        } catch (Exception ignored) {
        }
        return false;
    }

    private int frameRate() {
        int fps = prefs.getInt("fps", 60);
        List<Integer> rates = frameRates();
        return rates.contains(fps) ? fps : 60;
    }

    private String frameRateLabel() {
        int fps = frameRate();
        return "Frame rate: " + fps + " fps" + (fps == 60 ? " (saves battery)" : "");
    }

    // ------------------------------------------------------------- views

    private TextView text(String s, float sp, boolean bold) {
        TextView t = new TextView(this);
        t.setText(s);
        t.setTextColor(bold ? 0xFFFFFFFF : 0xFFB8BCCB);
        t.setTextSize(TypedValue.COMPLEX_UNIT_SP, sp);
        if (bold) {
            t.setTypeface(Typeface.DEFAULT_BOLD);
        }
        t.setPadding(0, dp(bold ? 14 : 6), 0, dp(6));
        return t;
    }

    private Button button(String s, View.OnClickListener onClick) {
        Button b = new Button(this);
        b.setText(s);
        b.setAllCaps(false);
        b.setTextColor(0xFFFFFFFF);
        b.setTextSize(TypedValue.COMPLEX_UNIT_SP, 17);
        b.setGravity(Gravity.START | Gravity.CENTER_VERTICAL);
        b.setPadding(dp(18), dp(12), dp(18), dp(12));
        GradientDrawable bg = new GradientDrawable();
        bg.setColor(0xFF262A38);
        bg.setCornerRadius(dp(10));
        b.setBackground(bg);
        if (onClick != null) {
            b.setOnClickListener(onClick);
        }
        LinearLayout.LayoutParams lp = new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT,
                ViewGroup.LayoutParams.WRAP_CONTENT);
        lp.setMargins(0, dp(5), 0, dp(5));
        b.setLayoutParams(lp);
        return b;
    }

    private int dp(float v) {
        return Math.round(TypedValue.applyDimension(TypedValue.COMPLEX_UNIT_DIP, v, getResources().getDisplayMetrics()));
    }
}
