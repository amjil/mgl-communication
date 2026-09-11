package net.amjil.mglpush

import android.app.Activity
import android.app.Application
import android.content.Context
import android.content.pm.ApplicationInfo
import android.content.pm.PackageManager
import android.util.Log
import java.lang.reflect.Proxy

/**
 * OPPO / HeyTap Push provider (Phase 3).
 *
 * Host app must add HeyTap/OPPO Push SDK and meta-data:
 * - OPPO_APP_KEY
 * - OPPO_APP_SECRET
 *
 * Notification channel (Android 8+) is configured by the host app;
 * server sends channel_id (default mgl_default).
 */
class OppoProvider(private val context: Context) : PushProvider {
    private var callback: ProviderCallback? = null
    @Volatile private var token: String? = null

    override fun name(): String = "oppo"

    override fun isAvailable(): Boolean {
        return try {
            Class.forName(HEYTAP_MANAGER)
            val key = meta(META_APP_KEY)
            val secret = meta(META_APP_SECRET)
            !key.isNullOrBlank() && !secret.isNullOrBlank()
        } catch (_: Throwable) {
            false
        }
    }

    override fun initialize(callback: ProviderCallback) {
        this.callback = callback
        OppoBridge.register(this, context)
        try {
            val appKey = meta(META_APP_KEY)
            val appSecret = meta(META_APP_SECRET)
            if (appKey.isNullOrBlank() || appSecret.isNullOrBlank()) {
                callback.onError("INIT_ERROR", "OPPO_APP_KEY / OPPO_APP_SECRET meta-data required")
                return
            }

            val manager = Class.forName(HEYTAP_MANAGER)
            // init(Context, boolean needLog?) — variants differ by SDK version
            tryInit(manager, context.applicationContext)

            // register(Activity/Application, appKey, appSecret, callback)
            val cbIface = Class.forName(CALLBACK_IFACE)
            val proxy = Proxy.newProxyInstance(
                cbIface.classLoader,
                arrayOf(cbIface)
            ) { _, method, args ->
                when (method.name) {
                    "onRegister" -> {
                        // onRegister(int code, String registerId)
                        if (args != null && args.size >= 2) {
                            val code = args[0] as? Int ?: -1
                            val regId = args[1] as? String
                            if (code == 0 && !regId.isNullOrBlank()) {
                                OppoBridge.onRegister(regId)
                            } else {
                                OppoBridge.onError("TOKEN_ERROR", "oppo register code=$code")
                            }
                        }
                    }
                    "onUnRegister" -> {}
                    "onSetPushTime" -> {}
                    "onGetPushStatus" -> {}
                    "onGetNotificationStatus" -> {}
                    "onError" -> {
                        val code = args?.getOrNull(0)
                        val msg = args?.getOrNull(1)?.toString() ?: "oppo error"
                        OppoBridge.onError("OPPO_ERROR", "code=$code $msg")
                    }
                }
                null
            }

            val app = context.applicationContext
            var registered = false
            for (m in manager.methods) {
                if (m.name != "register") continue
                val params = m.parameterTypes
                try {
                    when {
                        params.size == 4 &&
                            Application::class.java.isAssignableFrom(params[0]) -> {
                            m.invoke(null, app as Application, appKey, appSecret, proxy)
                            registered = true
                            break
                        }
                        params.size == 4 &&
                            Context::class.java.isAssignableFrom(params[0]) -> {
                            m.invoke(null, app, appKey, appSecret, proxy)
                            registered = true
                            break
                        }
                        params.size == 3 -> {
                            m.invoke(null, app, appKey, appSecret)
                            registered = true
                            break
                        }
                    }
                } catch (e: Exception) {
                    Log.w(TAG, "register variant failed: ${e.message}")
                }
            }
            if (!registered) {
                callback.onError("INIT_ERROR", "HeytapPushManager.register not found")
            }

            // Try immediate getRegisterID
            tryGetRegisterID(manager)?.let {
                if (it.isNotBlank()) {
                    token = it
                    callback.onTokenChanged(it)
                }
            }
        } catch (e: Exception) {
            callback.onError("INIT_ERROR", e.message ?: "OPPO initialize failed")
        }
    }

    override fun getToken(): String? = token

