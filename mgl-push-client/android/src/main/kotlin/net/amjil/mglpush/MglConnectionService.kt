package net.amjil.mglpush

import android.os.Build
import android.os.Bundle
import android.telecom.Connection
import android.telecom.ConnectionRequest
import android.telecom.ConnectionService
import android.telecom.DisconnectCause
import android.telecom.PhoneAccountHandle
import android.telecom.TelecomManager
import android.telecom.VideoProfile
import android.util.Log

/**
 * Self-managed [ConnectionService] for system Incoming Call UI.
 */
class MglConnectionService : ConnectionService() {

    override fun onCreate() {
        super.onCreate()
        MglConnectionServiceAppContext.set(applicationContext)
    }

    override fun onCreateIncomingConnection(
        connectionManagerPhoneAccount: PhoneAccountHandle?,
        request: ConnectionRequest?
    ): Connection {
        MglConnectionServiceAppContext.set(applicationContext)
        val extras = callExtras(request)
        val callId = extras.getString(TelecomIncomingCall.EXTRA_CALL_ID).orEmpty()
        val eventId = extras.getString(TelecomIncomingCall.EXTRA_EVENT_ID).orEmpty()
        val caller = extras.getString(TelecomIncomingCall.EXTRA_CALLER_NAME) ?: "Incoming Call"
        val media = extras.getString(TelecomIncomingCall.EXTRA_MEDIA_TYPE) ?: "audio"
        val payloadJson = extras.getString(TelecomIncomingCall.EXTRA_PAYLOAD).orEmpty()
        val provider = extras.getString(TelecomIncomingCall.EXTRA_PROVIDER)
        val payload = IncomingCallActivity.parseJsonMap(payloadJson)

        val conn = MglConnection(
            callId = callId,
            eventId = eventId,
            payload = payload,
            provider = provider
        )
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.N_MR1) {
            conn.connectionProperties = Connection.PROPERTY_SELF_MANAGED
        }
        conn.setAddress(
            TelecomIncomingCall.addressFor(callId.ifEmpty { "unknown" }),
            TelecomManager.PRESENTATION_ALLOWED
        )
        conn.setCallerDisplayName(caller, TelecomManager.PRESENTATION_ALLOWED)
        if (media == "video") {
            conn.videoState = VideoProfile.STATE_BIDIRECTIONAL
        }
        conn.setRinging()
        if (callId.isNotEmpty()) {
            active[callId] = conn
        }
        return conn
    }

    override fun onCreateIncomingConnectionFailed(
        connectionManagerPhoneAccount: PhoneAccountHandle?,
        request: ConnectionRequest?
    ) {
        Log.w(TAG, "onCreateIncomingConnectionFailed — falling back to notification UI")
        val extras = callExtras(request)
        val callId = extras.getString(TelecomIncomingCall.EXTRA_CALL_ID)
        val event = callId?.let { IncomingCallRegistry.get(it) }
        if (event != null) {
            IncomingCallNotifier.showNotificationOnly(applicationContext, event)
        }
    }

    private fun callExtras(request: ConnectionRequest?): Bundle {
        val extras = request?.extras ?: return Bundle()
        val nested = extras.getBundle(TelecomManager.EXTRA_INCOMING_CALL_EXTRAS)
        return nested ?: extras
    }

    companion object {
        private const val TAG = "MglConnectionService"
        private val active = linkedMapOf<String, MglConnection>()

        fun endCall(callId: String?) {
            if (callId.isNullOrEmpty()) {
                val all = active.values.toList()
                active.clear()
                for (c in all) {
                    c.setDisconnected(DisconnectCause(DisconnectCause.REMOTE))
                    c.destroy()
                }
                return
            }
            val c = active.remove(callId) ?: return
            c.setDisconnected(DisconnectCause(DisconnectCause.REMOTE))
            c.destroy()
        }

        fun remove(callId: String) {
            active.remove(callId)
        }
    }
}

class MglConnection(
    private val callId: String,
    private val eventId: String,
    private val payload: Map<String, String>,
    private val provider: String?
) : Connection() {

    private var answered = false

    override fun onAnswer() {
        answered = true
        setActive()
        emitAction("accepted")
    }

    override fun onAnswer(videoState: Int) {
        onAnswer()
    }

    override fun onReject() {
        emitAction("rejected")
        setDisconnected(DisconnectCause(DisconnectCause.REJECTED))
        destroy()
        MglConnectionService.remove(callId)
        IncomingCallRegistry.remove(callId)
    }

    override fun onDisconnect() {
        if (!answered) {
            emitAction("rejected")
        } else {
            emitEnded("user-ended")
        }
        setDisconnected(DisconnectCause(DisconnectCause.LOCAL))
        destroy()
        MglConnectionService.remove(callId)
        IncomingCallRegistry.remove(callId)
    }

    override fun onAbort() {
        onReject()
    }

    private fun emitAction(action: String) {
        val ctx = MglConnectionServiceAppContext.getOrNull() ?: return
        IncomingCallActionBridge.emitAction(
            ctx,
            callId = callId,
            eventId = eventId,
            action = action,
            payload = payload,
            provider = provider
        )
    }

    private fun emitEnded(reason: String) {
        val ctx = MglConnectionServiceAppContext.getOrNull() ?: return
        val data = payload.toMutableMap()
        data["call_id"] = callId
        data["reason"] = reason
        val id = if (eventId.isNotEmpty()) "$eventId:ended" else "$callId:ended"
        val event = mapOf(
            "version" to 1,
            "type" to "call-ended",
            "id" to id,
            "timestamp" to (System.currentTimeMillis() / 1000),
            "provider" to (provider ?: "android"),
            "data" to data
        )
        val p = IncomingCallActionBridge.plugin
        if (p != null) {
            p.emitFromNative(event)
        } else {
            PendingNativeStore.save(ctx, event)
        }
    }
}

object MglConnectionServiceAppContext {
    @Volatile
    private var appContext: android.content.Context? = null

    fun set(ctx: android.content.Context) {
        appContext = ctx.applicationContext
    }

    fun getOrNull(): android.content.Context? = appContext
}
