package net.amjil.mglpush

import android.content.ComponentName
import android.content.Context
import android.net.Uri
import android.os.Build
import android.os.Bundle
import android.telecom.Connection
import android.telecom.DisconnectCause
import android.telecom.PhoneAccount
import android.telecom.PhoneAccountHandle
import android.telecom.TelecomManager
import android.util.Log

/**
 * Self-managed Telecom incoming-call path (Android Call Control).
 * Falls back to [IncomingCallNotifier] when Telecom is unavailable.
 */
object TelecomIncomingCall {
    private const val TAG = "MglTelecom"
    private const val ACCOUNT_ID = "mgl_push_self_managed"
    const val EXTRA_CALL_ID = "mgl_call_id"
    const val EXTRA_EVENT_ID = "mgl_event_id"
    const val EXTRA_CALLER_NAME = "mgl_caller_name"
    const val EXTRA_MEDIA_TYPE = "mgl_media_type"
    const val EXTRA_PAYLOAD = "mgl_payload_json"
    const val EXTRA_PROVIDER = "mgl_provider"
    const val EXTRA_EVENT = "mgl_event_bundle"

    @Volatile
    private var accountRegistered = false

    fun isAvailable(context: Context): Boolean {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.O) return false
        return try {
            context.getSystemService(TelecomManager::class.java) != null
        } catch (_: Exception) {
            false
        }
    }

    fun ensurePhoneAccount(context: Context): PhoneAccountHandle? {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.O) return null
        val tm = try {
            context.getSystemService(TelecomManager::class.java)
        } catch (_: Exception) {
            null
        } ?: return null

        val handle = PhoneAccountHandle(
            ComponentName(context, MglConnectionService::class.java),
            ACCOUNT_ID
        )
        try {
            val existing = tm.getPhoneAccount(handle)
            if (existing == null || !accountRegistered) {
                val label = context.applicationInfo.loadLabel(context.packageManager)
                val account = PhoneAccount.builder(handle, label)
                    .setCapabilities(PhoneAccount.CAPABILITY_SELF_MANAGED)
                    .setSupportedUriScheme(PhoneAccount.SCHEME_SIP)
                    .build()
                tm.registerPhoneAccount(account)
                accountRegistered = true
            }
            return handle
        } catch (e: SecurityException) {
            Log.w(TAG, "registerPhoneAccount denied", e)
            return null
        } catch (e: Exception) {
            Log.w(TAG, "registerPhoneAccount failed", e)
            return null
        }
    }

    /**
     * @return true when Telecom accepted the incoming call request.
     */
    fun show(context: Context, event: Map<String, Any?>): Boolean {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.O) return false
        val tm = try {
            context.getSystemService(TelecomManager::class.java)
        } catch (_: Exception) {
            null
        } ?: return false

        val handle = ensurePhoneAccount(context) ?: return false
        val data = stringData(event)
        val callId = data["call_id"] ?: data["callId"] ?: return false
        val eventId = (event["id"] as? String).orEmpty()
        val caller = data["caller_display_name"]
            ?: data["caller_id"]
            ?: data["callerId"]
            ?: "Incoming Call"
        val media = data["media_type"] ?: data["mediaType"] ?: "audio"
        val payloadJson = mapToJsonString(data)
        val provider = event["provider"] as? String

        val extras = Bundle().apply {
            putParcelable(TelecomManager.EXTRA_PHONE_ACCOUNT_HANDLE, handle)
            putBundle(
                TelecomManager.EXTRA_INCOMING_CALL_EXTRAS,
                Bundle().apply {
                    putString(EXTRA_CALL_ID, callId)
                    putString(EXTRA_EVENT_ID, eventId)
                    putString(EXTRA_CALLER_NAME, caller)
                    putString(EXTRA_MEDIA_TYPE, media)
                    putString(EXTRA_PAYLOAD, payloadJson)
                    putString(EXTRA_PROVIDER, provider)
                }
            )
        }

        return try {
            tm.addNewIncomingCall(handle, extras)
            IncomingCallRegistry.put(callId, event)
            true
        } catch (e: SecurityException) {
            Log.w(TAG, "addNewIncomingCall denied", e)
            false
        } catch (e: Exception) {
            Log.w(TAG, "addNewIncomingCall failed", e)
            false
        }
    }

    fun end(context: Context, callId: String?) {
        MglConnectionService.endCall(callId)
        if (callId.isNullOrEmpty()) {
            IncomingCallRegistry.clear()
        } else {
            IncomingCallRegistry.remove(callId)
        }
    }

    fun addressFor(callId: String): Uri =
        Uri.fromParts(PhoneAccount.SCHEME_SIP, "mgl-$callId", null)

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
