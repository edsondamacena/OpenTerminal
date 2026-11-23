package br.edson.openterminal;

import android.security.keystore.KeyGenParameterSpec;
import android.security.keystore.KeyProperties;
import android.util.Base64;
import java.security.KeyStore;
import javax.crypto.Cipher;
import javax.crypto.KeyGenerator;
import javax.crypto.SecretKey;
import javax.crypto.spec.GCMParameterSpec;
import android.app.Activity;
import android.content.ActivityNotFoundException;
import android.content.Intent;
import android.database.Cursor;
import android.graphics.Color;
import android.graphics.Insets;
import android.net.Uri;
import android.os.Build;
import android.os.Bundle;
import android.provider.OpenableColumns;
import android.view.Gravity;
import android.view.View;
import android.view.WindowInsets;
import android.webkit.JavascriptInterface;
import android.webkit.WebResourceRequest;
import android.webkit.WebSettings;
import android.webkit.WebView;
import android.webkit.WebViewClient;
import android.widget.LinearLayout;
import android.widget.Toast;

import org.json.JSONException;
import org.json.JSONObject;

import java.io.BufferedReader;
import java.io.File;
import java.io.FileInputStream;
import java.io.FileOutputStream;
import java.io.IOException;
import java.io.InputStream;
import java.io.InputStreamReader;
import java.io.OutputStream;
import java.io.OutputStreamWriter;
import java.io.Writer;
import java.nio.charset.StandardCharsets;
import java.util.HashMap;
import java.util.Map;

/**
 * Casca do OpenTerminal no Android: executa o núcleo Go (libotcore.so), abre a interface dele num
 * WebView e atende os pedidos de arquivo e de link pelo stdin/stdout do processo.
 */
public class MainActivity extends Activity {
    private static final int REQ_OPEN = 1;
    private static final int REQ_SAVE = 2;

    // Um núcleo por processo do app: sobrevive à recriação da Activity.
    private static Process core;
    private static Writer coreIn;
    private static String coreUrl;

    private WebView web;
    private android.widget.ImageView pasteButton;
    private int pendingId;
    private String pendingSaveName;
    private final Map<String, Uri> saveTargets = new HashMap<>();

    @Override
    protected void onCreate(Bundle state) {
        super.onCreate(state);
        LinearLayout root = new LinearLayout(this);
        root.setOrientation(LinearLayout.VERTICAL);
        root.setBackgroundColor(Color.BLACK);

        web = new WebView(this);
        WebSettings ws = web.getSettings();
        ws.setJavaScriptEnabled(true);
        ws.setDomStorageEnabled(true);
        ws.setSaveFormData(false);
        web.addJavascriptInterface(new JsBridge(), "OpenTerminalAndroid");
        // Só o núcleo local abre no WebView; qualquer outro link vai para o navegador.
        web.setWebViewClient(new WebViewClient() {
            @Override
            public boolean shouldOverrideUrlLoading(WebView view, WebResourceRequest req) {
                Uri u = req.getUrl();
                if ("127.0.0.1".equals(u.getHost())) return false;
                try {
                    startActivity(new Intent(Intent.ACTION_VIEW, u));
                } catch (ActivityNotFoundException ignored) {
                }
                return true;
            }
        });
        root.addView(web, new LinearLayout.LayoutParams(LinearLayout.LayoutParams.MATCH_PARENT, 0, 1f));
        root.addView(buildKeyBar());
        setContentView(root);

        // targetSdk 35 desenha atrás das barras do sistema: o conteúdo recua delas e do teclado.
        if (Build.VERSION.SDK_INT >= 30) {
            root.setOnApplyWindowInsetsListener((v, insets) -> {
                Insets b = insets.getInsets(WindowInsets.Type.systemBars() | WindowInsets.Type.ime());
                v.setPadding(b.left, b.top, b.right, b.bottom);
                return WindowInsets.CONSUMED;
            });
        }

        if (coreUrl != null && core != null && core.isAlive()) {
            web.loadUrl(coreUrl);
        } else {
            startCore();
        }
    }

