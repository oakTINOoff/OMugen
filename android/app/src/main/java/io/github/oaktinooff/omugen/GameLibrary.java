package io.github.oaktinooff.omugen;

import android.content.Context;
import android.os.Build;
import android.os.Environment;
import android.os.storage.StorageManager;
import android.os.storage.StorageVolume;

import java.io.File;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Collection;
import java.util.HashSet;
import java.util.List;
import java.util.Locale;
import java.util.Set;

/**
 * Where OMugen finds games: the OMugen folder of the phone's storage, every
 * USB drive and SD card plugged in (games at the top of the drive or in its
 * own OMugen folder), and the folders the player added (a game, or a folder
 * of games, anywhere). Big collections live on a drive, so nothing is copied:
 * the engine plays them where they are.
 *
 * Drives need Android 11: "all files access" covers them there (reading and
 * writing the game's save folder); before, apps can't write to them.
 */
final class GameLibrary {
    /** A place games can be: the phone's storage or a drive. */
    static final class Place {
        final String name;
        final File dir;

        Place(String name, File dir) {
            this.name = name;
            this.dir = dir;
        }
    }

    /** A game folder (one with a data folder) and where it is. */
    static final class Game {
        final File dir;
        final String where;

        Game(File dir, String where) {
            this.dir = dir;
            this.where = where;
        }
    }

    private final Context context;

    GameLibrary(Context context) {
        this.context = context;
    }

    /** Whether games on USB drives and SD cards can be played (Android 11+). */
    static boolean drivesSupported() {
        return Build.VERSION.SDK_INT >= 30;
    }

    /** The phone's OMugen folder. */
    @SuppressWarnings("deprecation")
    static File phoneFolder() {
        return new File(Environment.getExternalStorageDirectory(), "OMugen");
    }

    /** The phone's storage and the drives that are plugged in, for the folder browser. */
    List<Place> places() {
        List<Place> places = new ArrayList<>();
        @SuppressWarnings("deprecation")
        File phone = Environment.getExternalStorageDirectory();
        places.add(new Place("Phone storage", phone));
        places.addAll(drives());
        return places;
    }

    /** USB drives and SD cards that are plugged in and readable. */
    List<Place> drives() {
        List<Place> drives = new ArrayList<>();
        if (!drivesSupported()) {
            return drives;
        }
        try {
            StorageManager sm = (StorageManager) context.getSystemService(Context.STORAGE_SERVICE);
            for (StorageVolume v : sm.getStorageVolumes()) {
                String state = v.getState();
                if (v.isPrimary() || !(Environment.MEDIA_MOUNTED.equals(state)
                        || Environment.MEDIA_MOUNTED_READ_ONLY.equals(state))) {
                    continue;
                }
                File dir = v.getDirectory();
                if (dir != null && dir.isDirectory()) {
                    String name = v.getDescription(context);
                    drives.add(new Place(name != null && !name.isEmpty() ? name : dir.getName(), dir));
                }
            }
        } catch (Exception ignored) {
        }
        return drives;
    }

    /**
     * Every game it can find, phone first, then each drive, then the added
     * folders; each game once. Lists folders on drives, so call it off the UI
     * thread.
     */
    List<Game> scan(Collection<String> added) {
        List<Game> games = new ArrayList<>();
        Set<String> seen = new HashSet<>();
        File phone = phoneFolder();
        if (!phone.isDirectory()) {
            phone.mkdirs();
        }
        addGames(games, seen, phone, "Phone");
        for (Place d : drives()) {
            // A game at the top of the drive, or in a folder there, or in its OMugen folder.
            addGames(games, seen, d.dir, d.name);
            File omugen = child(d.dir, "OMugen");
            if (omugen != null) {
                addGames(games, seen, omugen, d.name);
            }
        }
        for (String path : sorted(added)) {
            File dir = new File(path);
            if (dir.isDirectory()) {
                addGames(games, seen, dir, placeOf(dir));
            }
        }
        return games;
    }

    /** The place a folder is on ("Phone" or the drive's name). */
    String placeOf(File dir) {
        String path = dir.getAbsolutePath();
        for (Place d : drives()) {
            String root = d.dir.getAbsolutePath();
            if (path.equals(root) || path.startsWith(root + "/")) {
                return d.name;
            }
        }
        return "Phone";
    }

    /** dir itself if it is a game, else its sub-folders that are games. */
    private static void addGames(List<Game> games, Set<String> seen, File dir, String where) {
        if (isGame(dir)) {
            add(games, seen, dir, where);
            return;
        }
        File[] subs = dir.listFiles();
        if (subs == null) {
            return;
        }
        Arrays.sort(subs, (a, b) -> a.getName().compareToIgnoreCase(b.getName()));
        for (File s : subs) {
            if (s.isDirectory() && !s.getName().startsWith(".") && isGame(s)) {
                add(games, seen, s, where);
            }
        }
    }

    private static void add(List<Game> games, Set<String> seen, File dir, String where) {
        String key;
        try {
            key = dir.getCanonicalPath();
        } catch (Exception e) {
            key = dir.getAbsolutePath();
        }
        if (seen.add(key.toLowerCase(Locale.ROOT))) {
            games.add(new Game(dir, where));
        }
    }

    /** A MUGEN game folder: it has a data folder (any case). */
    static boolean isGame(File dir) {
        return child(dir, "data") != null;
    }

    /** dir's sub-folder named name in any case, or null. */
    static File child(File dir, String name) {
        String[] names = dir.list();
        if (names == null) {
            return null;
        }
        for (String n : names) {
            if (n.equalsIgnoreCase(name)) {
                File f = new File(dir, n);
                if (f.isDirectory()) {
                    return f;
                }
            }
        }
        return null;
    }

    /** dir's sub-folders, by name, without hidden ones. */
    static List<File> subFolders(File dir) {
        List<File> out = new ArrayList<>();
        File[] subs = dir.listFiles();
        if (subs == null) {
            return out;
        }
        for (File s : subs) {
            if (s.isDirectory() && !s.getName().startsWith(".")) {
                out.add(s);
            }
        }
        out.sort((a, b) -> a.getName().compareToIgnoreCase(b.getName()));
        return out;
    }

    private static List<String> sorted(Collection<String> c) {
        List<String> l = new ArrayList<>(c);
        l.sort(String.CASE_INSENSITIVE_ORDER);
        return l;
    }
}