    override fun unregister() {
        try {
            val manager = Class.forName(HEYTAP_MANAGER)
            for (m in manager.methods) {
                if (m.name == "unRegister" && m.parameterTypes.size <= 1) {
                    if (m.parameterTypes.isEmpty()) m.invoke(null)
                    else m.invoke(null, context.applicationContext)
                    break
                }
            }
        } catch (e: Exception) {
            Log.w(TAG, "unRegister: ${e.message}")
        }
        token = null
        OppoBridge.unregister(this)
    }

    override fun setAlias(alias: String?) {
        // OPPO alias APIs vary; binding is primarily server-side via installation_id.
    }

    override fun subscribeTopic(topic: String) {}
    override fun unsubscribeTopic(topic: String) {}

    internal fun onNewToken(regId: String) {
        token = regId
        callback?.onTokenChanged(regId)
    }

    internal fun onMessage(message: ProviderMessage) {
        callback?.onMessage(message)
    }

    internal fun onNotificationOpened(data: Map<String, String>) {
        callback?.onNotificationOpened(data)
    }

    internal fun onError(code: String, message: String) {
        callback?.onError(code, message)
    }

    private fun tryInit(manager: Class<*>, appContext: Context) {
        for (m in manager.methods) {
            if (m.name != "init") continue
            try {
                when (m.parameterTypes.size) {
                    1 -> m.invoke(null, appContext)
                    2 -> m.invoke(null, appContext, false)
                }
            } catch (_: Exception) {
            }
        }
    }

    private fun tryGetRegisterID(manager: Class<*>): String? {
        return try {
            val m = manager.methods.firstOrNull {
                it.name.equals("getRegisterID", true) || it.name.equals("getRegistrationId", true)
            } ?: return null
            when (m.parameterTypes.size) {
                0 -> m.invoke(null) as? String
                1 -> m.invoke(null, context.applicationContext) as? String
                else -> null
            }
        } catch (_: Exception) {
            null
        }
    }

    private fun meta(key: String): String? {
        return try {
            val ai: ApplicationInfo = context.packageManager.getApplicationInfo(
                context.packageName, PackageManager.GET_META_DATA
            )
            ai.metaData?.getString(key) ?: ai.metaData?.get(key)?.toString()
        } catch (_: Exception) {
            null
        }
    }

    companion object {
        private const val TAG = "OppoProvider"
        private const val HEYTAP_MANAGER = "com.heytap.msp.push.HeytapPushManager"
        private const val CALLBACK_IFACE = "com.heytap.msp.push.callback.ICallBackResultService"
        const val META_APP_KEY = "OPPO_APP_KEY"
        const val META_APP_SECRET = "OPPO_APP_SECRET"
    }
}

object OppoBridge {
    @Volatile private var provider: OppoProvider? = null
    @Volatile private var appContext: android.content.Context? = null

    fun register(p: OppoProvider) {
        provider = p
    }

    fun register(p: OppoProvider, context: android.content.Context) {
        provider = p
        appContext = context.applicationContext
    }

    fun unregister(p: OppoProvider) {
        if (provider === p) provider = null
    }

    fun onRegister(regId: String?) {
        if (!regId.isNullOrBlank()) provider?.onNewToken(regId)
    }

    fun onMessage(title: String?, body: String?, extra: Map<String, String>) {
        onMessage(appContext, title, body, extra)
    }

    fun onMessage(
        context: android.content.Context?,
        title: String?,
        body: String?,
        extra: Map<String, String>
    ) {
        if (context != null) appContext = context.applicationContext
        val message = ProviderMessage(
            messageId = extra["mgl_message_id"] ?: extra["mgl_event_id"],
            title = title,
            body = body,
            data = extra,
            deepLink = extra["deep_link"]
        )
        val p = provider
        if (p != null) {
            p.onMessage(message)
        } else {
            PendingNativeStore.saveProviderMessage(appContext, "oppo", message)
        }
    }

    fun onNotificationOpened(extra: Map<String, String>) {
        provider?.onNotificationOpened(extra)
    }

    fun onError(code: String, message: String) {
        provider?.onError(code, message)
    }
}

/** Optional helper if host obtains Activity for register variants. */
fun OppoProvider.registerWithActivity(activity: Activity) {
    // Reserved for SDK variants requiring Activity; initialize() covers Application path.
    Log.d("OppoProvider", "activity=${activity.localClassName}")
}
