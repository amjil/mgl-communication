package net.amjil.mglpush

/**
 * Example MiPush PushMessageReceiver for host apps.
 *
 * Copy into your app module (must extend com.xiaomi.mipush.sdk.PushMessageReceiver),
 * register in AndroidManifest, and keep this forwarding logic.
 *
 * This file is documentation-only and is NOT compiled into the plugin
 * (SDK JAR is provided by the host app).
 *
 * ```kotlin
 * package your.app
 *
 * import android.content.Context
 * import com.xiaomi.mipush.sdk.*
 * import net.amjil.mglpush.XiaomiBridge
 *
 * class AppMiPushReceiver : PushMessageReceiver() {
 *   override fun onReceiveRegisterResult(context: Context, message: MiPushCommandMessage) {
 *     if (message.command == MiPushClient.COMMAND_REGISTER && message.resultCode == ErrorCode.SUCCESS.toLong()) {
 *       XiaomiBridge.onRegister(message.commandArguments?.firstOrNull())
 *     }
 *   }
 *   override fun onReceivePassThroughMessage(context: Context, message: MiPushMessage) {
 *     XiaomiBridge.onPassThrough(message.title, message.content, message.extra ?: emptyMap())
 *   }
 *   override fun onNotificationMessageClicked(context: Context, message: MiPushMessage) {
 *     XiaomiBridge.onNotificationClicked(message.title, message.description, message.extra ?: emptyMap())
 *   }
 *   override fun onNotificationMessageArrived(context: Context, message: MiPushMessage) {
 *     XiaomiBridge.onNotificationArrived(message.title, message.description, message.extra ?: emptyMap())
 *   }
 * }
 * ```
 */
@Suppress("unused")
object MglMiPushReceiverDocs
