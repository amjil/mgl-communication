package net.amjil.mglpush

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent

/**
 * Handles Accept / Reject from the Incoming Call notification actions.
 */
class IncomingCallActionReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent?) {
        if (intent == null) return
        val callId = intent.getStringExtra(IncomingCallNotifier.EXTRA_CALL_ID).orEmpty()
        if (callId.isEmpty()) return
        val eventId = intent.getStringExtra(IncomingCallNotifier.EXTRA_EVENT_ID).orEmpty()
        val provider = intent.getStringExtra("provider")
        val payload = IncomingCallActivity.parseJsonMap(
            intent.getStringExtra(IncomingCallNotifier.EXTRA_PAYLOAD).orEmpty()
        )
        val action = when (intent.action) {
            IncomingCallNotifier.ACTION_ACCEPT -> "accepted"
            IncomingCallNotifier.ACTION_REJECT -> "rejected"
            IncomingCallNotifier.ACTION_TIMEOUT -> "timeout"
            else -> return
        }
        IncomingCallActionBridge.emitAction(
            context.applicationContext,
            callId = callId,
            eventId = eventId,
            action = action,
            payload = payload,
            provider = provider
        )
        IncomingCallNotifier.dismiss(context.applicationContext, callId)
        if (action == "accepted") {
            try {
                val launch = context.packageManager.getLaunchIntentForPackage(context.packageName)
                launch?.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_SINGLE_TOP)
                if (launch != null) context.startActivity(launch)
            } catch (_: Exception) {
            }
        }
    }
}

/**
 * Bridges native Accept/Reject into the Flutter EventChannel via [MglPushPlugin].
 */
object IncomingCallActionBridge {
    @Volatile
    var plugin: MglPushPlugin? = null

    fun emitAction(
        context: Context,
        callId: String,
        eventId: String,
        action: String,
        payload: Map<String, String>,
        provider: String?
    ) {
        val data = payload.toMutableMap()
        data["call_id"] = callId
        data["action"] = action
        val id = if (eventId.isNotEmpty()) "$eventId:$action" else "$callId:$action"
        val event = mapOf(
            "version" to 1,
            "type" to "incoming-call",
            "id" to id,
            "timestamp" to (System.currentTimeMillis() / 1000),
            "provider" to (provider ?: "android"),
            "data" to data
        )
        val p = plugin
        if (p != null) {
            p.emitFromNative(event)
        } else {
            PendingNativeStore.save(context, event)
        }
    }
}