    private void startCore() {
        File bin = new File(getApplicationInfo().nativeLibraryDir, "libotcore.so");
        File data = new File(getFilesDir(), "data");
        try {
            ProcessBuilder pb = new ProcessBuilder(bin.getAbsolutePath(), data.getAbsolutePath());
            pb.environment().put("TMPDIR", getCacheDir().getAbsolutePath());
            pb.redirectErrorStream(false);
            core = pb.start();
            coreIn = new OutputStreamWriter(core.getOutputStream(), StandardCharsets.UTF_8);
        } catch (IOException e) {
            Toast.makeText(this, "Não foi possível iniciar o núcleo: " + e.getMessage(), Toast.LENGTH_LONG).show();
            return;
        }
        Process p = core;
        new Thread(() -> readCore(p), "otcore-out").start();
        new Thread(() -> drainErrors(p), "otcore-err").start();
    }

    private void readCore(Process p) {
        try (BufferedReader r = new BufferedReader(new InputStreamReader(p.getInputStream(), StandardCharsets.UTF_8))) {
            String line;
            while ((line = r.readLine()) != null) {
                JSONObject msg;
                try {
                    msg = new JSONObject(line);
                } catch (JSONException e) {
                    continue;
                }
                JSONObject m = msg;
                runOnUiThread(() -> handle(m));
            }
        } catch (IOException ignored) {
        }
        runOnUiThread(() -> Toast.makeText(this, "O núcleo do OpenTerminal parou.", Toast.LENGTH_LONG).show());
    }

    private void drainErrors(Process p) {
        try (BufferedReader r = new BufferedReader(new InputStreamReader(p.getErrorStream(), StandardCharsets.UTF_8))) {
            String line;
            while ((line = r.readLine()) != null) {
                android.util.Log.w("OpenTerminal", line);
            }
        } catch (IOException ignored) {
        }
    }

    private void handle(JSONObject m) {
        switch (m.optString("op")) {
            case "url":
                coreUrl = m.optString("url");
                web.loadUrl(coreUrl);
                break;
            case "keyget":
                reply(m.optInt("id"), keyGet(m.optString("name")));
                break;
            case "keyset":
                reply(m.optInt("id"), keySet(m.optString("name"), m.optString("data")) ? "ok" : "");
                break;
            case "keydel":
                getSharedPreferences(KEY_PREFS, MODE_PRIVATE).edit().remove(m.optString("name")).apply();
                reply(m.optInt("id"), "ok");
                break;
            case "open": {
                pendingId = m.optInt("id");
                Intent i = new Intent(Intent.ACTION_OPEN_DOCUMENT);
                i.addCategory(Intent.CATEGORY_OPENABLE);
                i.setType("*/*");
                startPicker(i, REQ_OPEN);
                break;
            }
            case "save": {
                pendingId = m.optInt("id");
                pendingSaveName = m.optString("name", "openterminal");
                Intent i = new Intent(Intent.ACTION_CREATE_DOCUMENT);
                i.addCategory(Intent.CATEGORY_OPENABLE);
                i.setType("application/octet-stream");
                i.putExtra(Intent.EXTRA_TITLE, pendingSaveName);
                startPicker(i, REQ_SAVE);
                break;
            }
            case "saved":
                finishSave(m.optString("path"));
                break;
            case "installapk":
                installApk(new File(m.optString("path")));
                break;
            case "openurl":
                // Login do Google: o app não pode congelar enquanto o navegador está na frente.
                if ("accounts.google.com".equals(Uri.parse(m.optString("url")).getHost())) LoginService.start(this);
                try {
                    startActivity(new Intent(Intent.ACTION_VIEW, Uri.parse(m.optString("url"))));
                } catch (ActivityNotFoundException ignored) {
                }
                break;
            default:
                break;
        }
    }

    private void startPicker(Intent i, int req) {
        try {
            startActivityForResult(i, req);
        } catch (ActivityNotFoundException e) {
            reply(pendingId, "");
        }
    }

    @Override
    protected void onActivityResult(int req, int result, Intent data) {
        super.onActivityResult(req, result, data);
        Uri uri = result == RESULT_OK && data != null ? data.getData() : null;
        if (uri == null) {
            reply(pendingId, "");
            return;
        }
        try {
            if (req == REQ_OPEN) {
                // O núcleo lê caminhos: o arquivo escolhido vira uma cópia privada no cache.
                File dir = new File(getCacheDir(), "open");
                dir.mkdirs();
                File out = new File(dir, safeName(displayName(uri)));
                try (InputStream in = getContentResolver().openInputStream(uri);
                     OutputStream o = new FileOutputStream(out)) {
                    copy(in, o);
                }
                reply(pendingId, out.getAbsolutePath());
            } else if (req == REQ_SAVE) {
                File dir = new File(getCacheDir(), "save");
                dir.mkdirs();
                File tmp = new File(dir, safeName(pendingSaveName));
                tmp.delete();
                saveTargets.put(tmp.getAbsolutePath(), uri);
                reply(pendingId, tmp.getAbsolutePath());
            }
        } catch (IOException e) {
            Toast.makeText(this, "Falha ao ler o arquivo: " + e.getMessage(), Toast.LENGTH_LONG).show();
            reply(pendingId, "");
        }
    }

