/**
 * Example MiPush PushMessageReceiver for host apps (Phase 2).
 *
 * Full copy-paste sample: `android/examples/AppMiPushReceiver.kt`
 *
 * Host app must:
 * 1. Add MiPush Android SDK (AAR from Xiaomi console)
 * 2. Declare meta-data `XIAOMI_APP_ID` / `XIAOMI_APP_KEY`
 * 3. Register a `PushMessageReceiver` that forwards to [XiaomiBridge]
 *
 * When Flutter is not yet running, [XiaomiBridge] writes to [PendingNativeStore]
 * so cold-start incoming-call / silent events are not lost.
 */
@Suppress("unused")
object MglMiPushReceiverDocs
