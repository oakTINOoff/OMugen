package io.github.oaktinooff.omugen;

import android.content.Context;
import android.graphics.Canvas;
import android.graphics.Paint;
import android.graphics.Path;
import android.graphics.RectF;
import android.graphics.Typeface;
import android.util.SparseIntArray;
import android.view.HapticFeedbackConstants;
import android.view.KeyEvent;
import android.view.MotionEvent;
import android.view.View;

import org.libsdl.app.SDLControllerManager;

/**
 * OMugen's on-screen controller: a DualShock 4 drawn over the game. To the
 * engine it is one more PlayStation controller (an SDL joystick with Sony's
 * vendor ID), so it plays with the same layouts as a real one. The D-pad also
 * moves the left stick, for layouts that move with the stick. Fingers can
 * slide from button to button, and several buttons can be held at once.
 */
public class TouchPadView extends View {
    private static final int DEVICE_ID = 0x4F4D0001;
    private static final int VENDOR_SONY = 0x054C;
    private static final int PRODUCT = 0x4F4D; // no real pad's, so SDL builds the mapping below
    // SDL's Android mapping: these SDL_GameControllerButton bits (A B X Y
    // back start L1 R1, D-pad 11-14) and six axes (sticks, then L2 and R2).
    private static final int BUTTON_MASK = (1 << 0) | (1 << 1) | (1 << 2) | (1 << 3) | (1 << 4) | (1 << 6)
            | (1 << 9) | (1 << 10) | (1 << 11) | (1 << 12) | (1 << 13) | (1 << 14);
    private static final int AXES = 6, AXIS_MASK = 0x3F;

    private static final int CROSS = 0, CIRCLE = 1, SQUARE = 2, TRIANGLE = 3, L1 = 4, R1 = 5, L2 = 6, R2 = 7,
            SHARE = 8, OPTIONS = 9, COUNT = 10;
    private static final int[] KEYS = { KeyEvent.KEYCODE_BUTTON_A, KeyEvent.KEYCODE_BUTTON_B,
            KeyEvent.KEYCODE_BUTTON_X, KeyEvent.KEYCODE_BUTTON_Y, KeyEvent.KEYCODE_BUTTON_L1,
            KeyEvent.KEYCODE_BUTTON_R1, 0, 0, KeyEvent.KEYCODE_BUTTON_SELECT, KeyEvent.KEYCODE_BUTTON_START };
    private static final String[] LABELS = { "", "", "", "", "L1", "R1", "L2", "R2", "SHARE", "OPTIONS" };
    private static final int UP = 1, DOWN = 2, LEFT = 4, RIGHT = 8;
    private static final int[] DPAD_KEYS = { KeyEvent.KEYCODE_DPAD_UP, KeyEvent.KEYCODE_DPAD_DOWN,
            KeyEvent.KEYCODE_DPAD_LEFT, KeyEvent.KEYCODE_DPAD_RIGHT };
    private static final int DPAD = -1, NONE = -2;
    // PlayStation symbol colours: cross, circle, square, triangle.
    private static final int[] SYMBOL = { 0xFF7AA8FF, 0xFFFF5D6C, 0xFFF08CD0, 0xFF3FE0B0 };

    private final Paint fill = new Paint(Paint.ANTI_ALIAS_FLAG);
    private final Paint stroke = new Paint(Paint.ANTI_ALIAS_FLAG);
    private final Paint label = new Paint(Paint.ANTI_ALIAS_FLAG);
    private final Path path = new Path();
    private final float[] bx = new float[COUNT], by = new float[COUNT], br = new float[COUNT]; // face buttons
    private final RectF[] rects = new RectF[COUNT]; // shoulders, share, options
    private float dx, dy, dr; // D-pad centre and radius
    private final SparseIntArray targets = new SparseIntArray(); // pointer id -> button, DPAD or NONE
    private final SparseIntArray dirs = new SparseIntArray(); // pointer id -> D-pad directions
    private int buttons, dpad;
    private boolean attached;