    private void finishSave(String path) {
        Uri uri = saveTargets.remove(path);
        if (uri == null) return;
        File tmp = new File(path);
        try (InputStream in = new FileInputStream(tmp);
             OutputStream o = getContentResolver().openOutputStream(uri, "wt")) {
            copy(in, o);
            Toast.makeText(this, "Arquivo salvo.", Toast.LENGTH_SHORT).show();
        } catch (IOException e) {
            Toast.makeText(this, "Falha ao salvar: " + e.getMessage(), Toast.LENGTH_LONG).show();
        } finally {
            tmp.delete();
        }
    }

    // --- Atualização: instala o APK que o núcleo baixou e conferiu (SHA256SUMS) ----------------

    private void installApk(File apk) {
        if (!getPackageManager().canRequestPackageInstalls()) {
            // Uma vez só: o Android exige liberar "instalar apps desta fonte" para o OpenTerminal.
            Toast.makeText(this, "Permita instalar atualizações do OpenTerminal e toque em Atualizar de novo.", Toast.LENGTH_LONG).show();
            try {
                startActivity(new Intent(android.provider.Settings.ACTION_MANAGE_UNKNOWN_APP_SOURCES,
                        Uri.parse("package:" + getPackageName())));
            } catch (ActivityNotFoundException ignored) {
            }
            return;
        }
        Toast.makeText(this, "Instalando a atualização…", Toast.LENGTH_SHORT).show();
        new Thread(() -> {
            android.content.pm.PackageInstaller pi = getPackageManager().getPackageInstaller();
            android.content.pm.PackageInstaller.SessionParams params =
                    new android.content.pm.PackageInstaller.SessionParams(
                            android.content.pm.PackageInstaller.SessionParams.MODE_FULL_INSTALL);
            params.setAppPackageName(getPackageName());
            if (Build.VERSION.SDK_INT >= 31) {
                // Sem confirmação quando o próprio OpenTerminal instalou a versão atual.
                params.setRequireUserAction(android.content.pm.PackageInstaller.SessionParams.USER_ACTION_NOT_REQUIRED);
            }
            try {
                int id = pi.createSession(params);
                try (android.content.pm.PackageInstaller.Session session = pi.openSession(id)) {
                    try (InputStream in = new FileInputStream(apk);
                         OutputStream out = session.openWrite("base.apk", 0, apk.length())) {
                        copy(in, out);
                        session.fsync(out);
                    }
                    Intent result = new Intent(this, InstallReceiver.class);
                    android.app.PendingIntent pending = android.app.PendingIntent.getBroadcast(this, id, result,
                            android.app.PendingIntent.FLAG_UPDATE_CURRENT | android.app.PendingIntent.FLAG_MUTABLE);
                    session.commit(pending.getIntentSender());
                }
            } catch (IOException | RuntimeException e) {
                runOnUiThread(() -> Toast.makeText(this, "Falha ao instalar: " + e.getMessage(), Toast.LENGTH_LONG).show());
            } finally {
                apk.delete();
            }
        }, "install-update").start();
    }

    // --- Desbloqueio automático: segredo cifrado com chave do Android Keystore -----------------

    private static final String KEY_ALIAS = "openterminal-unlock";
    private static final String KEY_PREFS = "openterminal-unlock";

    private SecretKey unlockKey() throws Exception {
        KeyStore ks = KeyStore.getInstance("AndroidKeyStore");
        ks.load(null);
        if (ks.containsAlias(KEY_ALIAS)) {
            return ((KeyStore.SecretKeyEntry) ks.getEntry(KEY_ALIAS, null)).getSecretKey();
        }
        KeyGenerator g = KeyGenerator.getInstance(KeyProperties.KEY_ALGORITHM_AES, "AndroidKeyStore");
        g.init(new KeyGenParameterSpec.Builder(KEY_ALIAS,
                KeyProperties.PURPOSE_ENCRYPT | KeyProperties.PURPOSE_DECRYPT)
                .setBlockModes(KeyProperties.BLOCK_MODE_GCM)
                .setEncryptionPaddings(KeyProperties.ENCRYPTION_PADDING_NONE)
                .build());
        return g.generateKey();
    }

