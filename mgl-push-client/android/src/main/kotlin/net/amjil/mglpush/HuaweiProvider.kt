package net.amjil.mglpush

import android.content.Context
import android.text.TextUtils
import android.util.Log
import com.huawei.agconnect.config.AGConnectServicesConfig
import com.huawei.hms.aaid.HmsInstanceId
import com.huawei.hms.api.ConnectionResult
import com.huawei.hms.api.HuaweiApiAvailability
import com.huawei.hms.push.HmsMessaging

/**
 * Huawei Push Kit provider (Phase 4).
 * Host app must ship agconnect-services.json and apply AGConnect/Huawei plugins.
 */
class HuaweiProvider(private val context: Context) : PushProvider {
    private var callback: ProviderCallback? = null
    @Volatile private var token: String? = null

    override fun name(): String = "huawei"

    override fun isAvailable(): Boolean {
        return try {
            val status = HuaweiApiAvailability.getInstance()
                .isHuaweiMobileServicesAvailable(context)
            if (status != ConnectionResult.SUCCESS) {
                return false
            }
            // agconnect-services.json present?
            val appId = readAppId()
            !appId.isNullOrBlank()
        } catch (_: Throwable) {
            false
        }
    }

    override fun initialize(callback: ProviderCallback) {
        this.callback = callback
        HuaweiBridge.register(this)
        try {
            HmsMessaging.getInstance(context).isAutoInitEnabled = true
            Thread {
                try {
                    val appId = readAppId()
                    if (appId.isNullOrBlank()) {
                        callback.onError("INIT_ERROR", "Huawei app_id missing (agconnect-services.json)")
                        return@Thread
                    }
                    val t = HmsInstanceId.getInstance(context).getToken(appId, "HCM")
                    if (!TextUtils.isEmpty(t)) {
                        token = t
                        callback.onTokenChanged(t)
                    }
                } catch (e: Exception) {
                    callback.onError("TOKEN_ERROR", e.message ?: "failed to get Huawei token")
                }
            }.start()
        } catch (e: Exception) {
            callback.onError("INIT_ERROR", e.message ?: "Huawei initialize failed")
        }
    }

    override fun getToken(): String? = token

    override fun unregister() {
        Thread {
            try {
                val appId = readAppId()
                if (!appId.isNullOrBlank()) {
                    HmsInstanceId.getInstance(context).deleteToken(appId, "HCM")
                }
            } catch (e: Exception) {
                Log.w(TAG, "deleteToken failed: ${e.message}")
            }
        }.start()
        token = null
        HuaweiBridge.unregister(this)
    }

    override fun setAlias(alias: String?) {
        // Alias/topic managed via HMS profile APIs in later iterations if needed.
    }

    override fun subscribeTopic(topic: String) {
        HmsMessaging.getInstance(context).subscribe(topic)
            .addOnFailureListener { e ->
                callback?.onError("TOPIC_ERROR", e.message ?: "subscribe failed")
            }
    }

    override fun unsubscribeTopic(topic: String) {
        HmsMessaging.getInstance(context).unsubscribe(topic)
            .addOnFailureListener { e ->
                callback?.onError("TOPIC_ERROR", e.message ?: "unsubscribe failed")
            }
    }

    internal fun onNewToken(newToken: String) {
        token = newToken
        callback?.onTokenChanged(newToken)
    }

    internal fun onMessage(message: ProviderMessage) {
        callback?.onMessage(message)
    }

    private fun readAppId(): String? {
        return try {
            AGConnectServicesConfig.fromContext(context).getString("client/app_id")
        } catch (_: Throwable) {
            null
        }
    }

    companion object {
        private const val TAG = "HuaweiProvider"
    }
}

object HuaweiBridge {
    @Volatile private var provider: HuaweiProvider? = null

    fun register(p: HuaweiProvider) {
        provider = p
    }

    fun unregister(p: HuaweiProvider) {
        if (provider === p) provider = null
    }

    fun onNewToken(token: String) {
        provider?.onNewToken(token)
    }

    fun onMessage(message: ProviderMessage) {
        provider?.onMessage(message)
    }
}
