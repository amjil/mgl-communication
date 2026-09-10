package net.amjil.mglpush

/**
 * Example vivo OpenClientPushMessageReceiver wiring for host apps.
 *
 * ```kotlin
 * class AppVivoReceiver : OpenClientPushMessageReceiver() {
 *   override fun onReceiveRegId(context: Context, regId: String) {
 *     VivoBridge.onRegister(regId)
 *   }
 *   override fun onNotificationMessageClicked(context: Context, msg: UPSNotificationMessage) {
 *     VivoBridge.onNotificationOpened(msg.params ?: emptyMap())
 *   }
 * }
 * ```
 */
@Suppress("unused")
object MglVivoPushDocs
