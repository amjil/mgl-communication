package net.amjil.mglpush

import android.Manifest
import android.app.Activity
import android.content.Context
import android.content.pm.PackageManager
import android.os.Build
import androidx.core.app.ActivityCompat
import androidx.core.content.ContextCompat
import com.google.android.gms.common.ConnectionResult
import com.google.android.gms.common.GoogleApiAvailability
import com.google.firebase.FirebaseApp
import com.google.firebase.messaging.FirebaseMessaging

/**
 * FCM provider (Phase 2).
 * Host app must ship google-services.json and apply the Google Services plugin.
 */
class FcmProvider(private val context: Context) : PushProvider {
    private var callback: ProviderCallback? = null
    @Volatile private var token: String? = null

    override fun name(): String = "fcm"

    override fun isAvailable(): Boolean {
        return try {
            if (FirebaseApp.getApps(context).isEmpty()) {
                return false
            }
            val gms = GoogleApiAvailability.getInstance()
                .isGooglePlayServicesAvailable(context)
            gms == ConnectionResult.SUCCESS
        } catch (_: Exception) {
            false
        }
    }

    override fun initialize(callback: ProviderCallback) {
        this.callback = callback
        FcmBridge.register(this)
        try {
            FirebaseMessaging.getInstance().token
                .addOnSuccessListener { t ->
                    if (!t.isNullOrBlank()) {
                        token = t
                        callback.onTokenChanged(t)
                    }
                }
                .addOnFailureListener { e ->
                    callback.onError("TOKEN_ERROR", e.message ?: "failed to get FCM token")
                }

            FirebaseMessaging.getInstance().isAutoInitEnabled = true
        } catch (e: Exception) {
            callback.onError("INIT_ERROR", e.message ?: "FCM initialize failed")
        }
    }

    override fun getToken(): String? = token

    override fun unregister() {
        try {
            FirebaseMessaging.getInstance().deleteToken()
        } catch (_: Exception) {
        }
        token = null
        FcmBridge.unregister(this)
    }

    override fun setAlias(alias: String?) {
        // FCM has no alias; user binding is server-side via installation_id.
    }

    override fun subscribeTopic(topic: String) {
        FirebaseMessaging.getInstance().subscribeToTopic(topic)
    }

    override fun unsubscribeTopic(topic: String) {
        FirebaseMessaging.getInstance().unsubscribeFromTopic(topic)
    }

    internal fun onNewToken(newToken: String) {
        token = newToken
        callback?.onTokenChanged(newToken)
    }

    internal fun onMessage(message: ProviderMessage) {
        callback?.onMessage(message)
    }

    fun requestPermission(activity: Activity?, onResult: (Boolean) -> Unit) {
        if (Build.VERSION.SDK_INT < 33) {
            onResult(true)
            return
        }
        if (activity == null) {
            onResult(false)
            return
        }
        val granted = ContextCompat.checkSelfPermission(
            activity, Manifest.permission.POST_NOTIFICATIONS
        ) == PackageManager.PERMISSION_GRANTED
        if (granted) {
            onResult(true)
            return
        }
        ActivityCompat.requestPermissions(
            activity,
            arrayOf(Manifest.permission.POST_NOTIFICATIONS),
            REQUEST_POST_NOTIFICATIONS
        )
        // Result delivered asynchronously via activity; optimistic true if we requested.
        onResult(true)
    }

    companion object {
        const val REQUEST_POST_NOTIFICATIONS = 0x4D47 // 'MG'
    }
}

/** Forwards FCM service callbacks to the active provider instance. */
object FcmBridge {
    @Volatile private var provider: FcmProvider? = null

    fun register(p: FcmProvider) {
        provider = p
    }

    fun unregister(p: FcmProvider) {
        if (provider === p) provider = null
    }

    fun onNewToken(token: String) {
        provider?.onNewToken(token)
    }

    fun onMessage(message: ProviderMessage) {
        provider?.onMessage(message)
    }
}
