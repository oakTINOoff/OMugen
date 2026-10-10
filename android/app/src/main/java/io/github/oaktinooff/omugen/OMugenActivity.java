package io.github.oaktinooff.omugen;

import android.Manifest;
import android.app.AlertDialog;
import android.content.Intent;
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
import android.view.View;
import android.view.ViewGroup;
import android.view.WindowManager;
import android.widget.Button;
import android.widget.FrameLayout;
import android.widget.LinearLayout;
import android.widget.ScrollView;
import android.widget.TextView;

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
import java.util.Arrays;
import java.util.Date;
import java.util.List;
import java.util.Locale;

/**
 * OMugen on Android: a game picker over SDL's activity. The engine
 * (libmain.so) waits until a game is chosen, then runs it with the chosen
 * options; the on-screen controller (TouchPadView) is added once the engine
 * reads controllers.
 *
 * Games are MUGEN folders (data, chars, stages) in Internal storage/OMugen.
 */
public class OMugenActivity extends SDLActivity {
    private static final String PREFS = "OMugen";
    private static final int[] FRAME_RATES = { 60, 90, 120, 144 };

    /**
     * Starts the engine on a game folder, with the app's folders (its files:
     * the engine's, your pad layouts) and command-line flags (util_android.go).
     */
    private static native void nativeOnSDLReady(String gameDir, String filesDir, String cacheDir, String args);

    private SharedPreferences prefs;
    private View picker;
    private TouchPadView touchPad;
    private boolean started;

