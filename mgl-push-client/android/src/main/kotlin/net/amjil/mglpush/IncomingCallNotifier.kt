package net.amjil.mglpush

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import android.os.Build
import androidx.core.app.NotificationCompat
import androidx.core.app.NotificationManagerCompat

/**
 * High-priority full-screen Incoming Call notification (Android Call Control fallback).
 * Prefer [TelecomIncomingCall] (ConnectionService) when available.
 */
object IncomingCallNotifier {
    const val CHANNEL_ID = "mgl_incoming_call"
    const val NOTIFICATION_ID = 0x4D4743 // 'MGC'
    const val ACTION_ACCEPT = "net.amjil.mglpush.INCOMING_CALL_ACCEPT"
    const val ACTION_REJECT = "net.amjil.mglpush.INCOMING_CALL_REJECT"
    const val ACTION_TIMEOUT = "net.amjil.mglpush.INCOMING_CALL_TIMEOUT"
    const val EXTRA_CALL_ID = "call_id"
    const val EXTRA_EVENT_ID = "event_id"
    const val EXTRA_CALLER_NAME = "caller_display_name"
    const val EXTRA_MEDIA_TYPE = "media_type"
    const val EXTRA_PAYLOAD = "payload_json"

    fun ensureChannel(context: Context) {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.O) return
        val mgr = context.getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
        val existing = mgr.getNotificationChannel(CHANNEL_ID)
        if (existing != null) return
        val channel = NotificationChannel(
            CHANNEL_ID,
            "Incoming calls",
            NotificationManager.IMPORTANCE_HIGH
        ).apply {
            description = "Incoming call alerts"
            setSound(null, null)
            enableVibration(true)
            lockscreenVisibility = Notification.VISIBILITY_PUBLIC
        }
        mgr.createNotificationChannel(channel)
    }

    /** Prefer Telecom; fall back to full-screen notification. */
    fun show(context: Context, event: Map<String, Any?>) {
        if (TelecomIncomingCall.show(context, event)) {
            return
        }
        showNotificationOnly(context, event)
    }

    fun showNotificationOnly(context: Context, event: Map<String, Any?>) {
        ensureChannel(context)
        val data = stringData(event)
        val callId = data["call_id"] ?: data["callId"] ?: return
        val eventId = (event["id"] as? String).orEmpty()
        val caller = data["caller_display_name"]
            ?: data["caller_id"]
            ?: data["callerId"]
            ?: "Incoming Call"
        val media = data["media_type"] ?: data["mediaType"] ?: "audio"
        val payloadJson = mapToJsonString(data)

        val fullScreen = PendingIntent.getActivity(
            context,
            callId.hashCode(),
            IncomingCallActivity.intent(
                context,
                callId = callId,
                eventId = eventId,
                callerName = caller,
                mediaType = media,
                payloadJson = payloadJson,
                provider = event["provider"] as? String
            ),
            pendingFlags()
        )

        val accept = PendingIntent.getBroadcast(
            context,
            callId.hashCode() + 1,
            Intent(ACTION_ACCEPT).setPackage(context.packageName)
                .putExtra(EXTRA_CALL_ID, callId)
                .putExtra(EXTRA_EVENT_ID, eventId)
                .putExtra(EXTRA_PAYLOAD, payloadJson)
                .putExtra("provider", event["provider"] as? String),
            pendingFlags()
        )
        val reject = PendingIntent.getBroadcast(
            context,
            callId.hashCode() + 2,
            Intent(ACTION_REJECT).setPackage(context.packageName)
                .putExtra(EXTRA_CALL_ID, callId)
                .putExtra(EXTRA_EVENT_ID, eventId)
                .putExtra(EXTRA_PAYLOAD, payloadJson)
                .putExtra("provider", event["provider"] as? String),
            pendingFlags()
        )

        val notification = NotificationCompat.Builder(context, CHANNEL_ID)
            .setSmallIcon(android.R.drawable.sym_call_incoming)
            .setContentTitle(caller)
            .setContentText(if (media == "video") "Incoming video call" else "Incoming call")
            .setCategory(NotificationCompat.CATEGORY_CALL)
            .setPriority(NotificationCompat.PRIORITY_MAX)
            .setOngoing(true)
            .setAutoCancel(false)
            .setTimeoutAfter(timeoutMs(data))
            .setFullScreenIntent(fullScreen, true)
            .setContentIntent(fullScreen)
            .addAction(0, "Reject", reject)
            .addAction(0, "Accept", accept)
            .setVisibility(NotificationCompat.VISIBILITY_PUBLIC)
            .build()

        try {
            NotificationManagerCompat.from(context).notify(notificationId(callId), notification)
        } catch (_: SecurityException) {
            // POST_NOTIFICATIONS may be denied; still try to launch activity.
            try {
                context.startActivity(
                    IncomingCallActivity.intent(
                        context,
                        callId = callId,
                        eventId = eventId,
                        callerName = caller,
                        mediaType = media,
                        payloadJson = payloadJson,
                        provider = event["provider"] as? String
                    ).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
                )
            } catch (_: Exception) {
            }
        }

        IncomingCallRegistry.put(callId, event)
    }

    fun dismiss(context: Context, callId: String?) {
        TelecomIncomingCall.end(context, callId)
        if (callId.isNullOrEmpty()) {
            NotificationManagerCompat.from(context).cancel(NOTIFICATION_ID)
            IncomingCallRegistry.clear()
            return
        }
        NotificationManagerCompat.from(context).cancel(notificationId(callId))
        IncomingCallRegistry.remove(callId)
    }

    fun notificationId(callId: String): Int =
        NOTIFICATION_ID xor callId.hashCode()

    private fun pendingFlags(): Int {
        var flags = PendingIntent.FLAG_UPDATE_CURRENT
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.M) {
            flags = flags or PendingIntent.FLAG_IMMUTABLE
        }
        return flags
    }

    private fun timeoutMs(data: Map<String, String>): Long {
        val expires = data["expires_at"]?.toLongOrNull() ?: return 35_000L
        val now = System.currentTimeMillis() / 1000
        val remaining = (expires - now).coerceAtLeast(5).coerceAtMost(60)
        return remaining * 1000L
    }

    @Suppress("UNCHECKED_CAST")
    private fun stringData(event: Map<String, Any?>): Map<String, String> {
        val raw = event["data"]
        val out = mutableMapOf<String, String>()
        if (raw is Map<*, *>) {
            for ((k, v) in raw) {
                if (k != null && v != null) out[k.toString()] = v.toString()
            }
        }
        return out
    }

    private fun mapToJsonString(map: Map<String, String>): String {
        val sb = StringBuilder("{")
        var first = true
        for ((k, v) in map) {
            if (!first) sb.append(',')
            first = false
            sb.append('"').append(escape(k)).append('"').append(':')
            sb.append('"').append(escape(v)).append('"')
        }
        sb.append('}')
        return sb.toString()
    }

    private fun escape(s: String): String =
        s.replace("\\", "\\\\").replace("\"", "\\\"")
}

/** In-memory registry of active ringing calls. */
object IncomingCallRegistry {
    private val active = linkedMapOf<String, Map<String, Any?>>()

    @Synchronized
    fun put(callId: String, event: Map<String, Any?>) {
        active[callId] = event
    }

    @Synchronized
    fun remove(callId: String) {
        active.remove(callId)
    }

    @Synchronized
    fun clear() {
        active.clear()
    }

    @Synchronized
    fun get(callId: String): Map<String, Any?>? = active[callId]
}
