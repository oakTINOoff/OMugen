package io.github.oaktinooff.omugen;

import android.Manifest;
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
import android.system.Os;
import android.util.TypedValue;
import android.view.Display;
import android.view.Gravity;
import android.view.View;
import android.view.ViewGroup;
import android.widget.Button;
import android.widget.FrameLayout;
import android.widget.LinearLayout;
import android.widget.ScrollView;
import android.widget.TextView;

import org.libsdl.app.SDLActivity;

import java.io.File;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.List;

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

    /** Starts the engine on a game folder, with command-line flags (util_android.go). */
    private static native void nativeOnSDLReady(String gameDir, String args);

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
        // OMugen's own files (the engine's, your pad layouts) go to the app's
        // storage. Set before the engine loads, which reads it at start.
        try {
            Os.setenv("LOCALAPPDATA", getFilesDir().getAbsolutePath(), true);
        } catch (Exception ignored) {
        }
        super.onCreate(savedInstanceState);
        prefs = getSharedPreferences(PREFS, MODE_PRIVATE);
        showPicker();
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
        prefs.edit().putString("last", game.getAbsolutePath()).apply();
        if (picker != null && picker.getParent() != null) {
            ((ViewGroup) picker.getParent()).removeView(picker);
        }
        picker = null;
        if (onScreenController()) {
            touchPad = new TouchPadView(this);
            addContentView(touchPad, new FrameLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT,
                    ViewGroup.LayoutParams.MATCH_PARENT));
        }
        nativeOnSDLReady(game.getAbsolutePath(), "-framerate " + frameRate());
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
