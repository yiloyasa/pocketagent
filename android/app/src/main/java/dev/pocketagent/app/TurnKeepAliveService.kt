package dev.pocketagent.app

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.Service
import android.content.Intent
import android.os.IBinder

/**
 * 轮次进行中的前台保活服务。
 * 【为什么需要】手机锁屏/切后台时,系统(尤其 MIUI)会激进杀后台进程,
 * 正在跑的 gRPC 流一起被杀,用户回来看到的就是"[连接中断]"。
 * 前台服务 + 通知 = 告诉系统"我在干活别杀我",任务跑完自动停。
 */
class TurnKeepAliveService : Service() {

    override fun onBind(intent: Intent?): IBinder? = null

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        val nm = getSystemService(NotificationManager::class.java)
        nm.createNotificationChannel(
            NotificationChannel(CHANNEL_ID, "任务运行", NotificationManager.IMPORTANCE_LOW)
        )
        val n = Notification.Builder(this, CHANNEL_ID)
            .setSmallIcon(R.drawable.ic_lucide_monitor)
            .setContentTitle("PocketAgent 任务运行中")
            .setContentText("电脑上的 agent 正在执行,断网会自动重连续看结果")
            .setOngoing(true)
            .build()
        startForeground(1, n)
        return START_STICKY
    }

    companion object {
        private const val CHANNEL_ID = "pa_turn"
    }
}
