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
import android.widget.Button;
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
    private Button ctrlButton;
    private boolean ctrlArmed;
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
            case "openurl":
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

    private View buildKeyBar() {
        LinearLayout bar = new LinearLayout(this);
        bar.setOrientation(LinearLayout.HORIZONTAL);
        bar.setBackgroundColor(Color.rgb(24, 24, 27));
        // Setas ← ↑ → (histórico e edição da linha), as teclas usadas no dia a dia. Para voltar
        // outras, acrescente aqui: {"ESC", "\u001b"}, {"CTRL", null} (arma o Ctrl), {"TAB", "\t"}...
        bar.setGravity(Gravity.END);
        // CTRL arma o Ctrl para a próxima letra do teclado; ^C interrompe direto; COLAR manda o
        // texto da área de transferência do Android (o Ctrl+V do celular).
        String[][] keys = {
                {"CTRL", null},
                {"^C", "\u0003"},
                {"COLAR", PASTE},
                {"←", "\u001b[D"},
                {"↑", "\u001b[A"},
                {"→", "\u001b[C"},
        };
        for (String[] k : keys) {
            Button b = new Button(this, null, android.R.attr.borderlessButtonStyle);
            b.setText(k[0]);
            b.setAllCaps(false);
            b.setTextColor(Color.rgb(228, 228, 231));
            b.setMinWidth(0);
            b.setMinimumWidth(0);
            b.setPadding(dp(14), 0, dp(14), 0);
            b.setTextSize(k[0].length() > 2 ? 14 : 20);
            b.setGravity(Gravity.CENTER);
            String seq = k[1];
            if (seq == null) {
                ctrlButton = b;
                b.setOnClickListener(v -> setCtrl(!ctrlArmed));
            } else if (PASTE.equals(seq)) {
                b.setOnClickListener(v -> pasteClipboard());
            } else {
                b.setOnClickListener(v -> sendKeys(seq));
            }
            bar.addView(b, new LinearLayout.LayoutParams(LinearLayout.LayoutParams.WRAP_CONTENT, dp(42)));
        }
        return bar;
    }

    private void setCtrl(boolean armed) {
        if (ctrlButton == null) return;
        ctrlArmed = armed;
        ctrlButton.setTextColor(armed ? Color.rgb(249, 115, 22) : Color.rgb(228, 228, 231));
        web.evaluateJavascript("window.openterminalKeys&&window.openterminalKeys.ctrl(" + armed + ")", null);
    }

    private static final String PASTE = "__paste__";

    private void pasteClipboard() {
        android.content.ClipboardManager cm =
                (android.content.ClipboardManager) getSystemService(CLIPBOARD_SERVICE);
        if (cm == null || !cm.hasPrimaryClip() || cm.getPrimaryClip().getItemCount() == 0) return;
        CharSequence text = cm.getPrimaryClip().getItemAt(0).coerceToText(this);
        if (text != null && text.length() > 0) sendKeys(text.toString());
    }

    private void sendKeys(String seq) {
        web.evaluateJavascript("window.openterminalKeys&&window.openterminalKeys.send(" + JSONObject.quote(seq) + ")", null);
    }

    private int dp(int v) {
        return Math.round(v * getResources().getDisplayMetrics().density);
    }

    private class JsBridge {
        @JavascriptInterface
        public void ctrlUsed() {
            runOnUiThread(() -> {
                ctrlArmed = false;
                if (ctrlButton != null) ctrlButton.setTextColor(Color.rgb(228, 228, 231));
            });
        }
    }

    // Voltar: a página decide primeiro (do console volta para a lista de hosts). Se ela não tratar,
    // o app vai para segundo plano sem fechar: as sessões abertas continuam.
    @Override
    public void onResume() {
        super.onResume();
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