    private boolean keySet(String name, String data) {
        try {
            Cipher c = Cipher.getInstance("AES/GCM/NoPadding");
            c.init(Cipher.ENCRYPT_MODE, unlockKey());
            byte[] ct = c.doFinal(Base64.decode(data, Base64.NO_WRAP));
            String v = Base64.encodeToString(c.getIV(), Base64.NO_WRAP) + ":" + Base64.encodeToString(ct, Base64.NO_WRAP);
            getSharedPreferences(KEY_PREFS, MODE_PRIVATE).edit().putString(name, v).apply();
            return true;
        } catch (Exception e) {
            return false;
        }
    }

    private String keyGet(String name) {
        String v = getSharedPreferences(KEY_PREFS, MODE_PRIVATE).getString(name, null);
        if (v == null) return "";
        try {
            String[] parts = v.split(":", 2);
            Cipher c = Cipher.getInstance("AES/GCM/NoPadding");
            c.init(Cipher.DECRYPT_MODE, unlockKey(), new GCMParameterSpec(128, Base64.decode(parts[0], Base64.NO_WRAP)));
            return Base64.encodeToString(c.doFinal(Base64.decode(parts[1], Base64.NO_WRAP)), Base64.NO_WRAP);
        } catch (Exception e) {
            return "";
        }
    }

    private void reply(int id, String path) {
        if (coreIn == null) return;
        try {
            JSONObject r = new JSONObject();
            r.put("id", id);
            r.put("path", path);
            synchronized (MainActivity.class) {
                coreIn.write(r.toString());
                coreIn.write('\n');
                coreIn.flush();
            }
        } catch (JSONException | IOException ignored) {
        }
    }

    private String displayName(Uri uri) {
        try (Cursor c = getContentResolver().query(uri, new String[]{OpenableColumns.DISPLAY_NAME}, null, null, null)) {
            if (c != null && c.moveToFirst()) return c.getString(0);
        } catch (RuntimeException ignored) {
        }
        return "arquivo";
    }

    private static String safeName(String name) {
        String n = name == null ? "" : name.replaceAll("[/\\\\\\x00]", "_").trim();
        return n.isEmpty() || n.equals(".") || n.equals("..") ? "arquivo" : n;
    }

    private static void copy(InputStream in, OutputStream out) throws IOException {
        if (in == null || out == null) throw new IOException("sem acesso ao arquivo");
        byte[] buf = new byte[64 * 1024];
        int n;
        while ((n = in.read(buf)) > 0) out.write(buf, 0, n);
    }

    // --- Barra de teclas -------------------------------------------------------------------

    private static final int KEY_BG = Color.rgb(39, 39, 42);
    private static final int KEY_BG_PRESSED = Color.rgb(63, 63, 70);

    private View buildKeyBar() {
        LinearLayout bar = new LinearLayout(this);
        bar.setOrientation(LinearLayout.HORIZONTAL);
        bar.setBackgroundColor(Color.rgb(24, 24, 27));
        bar.setPadding(dp(6), dp(6), dp(6), dp(6));
        // Ícones: Ctrl+C (interrompe), Colar (área de transferência do celular) e as setas.
        addKey(bar, R.drawable.key_interrupt, "Ctrl+C", v -> sendKeys("\u0003"));
        pasteButton = addKey(bar, R.drawable.key_paste, "Colar", v -> pasteClipboard());
        addKey(bar, R.drawable.key_left, "Seta para a esquerda", v -> sendKeys("\u001b[D"));
        addKey(bar, R.drawable.key_up, "Seta para cima", v -> sendKeys("\u001b[A"));
        addKey(bar, R.drawable.key_down, "Seta para baixo", v -> sendKeys("\u001b[B"));
        addKey(bar, R.drawable.key_right, "Seta para a direita", v -> sendKeys("\u001b[C"));
        android.content.ClipboardManager cm =
                (android.content.ClipboardManager) getSystemService(CLIPBOARD_SERVICE);
        if (cm != null) cm.addPrimaryClipChangedListener(this::refreshPaste);
        refreshPaste();
        return bar;
    }