    // SDL drops values it gets before the engine opens the controller, and a
    // trigger it never heard of reads half pressed: send the axes now and then.
    private final Runnable resend = new Runnable() {
        @Override
        public void run() {
            if (attached) {
                sendAxes();
                postDelayed(this, 250);
            }
        }
    };

    public TouchPadView(Context context) {
        super(context);
        stroke.setStyle(Paint.Style.STROKE);
        label.setTextAlign(Paint.Align.CENTER);
        label.setTypeface(Typeface.DEFAULT_BOLD);
        setHapticFeedbackEnabled(true);
    }

    /** Adds the controller to SDL, once the engine reads controllers. */
    public void attach() {
        if (attached) {
            return;
        }
        SDLControllerManager.nativeAddJoystick(DEVICE_ID, "OMugen Touch Controller", "OMugen on-screen controller",
                VENDOR_SONY, PRODUCT, false, BUTTON_MASK, AXES, AXIS_MASK, 0, 0);
        attached = true;
        sendAxes();
        postDelayed(resend, 250);
    }

    public void detach() {
        if (!attached) {
            return;
        }
        attached = false;
        removeCallbacks(resend);
        SDLControllerManager.nativeRemoveJoystick(DEVICE_ID);
    }

    private void sendAxes() {
        float x = ((dpad & RIGHT) != 0 ? 1f : 0f) - ((dpad & LEFT) != 0 ? 1f : 0f);
        float y = ((dpad & DOWN) != 0 ? 1f : 0f) - ((dpad & UP) != 0 ? 1f : 0f);
        SDLControllerManager.onNativeJoy(DEVICE_ID, 0, x);
        SDLControllerManager.onNativeJoy(DEVICE_ID, 1, y);
        SDLControllerManager.onNativeJoy(DEVICE_ID, 2, 0f);
        SDLControllerManager.onNativeJoy(DEVICE_ID, 3, 0f);
        SDLControllerManager.onNativeJoy(DEVICE_ID, 4, held(L2) ? 1f : -1f);
        SDLControllerManager.onNativeJoy(DEVICE_ID, 5, held(R2) ? 1f : -1f);
    }

    private boolean held(int b) {
        return (buttons & (1 << b)) != 0;
    }

    // ------------------------------------------------------------- layout

    @Override
    protected void onSizeChanged(int w, int h, int oldw, int oldh) {
        float u = Math.min(h / 9f, w / 16f);
        float margin = 0.7f * u;
        // D-pad bottom left, face buttons bottom right.
        dr = 1.75f * u;
        dx = margin + dr;
        dy = h - margin - dr;
        float fr = 0.62f * u, fd = 1.2f * u;
        float fx = w - margin - fd - fr, fy = dy;
        face(TRIANGLE, fx, fy - fd, fr);
        face(CROSS, fx, fy + fd, fr);
        face(SQUARE, fx - fd, fy, fr);
        face(CIRCLE, fx + fd, fy, fr);
        // Shoulders above them, L2/R2 on top as on the pad.
        float sw = 2.4f * u, sh = 0.8f * u, gap = 0.3f * u;
        float l1Bottom = dy - dr - 0.6f * u;
        rects[L1] = new RectF(margin, l1Bottom - sh, margin + sw, l1Bottom);
        rects[L2] = new RectF(margin, l1Bottom - 2 * sh - gap, margin + sw, l1Bottom - sh - gap);
        rects[R1] = new RectF(w - margin - sw, l1Bottom - sh, w - margin, l1Bottom);
        rects[R2] = new RectF(w - margin - sw, l1Bottom - 2 * sh - gap, w - margin, l1Bottom - sh - gap);
        // Share and Options at the bottom centre.
        float pw = 1.6f * u, ph = 0.55f * u, py = h - 0.45f * u - ph;
        rects[SHARE] = new RectF(w / 2f - 0.4f * u - pw, py, w / 2f - 0.4f * u, py + ph);
        rects[OPTIONS] = new RectF(w / 2f + 0.4f * u, py, w / 2f + 0.4f * u + pw, py + ph);
        stroke.setStrokeWidth(Math.max(2f, 0.06f * u));
    }

    private void face(int b, float x, float y, float r) {
        bx[b] = x;
        by[b] = y;
        br[b] = r;
    }

