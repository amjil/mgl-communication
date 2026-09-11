package your.app

import android.content.Context
import com.xiaomi.mipush.sdk.ErrorCode
import com.xiaomi.mipush.sdk.MiPushClient
import com.xiaomi.mipush.sdk.MiPushCommandMessage
import com.xiaomi.mipush.sdk.MiPushMessage
import com.xiaomi.mipush.sdk.PushMessageReceiver
import net.amjil.mglpush.XiaomiBridge

/**
 * Host-app MiPush receiver for mgl-push Phase 2.
 *
 * Copy into the host app module (SDK AAR is provided by the host), then register:
 *
 * ```xml
 * <receiver
 *     android:name=".AppMiPushReceiver"
 *     android:exported="true">
 *   <intent-filter>
 *     <action android:name="com.xiaomi.mipush.RECEIVE_MESSAGE" />
 *   </intent-filter>
 *   <intent-filter>
 *     <action android:name="com.xiaomi.mipush.MESSAGE_ARRIVED" />
 *   </intent-filter>
 *   <intent-filter>
 *     <action android:name="com.xiaomi.mipush.ERROR" />
 *   </intent-filter>
 * </receiver>
 * ```
 *
 * Cold-start: [XiaomiBridge] persists via PendingNativeStore when Flutter is not ready.
 */
class AppMiPushReceiver : PushMessageReceiver() {
    override fun onReceiveRegisterResult(context: Context, message: MiPushCommandMessage) {
        if (message.command == MiPushClient.COMMAND_REGISTER &&
            message.resultCode == ErrorCode.SUCCESS.toLong()
        ) {
            XiaomiBridge.onRegister(message.commandArguments?.firstOrNull())
        }
    }

    override fun onReceivePassThroughMessage(context: Context, message: MiPushMessage) {
        XiaomiBridge.onPassThrough(
            context,
            message.title,
            message.content,
            message.extra ?: emptyMap()
        )
    }

    override fun onNotificationMessageClicked(context: Context, message: MiPushMessage) {
        XiaomiBridge.onNotificationClicked(
            message.title,
            message.description,
            message.extra ?: emptyMap()
        )
    }

    override fun onNotificationMessageArrived(context: Context, message: MiPushMessage) {
        XiaomiBridge.onNotificationArrived(
            context,
            message.title,
            message.description,
            message.extra ?: emptyMap()
        )
    }
}
