package net.amjil.mglpush

import com.google.firebase.messaging.FirebaseMessagingService
import com.google.firebase.messaging.RemoteMessage

/**
 * Receives FCM data/notification while app is in foreground/background.
 * System tray display for notification messages when app is killed is handled by OS.
 */
class MglFirebaseMessagingService : FirebaseMessagingService() {

    override fun onNewToken(token: String) {
        FcmBridge.onNewToken(token)
    }

    override fun onMessageReceived(message: RemoteMessage) {
        val data = mutableMapOf<String, String>()
        message.data.forEach { (k, v) -> data[k] = v }

        val n = message.notification
        FcmBridge.onMessage(
            applicationContext,
            ProviderMessage(
                messageId = data["mgl_message_id"]
                    ?: data["mgl_event_id"]
                    ?: message.messageId
                    ?: data["google.message_id"],
                title = n?.title ?: data["title"],
                body = n?.body ?: data["body"],
                data = data,
                deepLink = data["deep_link"]
            )
        )
    }
}