    // ------------------------------------------------------------- touch

    private int hit(float x, float y) {
        if (sq(x - dx) + sq(y - dy) <= sq(1.3f * dr)) {
            return DPAD;
        }
        int best = NONE;
        float bestDist = Float.MAX_VALUE;
        for (int b = 0; b < COUNT; b++) {
            RectF r = rects[b];
            if (r != null) {
                float m = 0.3f * r.height();
                if (x >= r.left - m && x <= r.right + m && y >= r.top - m && y <= r.bottom + m) {
                    return b;
                }
            } else if (br[b] > 0) {
                float d = sq(x - bx[b]) + sq(y - by[b]);
                if (d <= sq(1.45f * br[b]) && d < bestDist) {
                    best = b;
                    bestDist = d;
                }
            }
        }
        return best;
    }

    /** The D-pad directions at a point: 8 ways, nothing near the centre. */
    private int direction(float x, float y) {
        float ox = x - dx, oy = y - dy;
        if (sq(ox) + sq(oy) < sq(0.22f * dr)) {
            return 0;
        }
        int sector = (int) Math.round(Math.atan2(oy, ox) / (Math.PI / 4)); // 0 right, 2 down, -2 up
        switch ((sector + 8) % 8) {
            case 0: return RIGHT;
            case 1: return RIGHT | DOWN;
            case 2: return DOWN;
            case 3: return DOWN | LEFT;
            case 4: return LEFT;
            case 5: return LEFT | UP;
            case 6: return UP;
            default: return UP | RIGHT;
        }
    }

    @Override
    public boolean onTouchEvent(MotionEvent e) {
        int action = e.getActionMasked();
        int index = e.getActionIndex();
        switch (action) {
            case MotionEvent.ACTION_DOWN:
            case MotionEvent.ACTION_POINTER_DOWN: {
                int id = e.getPointerId(index);
                int t = hit(e.getX(index), e.getY(index));
                targets.put(id, t);
                if (t == DPAD) {
                    dirs.put(id, direction(e.getX(index), e.getY(index)));
                }
                break;
            }
            case MotionEvent.ACTION_MOVE:
                for (int i = 0; i < e.getPointerCount(); i++) {
                    int id = e.getPointerId(i);
                    int t = targets.get(id, NONE);
                    if (t == DPAD) {
                        dirs.put(id, direction(e.getX(i), e.getY(i)));
                    } else {
                        int n = hit(e.getX(i), e.getY(i)); // slide to another button, or off
                        if (n != DPAD) {
                            targets.put(id, n);
                        }
                    }
                }
                break;
            case MotionEvent.ACTION_UP:
            case MotionEvent.ACTION_POINTER_UP: {
                int id = e.getPointerId(index);
                targets.delete(id);
                dirs.delete(id);
                break;
            }
            case MotionEvent.ACTION_CANCEL:
                targets.clear();
                dirs.clear();
                break;
        }
        int nb = 0, nd = 0;
        for (int i = 0; i < targets.size(); i++) {
            int t = targets.valueAt(i);
            if (t >= 0) {
                nb |= 1 << t;
            } else if (t == DPAD) {
                nd |= dirs.get(targets.keyAt(i), 0);
            }
        }
        apply(nb, nd);
        return true;
    }

    private void apply(int nb, int nd) {
        int changed = nb ^ buttons, dchanged = nd ^ dpad;
        if (changed == 0 && dchanged == 0) {
            return;
        }
        buttons = nb;
        dpad = nd;
        if (attached) {
            for (int b = 0; b < COUNT; b++) {
                if ((changed & (1 << b)) == 0 || KEYS[b] == 0) {
                    continue;
                }
                if ((nb & (1 << b)) != 0) {
                    SDLControllerManager.onNativePadDown(DEVICE_ID, KEYS[b]);
                } else {
                    SDLControllerManager.onNativePadUp(DEVICE_ID, KEYS[b]);
                }
            }
            for (int i = 0; i < 4; i++) {
                if ((dchanged & (1 << i)) == 0) {
                    continue;
                }
                if ((nd & (1 << i)) != 0) {
                    SDLControllerManager.onNativePadDown(DEVICE_ID, DPAD_KEYS[i]);
                } else {
                    SDLControllerManager.onNativePadUp(DEVICE_ID, DPAD_KEYS[i]);
                }
            }
            if (dchanged != 0 || (changed & ((1 << L2) | (1 << R2))) != 0) {
                sendAxes();
            }
        }
        if ((changed & nb) != 0 || (dchanged & nd) != 0) {
            performHapticFeedback(HapticFeedbackConstants.VIRTUAL_KEY);
        }
        invalidate();
    }

