package net.amjil.mglpush

/**
 * Example OPPO/HeyTap service/callback wiring for host apps.
 *
 * Add HeyTap Push SDK, meta-data OPPO_APP_KEY / OPPO_APP_SECRET,
 * and forward click/data intents to [OppoBridge].
 *
 * ```kotlin
 * // In your Application / DataMessageCallback:
 * OppoBridge.onRegister(registerId)
 * OppoBridge.onMessage(title, content, extras)
 * OppoBridge.onNotificationOpened(extras)
 * ```
 *
 * Create notification channel `mgl_default` (or your Category) on Android 8+.
 */
@Suppress("unused")
object MglOppoPushDocs
