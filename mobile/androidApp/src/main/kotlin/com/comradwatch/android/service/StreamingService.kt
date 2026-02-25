package com.comradwatch.android.service

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.app.Service
import android.content.Context
import android.content.Intent
import android.content.pm.ServiceInfo
import android.os.Build
import android.os.IBinder
import androidx.core.app.NotificationCompat
import com.comradwatch.android.MainActivity

/**
 * Foreground service that keeps the app alive during recording.
 *
 * This service doesn't do the actual streaming — that happens in
 * CameraPreviewView. This service just tells Android "don't kill me,
 * I'm recording important video."
 *
 * When the user stops recording, the service stops too.
 */
class StreamingService : Service() {

    override fun onCreate() {
        super.onCreate()
        createNotificationChannel()
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        when (intent?.action) {
            ACTION_START -> startForegroundService()
            ACTION_STOP -> {
                stopForeground(STOP_FOREGROUND_REMOVE)
                stopSelf()
            }
        }
        return START_STICKY // Restart if killed
    }

    private fun startForegroundService() {
        val notification = buildNotification()

        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.UPSIDE_DOWN_CAKE) {
            startForeground(
                NOTIFICATION_ID,
                notification,
                ServiceInfo.FOREGROUND_SERVICE_TYPE_CAMERA or
                    ServiceInfo.FOREGROUND_SERVICE_TYPE_MICROPHONE
            )
        } else {
            startForeground(NOTIFICATION_ID, notification)
        }
    }

    private fun buildNotification(): Notification {
        val openAppIntent = Intent(this, MainActivity::class.java)
        val pendingIntent = PendingIntent.getActivity(
            this, 0, openAppIntent,
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE
        )

        return NotificationCompat.Builder(this, CHANNEL_ID)
            .setContentTitle("Comrad Watch")
            .setContentText("Recording and streaming...")
            .setSmallIcon(android.R.drawable.ic_media_play)
            .setOngoing(true)
            .setContentIntent(pendingIntent)
            .setForegroundServiceBehavior(NotificationCompat.FOREGROUND_SERVICE_IMMEDIATE)
            .build()
    }

    private fun createNotificationChannel() {
        val channel = NotificationChannel(
            CHANNEL_ID,
            "Recording",
            NotificationManager.IMPORTANCE_LOW // Low = no sound, just shows in tray
        ).apply {
            description = "Active recording notification"
        }
        val manager = getSystemService(NotificationManager::class.java)
        manager.createNotificationChannel(channel)
    }

    override fun onBind(intent: Intent?): IBinder? = null

    companion object {
        private const val CHANNEL_ID = "recording_channel"
        private const val NOTIFICATION_ID = 1
        const val ACTION_START = "com.comradwatch.START_RECORDING"
        const val ACTION_STOP = "com.comradwatch.STOP_RECORDING"

        fun start(context: Context) {
            val intent = Intent(context, StreamingService::class.java).apply {
                action = ACTION_START
            }
            context.startForegroundService(intent)
        }

        fun stop(context: Context) {
            val intent = Intent(context, StreamingService::class.java).apply {
                action = ACTION_STOP
            }
            context.startService(intent)
        }
    }
}
