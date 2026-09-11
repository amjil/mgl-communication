package net.amjil.mglpush

import android.content.Context
import android.content.pm.ApplicationInfo
import android.content.pm.PackageManager
import android.util.Log
import java.lang.reflect.Method

/**
 * Xiaomi MiPush provider (Phase 2).
 *
 * Host app must:
 * 1. Add MiPush Android SDK (AAR/JAR from Xiaomi console) to the app
 * 2. Declare meta-data XIAOMI_APP_ID / XIAOMI_APP_KEY
 * 3. Register a PushMessageReceiver that forwards to [XiaomiBridge]
 *
 * See docs/CLIENT.md and android/examples/AppMiPushReceiver.kt
 */
class XiaomiProvider(private val context: Context) : PushProvider {
    private var callback: ProviderCallback? = null
    @Volatile private var token: String? = null

    override fun name(): String = "xiaomi"

    override fun isAvailable(): Boolean {
        return try {
            Class.forName(MI_PUSH_CLIENT)
            val appId = meta(META_APP_ID)
            val appKey = meta(META_APP_KEY)
            !appId.isNullOrBlank() && !appKey.isNullOrBlank()
        } catch (_: Throwable) {
            false
        }
    }

    override fun initialize(callback: ProviderCallback) {
        this.callback = callback
        XiaomiBridge.register(this, context)
        try {
            val appId = meta(META_APP_ID)
            val appKey = meta(META_APP_KEY)
            if (appId.isNullOrBlank() || appKey.isNullOrBlank()) {
                callback.onError("INIT_ERROR", "XIAOMI_APP_ID / XIAOMI_APP_KEY meta-data required")
                return
            }
            val clazz = Class.forName(MI_PUSH_CLIENT)
            val register: Method = clazz.getMethod(
                "registerPush",
                Context::class.java,
                String::class.java,
                String::class.java
            )
            register.invoke(null, context.applicationContext, appId, appKey)
        } catch (e: Exception) {
            callback.onError("INIT_ERROR", e.message ?: "Xiaomi initialize failed")
        }
    }

    override fun getToken(): String? = token

    override fun unregister() {
        try {
            val clazz = Class.forName(MI_PUSH_CLIENT)
            val unregister = clazz.getMethod("unregisterPush", Context::class.java)
            unregister.invoke(null, context.applicationContext)
        } catch (e: Exception) {
            Log.w(TAG, "unregisterPush: ${e.message}")
        }
        token = null
        XiaomiBridge.unregister(this)
    }

    override fun setAlias(alias: String?) {
        try {
            val clazz = Class.forName(MI_PUSH_CLIENT)
            if (alias.isNullOrBlank()) {
                // clear aliases not always available; ignore
                return
            }
            val m = clazz.getMethod("setAlias", Context::class.java, String::class.java, String::class.java)
            m.invoke(null, context.applicationContext, alias, null)
        } catch (e: Exception) {
            Log.w(TAG, "setAlias: ${e.message}")
        }
    }

    override fun subscribeTopic(topic: String) {
        try {
            val clazz = Class.forName(MI_PUSH_CLIENT)
            val m = clazz.getMethod("subscribe", Context::class.java, String::class.java, String::class.java)
            m.invoke(null, context.applicationContext, topic, null)
        } catch (e: Exception) {
            callback?.onError("TOPIC_ERROR", e.message ?: "subscribe failed")
        }
    }

    override fun unsubscribeTopic(topic: String) {
        try {
            val clazz = Class.forName(MI_PUSH_CLIENT)
            val m = clazz.getMethod("unsubscribe", Context::class.java, String::class.java, String::class.java)
            m.invoke(null, context.applicationContext, topic, null)
        } catch (e: Exception) {
            callback?.onError("TOPIC_ERROR", e.message ?: "unsubscribe failed")
        }
    }

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

    private fun meta(key: String): String? {
        return try {
            val ai: ApplicationInfo = context.packageManager.getApplicationInfo(
                context.packageName, PackageManager.GET_META_DATA
            )
            ai.metaData?.getString(key)
                ?: ai.metaData?.get(key)?.toString()
        } catch (_: Exception) {
            null
        }
    }

    companion object {
        private const val TAG = "XiaomiProvider"
        private const val MI_PUSH_CLIENT = "com.xiaomi.mipush.sdk.MiPushClient"
        const val META_APP_ID = "XIAOMI_APP_ID"
        const val META_APP_KEY = "XIAOMI_APP_KEY"
    }
}

object XiaomiBridge {
    @Volatile private var provider: XiaomiProvider? = null
    @Volatile private var appContext: android.content.Context? = null

    fun register(p: XiaomiProvider) {
        provider = p
    }

    fun register(p: XiaomiProvider, context: android.content.Context) {
        provider = p
        appContext = context.applicationContext
    }

    fun unregister(p: XiaomiProvider) {
        if (provider === p) provider = null
    }

    fun onRegister(regId: String?) {
        if (!regId.isNullOrBlank()) {
            provider?.onNewToken(regId)
        }
    }

    fun onPassThrough(title: String?, content: String?, extra: Map<String, String>) {
        deliver(
            ProviderMessage(
                messageId = extra["mgl_message_id"] ?: extra["mgl_event_id"],
                title = title,
                body = content,
                data = extra,
                deepLink = extra["deep_link"]
            )
        )
    }

    fun onPassThrough(
        context: android.content.Context?,
        title: String?,
        content: String?,
        extra: Map<String, String>
    ) {
        if (context != null) appContext = context.applicationContext
        onPassThrough(title, content, extra)
    }

    fun onNotificationClicked(title: String?, description: String?, extra: Map<String, String>) {
        val data = extra.toMutableMap()
        if (!title.isNullOrBlank()) data.putIfAbsent("title", title)
        if (!description.isNullOrBlank()) data.putIfAbsent("body", description)
        provider?.onNotificationOpened(data)
    }

    fun onNotificationArrived(title: String?, description: String?, extra: Map<String, String>) {
        deliver(
            ProviderMessage(
                messageId = extra["mgl_message_id"] ?: extra["mgl_event_id"],
                title = title,
                body = description,
                data = extra,
                deepLink = extra["deep_link"]
            )
        )
    }

    fun onNotificationArrived(
        context: android.content.Context?,
        title: String?,
        description: String?,
        extra: Map<String, String>
    ) {
        if (context != null) appContext = context.applicationContext
        onNotificationArrived(title, description, extra)
    }

    fun onError(code: String, message: String) {
        provider?.onError(code, message)
    }

    private fun deliver(message: ProviderMessage) {
        val p = provider
        if (p != null) {
            p.onMessage(message)
        } else {
            PendingNativeStore.saveProviderMessage(appContext, "xiaomi", message)
        }
    }
}