    @Override
    protected String[] getLibraries() {
        return new String[] { "SDL2", "main" };
    }

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        prefs = getSharedPreferences(PREFS, MODE_PRIVATE);
        showPicker();
        checkLastRun();
    }

    @Override
    protected void onResume() {
        super.onResume();
        if (!started && picker != null) {
            showPicker(); // back from the file access settings
        }
    }

    @Override
    public void onRequestPermissionsResult(int requestCode, String[] permissions, int[] grantResults) {
        super.onRequestPermissionsResult(requestCode, permissions, grantResults);
        if (!started) {
            showPicker();
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
        prefs.edit().remove("running").commit();
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
            File root = gamesFolder();
            if (!root.isDirectory()) {
                root.mkdirs();
            }
            List<File> games = findGames(root);
            col.addView(text("Your games", 20, true));
            if (games.isEmpty()) {
                col.addView(text("No games yet. Copy a MUGEN game folder (the one with data, chars and stages inside) "
                        + "into the OMugen folder of your phone's internal storage, then tap Refresh.", 15, false));
            }
            String last = prefs.getString("last", "");
            for (final File game : games) {
                String name = game.getName();
                if (game.getAbsolutePath().equals(last)) {
                    name = "▶  " + name;
                }
                col.addView(button(name, new View.OnClickListener() {
                    @Override
                    public void onClick(View v) {
                        startGame(game);
                    }
                }));
            }
            col.addView(button("Refresh", new View.OnClickListener() {
                @Override
                public void onClick(View v) {
                    showPicker();
                }
            }));
            col.addView(text("Folder: Internal storage/OMugen", 13, false));
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

        ScrollView scroll = new ScrollView(this);
        scroll.setBackgroundColor(0xF2101016);
        scroll.addView(col);
        picker = scroll;
        addContentView(picker, new ViewGroup.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT,
                ViewGroup.LayoutParams.MATCH_PARENT));
    }

    private void startGame(File game) {
        started = true;
        // "running" stays until the game is quit from its menu: if it is still
        // there next time, OMugen looks for what went wrong (checkLastRun).
        prefs.edit().putString("last", game.getAbsolutePath())
                .putString("running", game.getAbsolutePath())
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
                getCacheDir().getAbsolutePath(), "-framerate " + frameRate());
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

    /** Folders of the OMugen folder that hold a MUGEN game (a data folder). */
    private static List<File> findGames(File root) {
        List<File> games = new ArrayList<>();
        if (isGame(root)) {
            games.add(root);
        }
        File[] dirs = root.listFiles();
        if (dirs != null) {
            Arrays.sort(dirs);
            for (File d : dirs) {
                if (d.isDirectory() && isGame(d)) {
                    games.add(d);
                }
            }
        }
        return games;
    }

    private static boolean isGame(File dir) {
        String[] names = dir.list();
        if (names == null) {
            return false;
        }
        for (String n : names) {
            if (n.equalsIgnoreCase("data") && new File(dir, n).isDirectory()) {
                return true;
            }
        }
        return false;
    }

    @SuppressWarnings("deprecation")
    private static File gamesFolder() {
        return new File(Environment.getExternalStorageDirectory(), "OMugen");
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
        final String game = prefs.getString("running", null);
        if (game == null) {
            return;
        }
        final int pid = prefs.getInt("runningPid", -1);
        final long since = prefs.getLong("runningSince", 0);
        prefs.edit().remove("running").remove("runningPid").remove("runningSince").commit();
        new Thread(new Runnable() {
            @Override
            public void run() {
                List<String> engineLog = readLines(new File(new File(getFilesDir(), "OMugen"), "last-run.log"), 400);
                List<String> systemLog = systemLog(pid);
                List<String> found = new ArrayList<>();
                for (String l : engineLog) {
                    if (isErrorLine(l)) {
                        found.add(l);
                    }
                }
                for (String l : systemLog) {
                    if (isErrorLine(l)) {
                        found.add(l);
                    }
                }
                if (found.isEmpty()) {
                    return; // closed some other way (swiped away, say)
                }
                StringBuilder r = new StringBuilder();
                r.append("OMugen error report\n");
                r.append("App: ").append(appVersion()).append('\n');
                r.append("Device: ").append(Build.MANUFACTURER).append(' ').append(Build.MODEL)
                        .append(" (").append(Build.DEVICE).append("), Android ").append(Build.VERSION.RELEASE)
                        .append(" (API ").append(Build.VERSION.SDK_INT).append(")\n");
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

    private static boolean isErrorLine(String l) {
        String s = l.toLowerCase(Locale.ROOT);
        return s.contains("critical error") || s.contains("panic") || s.contains("fatal")
                || s.contains("sigsegv") || s.contains("sigabrt") || s.contains("application error")
                || s.contains("engine error") || s.contains("unsatisfiedlinkerror");
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

    /** This app's lines of Android's log from the last run (apps only see their own). */
    private static List<String> systemLog(int pid) {
        List<String> lines = new ArrayList<>();
        String p = " " + pid + " ";
        try {
            Process proc = new ProcessBuilder("logcat", "-d", "-v", "threadtime", "-t", "5000")
                    .redirectErrorStream(true).start();
            try (BufferedReader r = new BufferedReader(new InputStreamReader(proc.getInputStream(), StandardCharsets.UTF_8))) {
                String l;
                while ((l = r.readLine()) != null) {
                    // threadtime: date time pid tid level tag: message. Native
                    // crash reports come from Android's crash_dump (tag DEBUG).
                    if ((pid > 0 && l.contains(p)) || l.contains(" DEBUG ") || l.contains("crash_dump")) {
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
        File f = new File(gamesFolder(), "OMugen-error-report.txt");
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
                .setPositiveButton("Share report", (d, w) -> {
                    Intent send = new Intent(Intent.ACTION_SEND);
                    send.setType("text/plain");
                    send.putExtra(Intent.EXTRA_SUBJECT, "OMugen error report");
                    // Binder transactions are limited: the start and end are what matter.
                    String text = report.length() > 90000
                            ? report.substring(0, 45000) + "\n...\n" + report.substring(report.length() - 45000)
                            : report;
                    send.putExtra(Intent.EXTRA_TEXT, text);
                    try {
                        startActivity(Intent.createChooser(send, "Share the OMugen error report"));
                    } catch (Exception ignored) {
                    }
                })
                .setNegativeButton("Close", null)
                .show();
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
        return onScreenController() ? "Controls: on-screen controller" : "Controls: Bluetooth controller only";
    }

    /** The frame rates this screen can show (60 always). */
    private List<Integer> frameRates() {
        float max = 60f;
        try {
            Display display = getWindowManager().getDefaultDisplay();
            for (Display.Mode m : display.getSupportedModes()) {
                max = Math.max(max, m.getRefreshRate());
            }
        } catch (Exception ignored) {
        }
        List<Integer> rates = new ArrayList<>();
        for (int r : FRAME_RATES) {
            if (r == 60 || r <= max + 1) {
                rates.add(r);
            }
        }
        return rates;
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
