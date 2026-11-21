package io.github.oaktinooff.omugen;

import android.app.Activity;
import android.content.Intent;
import android.os.Bundle;

import java.io.File;

/**
 * Restarts OMugen. The engine runs once per process, so after characters
 * are added the app's process ends and a new one starts: this activity runs
 * in a process of its own (AndroidManifest.xml), ends the app's, opens it
 * again (it goes back into the game), then ends itself.
 */
public class RestartActivity extends Activity {
    static final String PID = "pid";

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        int pid = getIntent().getIntExtra(PID, -1);
        if (pid > 0) {
            android.os.Process.killProcess(pid);
            // Until it is gone: it still has the game's files open.
            for (int i = 0; i < 60 && new File("/proc/" + pid).exists(); i++) {
                try {
                    Thread.sleep(50);
                } catch (InterruptedException e) {
                    break;
                }
            }
        }
        Intent i = new Intent(this, OMugenActivity.class);
        i.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK | Intent.FLAG_ACTIVITY_CLEAR_TASK);
        startActivity(i);
        finish();
        Runtime.getRuntime().exit(0);
    }
}
