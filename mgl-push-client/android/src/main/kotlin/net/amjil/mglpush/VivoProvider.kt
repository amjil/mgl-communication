package net.amjil.mglpush

import android.content.Context
import android.content.pm.ApplicationInfo
import android.content.pm.PackageManager
import android.util.Log
import java.lang.reflect.Proxy

/**
 * vivo Push provider (Phase 3).
 *
 * Host app must add vivo Push SDK and meta-data:
 * - VIVO_APP_ID
 * - VIVO_APP_KEY
 *
 * Prefer also setting com.vivo.push.api_key / com.vivo.push.app_id per vivo docs.
 */
class VivoProvider(private val context: Context) : PushProvider {
    private var callback: ProviderCallback? = null
    @Volatile private var token: String? = null

    override fun name(): String = "vivo"

    override fun isAvailable(): Boolean {
        return try {
            Class.forName(PUSH_CLIENT)
            val appId = meta(META_APP_ID) ?: meta("com.vivo.push.app_id")
            val appKey = meta(META_APP_KEY) ?: meta("com.vivo.push.api_key")
            !appId.isNullOrBlank() && !appKey.isNullOrBlank()
        } catch (_: Throwable) {
            false
        }
    }

    override fun initialize(callback: ProviderCallback) {
        this.callback = callback
        VivoBridge.register(this, context)
        try {
            val clientClass = Class.forName(PUSH_CLIENT)
            val getInstance = clientClass.getMethod("getInstance", Context::class.java)
            val client = getInstance.invoke(null, context.applicationContext)

            // initialize() if present
            try {
                clientClass.getMethod("initialize").invoke(client)
            } catch (_: Exception) {
            }

            // turnOnPush / bind listener variants
            val listenerIface = try {
                Class.forName(ACTION_LISTENER)
            } catch (_: Exception) {
                null
            }

            if (listenerIface != null) {
                val proxy = Proxy.newProxyInstance(
                    listenerIface.classLoader,
                    arrayOf(listenerIface)
                ) { _, method, args ->
                    when (method.name) {
                        "onStateChanged" -> {
                            // onStateChanged(int state)
                            val state = args?.getOrNull(0) as? Int
                            if (state != null && state != 0) {
                                VivoBridge.onError("TOKEN_ERROR", "vivo state=$state")
                            }
                        }
                    }
                    null
                }
                for (m in clientClass.methods) {
                    if (m.name == "turnOnPush" && m.parameterTypes.size == 1) {
                        try {
                            m.invoke(client, proxy)
                        } catch (e: Exception) {
                            Log.w(TAG, "turnOnPush: ${e.message}")
                        }
                    }
                }
            } else {
                try {
                    clientClass.getMethod("turnOnPush").invoke(client)
                } catch (_: Exception) {
                }
            }

            // getRegId
            tryGetRegId(clientClass, client)?.let {
                if (it.isNotBlank()) {
                    token = it
                    callback.onTokenChanged(it)
                }
            }
        } catch (e: Exception) {
            callback.onError("INIT_ERROR", e.message ?: "vivo initialize failed")
        }
    }

    override fun getToken(): String? = token

    override fun unregister() {
        try {
            val clientClass = Class.forName(PUSH_CLIENT)
            val client = clientClass.getMethod("getInstance", Context::class.java)
                .invoke(null, context.applicationContext)
            try {
                clientClass.getMethod("turnOffPush").invoke(client)
            } catch (_: Exception) {
                for (m in clientClass.methods) {
                    if (m.name == "turnOffPush") {
                        m.invoke(client, *arrayOfNulls(m.parameterTypes.size))
                        break
                    }
                }
            }
        } catch (e: Exception) {
            Log.w(TAG, "turnOffPush: ${e.message}")
        }
        token = null
        VivoBridge.unregister(this)
    }

    override fun setAlias(alias: String?) {
        if (alias.isNullOrBlank()) return
        try {
            val clientClass = Class.forName(PUSH_CLIENT)
            val client = clientClass.getMethod("getInstance", Context::class.java)
                .invoke(null, context.applicationContext)
            for (m in clientClass.methods) {
                if (m.name == "bindAlias" && m.parameterTypes.isNotEmpty()) {
                    when (m.parameterTypes.size) {
                        1 -> m.invoke(client, alias)
                        2 -> m.invoke(client, alias, null)
                    }
                    break
                }
            }
        } catch (e: Exception) {
            Log.w(TAG, "bindAlias: ${e.message}")
        }
    }

    override fun subscribeTopic(topic: String) {
        // vivo tags API — optional
    }

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

    private fun tryGetRegId(clientClass: Class<*>, client: Any?): String? {
        return try {
            val m = clientClass.methods.firstOrNull {
                it.name.equals("getRegId", true) && it.parameterTypes.isEmpty()
            } ?: return null
            m.invoke(client) as? String
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
        private const val TAG = "VivoProvider"
        private const val PUSH_CLIENT = "com.vivo.push.PushClient"
        private const val ACTION_LISTENER = "com.vivo.push.IPushActionListener"
        const val META_APP_ID = "VIVO_APP_ID"
        const val META_APP_KEY = "VIVO_APP_KEY"
    }
}

object VivoBridge {
    @Volatile private var provider: VivoProvider? = null
    @Volatile private var appContext: android.content.Context? = null

    fun register(p: VivoProvider) {
        provider = p
    }

    fun register(p: VivoProvider, context: android.content.Context) {
        provider = p
        appContext = context.applicationContext
    }

    fun unregister(p: VivoProvider) {
        if (provider === p) provider = null
    }

    fun onRegister(regId: String?) {
        if (!regId.isNullOrBlank()) provider?.onNewToken(regId)
    }

    fun onMessage(title: String?, content: String?, extra: Map<String, String>) {
        onMessage(appContext, title, content, extra)
    }

    fun onMessage(
        context: android.content.Context?,
        title: String?,
        content: String?,
        extra: Map<String, String>
    ) {
        if (context != null) appContext = context.applicationContext
        val message = ProviderMessage(
            messageId = extra["mgl_message_id"] ?: extra["mgl_event_id"],
            title = title,
            body = content,
            data = extra,
            deepLink = extra["deep_link"]
        )
        val p = provider
        if (p != null) {
            p.onMessage(message)
        } else {
            PendingNativeStore.saveProviderMessage(appContext, "vivo", message)
        }
    }

    fun onNotificationOpened(extra: Map<String, String>) {
        provider?.onNotificationOpened(extra)
    }

    fun onError(code: String, message: String) {
        provider?.onError(code, message)
    }
}