    private android.widget.ImageView addKey(LinearLayout bar, int icon, String description, View.OnClickListener action) {
        android.widget.ImageView b = new android.widget.ImageView(this);
        b.setImageResource(icon);
        b.setScaleType(android.widget.ImageView.ScaleType.CENTER);
        b.setContentDescription(description);
        b.setBackground(keyBackground());
        b.setClickable(true);
        b.setFocusable(false);
        b.setOnClickListener(v -> {
            v.performHapticFeedback(android.view.HapticFeedbackConstants.KEYBOARD_TAP);
            action.onClick(v);
        });
        LinearLayout.LayoutParams lp = new LinearLayout.LayoutParams(0, dp(42), 1f);
        lp.setMargins(dp(3), 0, dp(3), 0);
        bar.addView(b, lp);
        return b;
    }

    /** Colar fica apagado quando não há nada copiado no celular. */
    private void refreshPaste() {
        if (pasteButton == null) return;
        android.content.ClipboardManager cm =
                (android.content.ClipboardManager) getSystemService(CLIPBOARD_SERVICE);
        boolean has = cm != null && cm.hasPrimaryClip();
        pasteButton.setEnabled(has);
        pasteButton.setAlpha(has ? 1f : 0.4f);
    }

    private android.graphics.drawable.Drawable keyBackground() {
        android.graphics.drawable.GradientDrawable normal = new android.graphics.drawable.GradientDrawable();
        normal.setCornerRadius(dp(10));
        normal.setColor(KEY_BG);
        android.graphics.drawable.GradientDrawable pressed = new android.graphics.drawable.GradientDrawable();
        pressed.setCornerRadius(dp(10));
        pressed.setColor(KEY_BG_PRESSED);
        android.graphics.drawable.StateListDrawable states = new android.graphics.drawable.StateListDrawable();
        states.addState(new int[]{android.R.attr.state_pressed}, pressed);
        states.addState(new int[]{}, normal);
        return states;
    }

    private static final String PASTE = "__paste__";

    private void pasteClipboard() {
        android.content.ClipboardManager cm =
                (android.content.ClipboardManager) getSystemService(CLIPBOARD_SERVICE);
        if (cm == null || !cm.hasPrimaryClip() || cm.getPrimaryClip().getItemCount() == 0) return;
        CharSequence text = cm.getPrimaryClip().getItemAt(0).coerceToText(this);
        if (text == null || text.length() == 0) return;
        // paste() passa pela colagem do xterm (colchetes e confirmação de várias linhas).
        String js = JSONObject.quote(text.toString());
        web.evaluateJavascript("(function(k,t){if(!k)return;(k.paste||k.send)(t);})(window.openterminalKeys," + js + ")", null);
    }

    private void sendKeys(String seq) {
        web.evaluateJavascript("window.openterminalKeys&&window.openterminalKeys.send(" + JSONObject.quote(seq) + ")", null);
    }

    private int dp(int v) {
        return Math.round(v * getResources().getDisplayMetrics().density);
    }

    private class JsBridge {
        @JavascriptInterface
        public void copy(String text) {
            runOnUiThread(() -> {
                android.content.ClipboardManager cm =
                        (android.content.ClipboardManager) getSystemService(CLIPBOARD_SERVICE);
                if (cm != null) cm.setPrimaryClip(android.content.ClipData.newPlainText("OpenTerminal", text));
            });
        }

        @JavascriptInterface
        public void ctrlUsed() {
            // Sem tecla Ctrl na barra: nada a desarmar.
        }
    }

    // Voltar: a página decide primeiro (do console volta para a lista de hosts). Se ela não tratar,
    // o app vai para segundo plano sem fechar: as sessões abertas continuam.
    @Override
    public void onResume() {
        super.onResume();
        LoginService.stop(this);
        refreshPaste();
        // Voltou do segundo plano: o front sincroniza com a nuvem na hora.
        if (web != null) web.evaluateJavascript("window.openterminalResume&&window.openterminalResume()", null);
    }

    @Override
    public void onBackPressed() {
        if (coreUrl == null) {
            moveTaskToBack(true);
            return;
        }
        web.evaluateJavascript("(window.openterminalBack ? window.openterminalBack() : false)", handled -> {
            if (!"true".equals(handled)) moveTaskToBack(true);
        });
    }

    @Override
    protected void onDestroy() {
        if (isFinishing() && core != null) {
            core.destroy();
            core = null;
            coreIn = null;
            coreUrl = null;
        }
        web.destroy();
        super.onDestroy();
    }
}
