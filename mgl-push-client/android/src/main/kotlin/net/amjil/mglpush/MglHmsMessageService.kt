package net.amjil.mglpush

import com.huawei.hms.push.HmsMessageService
import com.huawei.hms.push.RemoteMessage

/**
 * Receives Huawei Push Kit messages and token refresh callbacks.
 * Persists to [PendingNativeStore] when Flutter is not yet running.
 */
class MglHmsMessageService : HmsMessageService() {

    override fun onNewToken(token: String?) {
        if (!token.isNullOrBlank()) {
            HuaweiBridge.onNewToken(token)
        }
    }

    override fun onMessageReceived(message: RemoteMessage?) {
        if (message == null) return
        val data = mutableMapOf<String, String>()
        message.dataOfMap?.forEach { (k, v) -> data[k] = v }

        val n = message.notification
        HuaweiBridge.onMessage(
            applicationContext,
            ProviderMessage(
                messageId = data["mgl_message_id"] ?: data["mgl_event_id"] ?: message.messageId,
                title = n?.title ?: data["title"],
                body = n?.body ?: data["body"],
                data = data,
                deepLink = data["deep_link"] ?: n?.intentUri
            )
        )
    }
}
