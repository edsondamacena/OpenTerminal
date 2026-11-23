package br.edson.openterminal;

import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.app.Service;
import android.content.Context;
import android.content.Intent;
import android.content.pm.ServiceInfo;
import android.os.Build;
import android.os.Handler;
import android.os.IBinder;
import android.os.Looper;

/**
 * Mantém o app vivo enquanto o login do Google acontece no navegador. Sem isso o Android (e o
 * MIUI com mais força) congela o app em segundo plano, o núcleo não responde ao retorno no
 * 127.0.0.1 e o navegador fica carregando depois do "Confirmar".
 */
public class LoginService extends Service {
    private static final String CHANNEL = "login";
    private static final long MAX_MS = 5 * 60 * 1000;
    private final Handler handler = new Handler(Looper.getMainLooper());

    static void start(Context c) {
        try {
            c.startForegroundService(new Intent(c, LoginService.class));
        } catch (RuntimeException ignored) {
            // Sem permissão de serviço em primeiro plano: o login ainda pode funcionar se o
            // usuário voltar ao app logo depois de confirmar.
        }
    }

    static void stop(Context c) {
        c.stopService(new Intent(c, LoginService.class));
    }

    @Override
    public int onStartCommand(Intent intent, int flags, int startId) {
        NotificationManager nm = getSystemService(NotificationManager.class);
        if (nm != null && nm.getNotificationChannel(CHANNEL) == null) {
            nm.createNotificationChannel(new NotificationChannel(CHANNEL, "Login do Google", NotificationManager.IMPORTANCE_LOW));
        }
        Notification n = new Notification.Builder(this, CHANNEL)
                .setSmallIcon(R.mipmap.ic_launcher)
                .setContentTitle("OpenTerminal")
                .setContentText("Conectando ao Google…")
                .setOngoing(true)
                .build();
        if (Build.VERSION.SDK_INT >= 29) {
            startForeground(1, n, ServiceInfo.FOREGROUND_SERVICE_TYPE_DATA_SYNC);
        } else {
            startForeground(1, n);
        }
        handler.removeCallbacksAndMessages(null);
        handler.postDelayed(this::stopSelf, MAX_MS);
        return START_NOT_STICKY;
    }

    @Override
    public void onDestroy() {
        handler.removeCallbacksAndMessages(null);
        super.onDestroy();
    }

    @Override
    public IBinder onBind(Intent intent) {
        return null;
    }
}