    // ------------------------------------------------------------- drawing

    @Override
    protected void onDraw(Canvas c) {
        // D-pad: a cross, the held arms lit.
        float arm = 0.36f * dr;
        drawPart(c, new RectF(dx - arm, dy - dr, dx + arm, dy + dr), false, 0.25f * arm);
        drawPart(c, new RectF(dx - dr, dy - arm, dx + dr, dy + arm), false, 0.25f * arm);
        if ((dpad & UP) != 0) drawPart(c, new RectF(dx - arm, dy - dr, dx + arm, dy - arm), true, 0.25f * arm);
        if ((dpad & DOWN) != 0) drawPart(c, new RectF(dx - arm, dy + arm, dx + arm, dy + dr), true, 0.25f * arm);
        if ((dpad & LEFT) != 0) drawPart(c, new RectF(dx - dr, dy - arm, dx - arm, dy + arm), true, 0.25f * arm);
        if ((dpad & RIGHT) != 0) drawPart(c, new RectF(dx + arm, dy - arm, dx + dr, dy + arm), true, 0.25f * arm);

        for (int b = CROSS; b <= TRIANGLE; b++) {
            drawFace(c, b);
        }
        for (int b = L1; b < COUNT; b++) {
            RectF r = rects[b];
            if (r == null) {
                continue;
            }
            drawPart(c, r, held(b), 0.45f * r.height());
            label.setColor(held(b) ? 0xFFFFFFFF : 0xC8FFFFFF);
            label.setTextSize(r.height() * (b >= SHARE ? 0.42f : 0.5f));
            c.drawText(LABELS[b], r.centerX(), r.centerY() - (label.descent() + label.ascent()) / 2, label);
        }
    }

    private void drawPart(Canvas c, RectF r, boolean lit, float radius) {
        fill.setColor(lit ? 0x8CFFFFFF : 0x30FFFFFF);
        c.drawRoundRect(r, radius, radius, fill);
        stroke.setColor(lit ? 0xE6FFFFFF : 0x80FFFFFF);
        c.drawRoundRect(r, radius, radius, stroke);
    }

    private void drawFace(Canvas c, int b) {
        boolean lit = held(b);
        float x = bx[b], y = by[b], r = br[b], s = 0.45f * r;
        fill.setColor(lit ? 0x8CFFFFFF : 0x40000000);
        c.drawCircle(x, y, r, fill);
        stroke.setColor(lit ? 0xE6FFFFFF : 0x80FFFFFF);
        c.drawCircle(x, y, r, stroke);
        stroke.setColor((SYMBOL[b] & 0x00FFFFFF) | (lit ? 0xFF000000 : 0xC8000000));
        switch (b) {
            case TRIANGLE:
                path.reset();
                path.moveTo(x, y - s);
                path.lineTo(x + 0.95f * s, y + 0.65f * s);
                path.lineTo(x - 0.95f * s, y + 0.65f * s);
                path.close();
                c.drawPath(path, stroke);
                break;
            case CIRCLE:
                c.drawCircle(x, y, 0.85f * s, stroke);
                break;
            case CROSS:
                c.drawLine(x - 0.75f * s, y - 0.75f * s, x + 0.75f * s, y + 0.75f * s, stroke);
                c.drawLine(x - 0.75f * s, y + 0.75f * s, x + 0.75f * s, y - 0.75f * s, stroke);
                break;
            default: // square
                c.drawRect(x - 0.7f * s, y - 0.7f * s, x + 0.7f * s, y + 0.7f * s, stroke);
        }
    }

    private static float sq(float v) {
        return v * v;
    }
}
