package net.amjil.mglpush

import android.app.Activity
import android.content.Context
import io.flutter.embedding.engine.plugins.FlutterPlugin
import io.flutter.embedding.engine.plugins.activity.ActivityAware
import io.flutter.embedding.engine.plugins.activity.ActivityPluginBinding
import io.flutter.plugin.common.EventChannel
import io.flutter.plugin.common.MethodCall
import io.flutter.plugin.common.MethodChannel

class MglPushPlugin : FlutterPlugin, MethodChannel.MethodCallHandler, EventChannel.StreamHandler, ActivityAware {

    private lateinit var methodChannel: MethodChannel
    private lateinit var eventChannel: EventChannel
    private var context: Context? = null
    private var activity: Activity? = null
    private var eventSink: EventChannel.EventSink? = null
    private val pendingEvents = mutableListOf<Map<String, Any?>>()

    private var activeProvider: PushProvider? = null
    private var initialNotification: Map<String, Any?>? = null
    private var availableProviders: List<PushProvider> = emptyList()

    private val callback = object : ProviderCallback {
        override fun onTokenChanged(token: String) {
            emit(mapOf(
                "type" to "token_changed",
                "provider" to (activeProvider?.name() ?: "unknown"),
                "token" to token
            ))
        }

        override fun onMessage(message: ProviderMessage) {
            emit(toDomainEvent(message))
        }

        override fun onNotificationOpened(data: Map<String, String>) {
            val event = mapOf(
                "type" to "notification_open",
                "message_id" to (data["mgl_message_id"] ?: data["message_id"]),
                "provider" to (activeProvider?.name() ?: "fcm"),
                "data" to data,
                "deep_link" to data["deep_link"]
            )
            if (eventSink == null) {
                initialNotification = event
            }
            emit(event)
        }

        override fun onError(code: String, message: String) {
            emit(mapOf(
                "type" to "error",
                "provider" to (activeProvider?.name() ?: "unknown"),
                "code" to code,
                "message" to message
            ))
        }
    }

    override fun onAttachedToEngine(binding: FlutterPlugin.FlutterPluginBinding) {
        context = binding.applicationContext
        methodChannel = MethodChannel(binding.binaryMessenger, "net.amjil.mgl_push/methods")
        eventChannel = EventChannel(binding.binaryMessenger, "net.amjil.mgl_push/events")
        methodChannel.setMethodCallHandler(this)
        eventChannel.setStreamHandler(this)
        IncomingCallActionBridge.plugin = this
        IncomingCallNotifier.ensureChannel(binding.applicationContext)
        MglConnectionServiceAppContext.set(binding.applicationContext)
        TelecomIncomingCall.ensurePhoneAccount(binding.applicationContext)
    }

    override fun onDetachedFromEngine(binding: FlutterPlugin.FlutterPluginBinding) {
        methodChannel.setMethodCallHandler(null)
        eventChannel.setStreamHandler(null)
        if (IncomingCallActionBridge.plugin === this) {
            IncomingCallActionBridge.plugin = null
        }
        context = null
    }

    override fun onMethodCall(call: MethodCall, result: MethodChannel.Result) {
        when (call.method) {
            "initialize" -> {
                try {
                    initializeProviders()
                    result.success(null)
                } catch (e: Exception) {
                    emit(mapOf(
                        "type" to "error",
                        "code" to "INIT_ERROR",
                        "message" to (e.message ?: "initialize failed")
                    ))
                    result.success(null)
                }
            }
            "register" -> {
                val provider = activeProvider
                result.success(mapOf(
                    "platform" to "android",
                    "provider" to (provider?.name() ?: "fcm"),
                    "token" to (provider?.getToken() ?: "")
                ))
            }
            "getDevice" -> {
                val provider = activeProvider
                if (provider == null) {
                    result.success(null)
                } else {
                    result.success(mapOf(
                        "platform" to "android",
                        "provider" to provider.name(),
                        "token" to (provider.getToken() ?: "")
                    ))
                }
            }
            "unregister" -> {
                activeProvider?.unregister()
                result.success(null)
            }
            "setUserId" -> {
                val userId = call.argument<String>("userId")
                activeProvider?.setAlias(userId)
                result.success(null)
            }
            "clearUserId" -> {
                activeProvider?.setAlias(null)
                result.success(null)
            }
            "requestPermission" -> {
                val fcm = activeProvider as? FcmProvider
                if (fcm != null) {
                    fcm.requestPermission(activity) { result.success(it) }
                } else {
                    result.success(null)
                }
            }
            "getInitialNotification" -> {
                val n = initialNotification
                initialNotification = null
                result.success(n)
            }
            "consumePendingEvents" -> {
                val stored = PendingNativeStore.load(context)
                PendingNativeStore.clear(context)
                val all = mutableListOf<Map<String, Any?>>()
                all.addAll(stored)
                all.addAll(pendingEvents)
                pendingEvents.clear()
                // Replay Incoming Call UI for still-ringing pending events.
                for (event in all) {
                    maybeShowIncomingCall(event)
                }
                result.success(all)
            }
            "getCapabilities" -> {
                val names = availableProviders.map { it.name() }.ifEmpty {
                    listOf(activeProvider?.name() ?: "fcm")
                }
                val telecom = context?.let { TelecomIncomingCall.isAvailable(it) } == true
                result.success(mapOf(
                    "notification" to true,
                    "silent_push" to true,
                    "background_push" to true,
                    "incoming_call_push" to true,
                    "callkit" to false,
                    "live_communication_kit" to false,
                    "telecom" to telecom,
                    "providers" to names
                ))
            }
            "endSystemCall" -> {
                val callId = call.argument<String>("callId") ?: call.argument<String>("call_id")
                val ctx = context
                if (ctx != null) {
                    IncomingCallNotifier.dismiss(ctx, callId)
                }
                result.success(null)
            }
            else -> result.notImplemented()
        }
    }

    /** Used by [IncomingCallActionBridge] when Flutter engine is alive. */
    fun emitFromNative(event: Map<String, Any?>) {
        emit(event)
    }

    private fun initializeProviders() {
        val ctx = context ?: return
        val candidates = listOf(
            HuaweiProvider(ctx),
            XiaomiProvider(ctx),
            OppoProvider(ctx),
            VivoProvider(ctx),
            FcmProvider(ctx)
        )
        val detector = DefaultPushProviderDetector(ctx, candidates)
        availableProviders = detector.detect()
        activeProvider = availableProviders.firstOrNull() ?: run {
            val fcm = FcmProvider(ctx)
            callback.onError("PROVIDER_FALLBACK", "No push provider available; FCM will try init")
            fcm
        }
        activeProvider?.initialize(callback)
    }

    private fun toDomainEvent(message: ProviderMessage): Map<String, Any?> {
        return PendingNativeStore.toDomainEventMap(
            activeProvider?.name() ?: "unknown",
            message
        )
    }

    private fun emit(event: Map<String, Any?>) {
        handleCallControl(event)
        PendingNativeStore.save(context, event)
        val sink = eventSink
        if (sink != null) {
            sink.success(event)
        } else {
            pendingEvents.add(event)
        }
    }

    private fun handleCallControl(event: Map<String, Any?>) {
        val ctx = context ?: return
        when (event["type"] as? String) {
            "incoming-call" -> {
                val data = event["data"]
                val action = if (data is Map<*, *>) data["action"]?.toString() else null
                // Only show UI for ringing (no action yet).
                if (action.isNullOrEmpty() || action == "ringing") {
                    maybeShowIncomingCall(event)
                } else {
                    val callId = callIdOf(event)
                    IncomingCallNotifier.dismiss(ctx, callId)
                }
            }
            "call-cancelled", "call-ended" -> {
                IncomingCallNotifier.dismiss(ctx, callIdOf(event))
            }
        }
    }

    private fun maybeShowIncomingCall(event: Map<String, Any?>) {
        if (event["type"] != "incoming-call") return
        val data = event["data"]
        val action = if (data is Map<*, *>) data["action"]?.toString() else null
        if (!action.isNullOrEmpty() && action != "ringing") return
        val ctx = context ?: return
        if (isExpired(event)) return
        IncomingCallNotifier.show(ctx, event)
    }

    private fun callIdOf(event: Map<String, Any?>): String? {
        val data = event["data"]
        if (data is Map<*, *>) {
            return data["call_id"]?.toString() ?: data["callId"]?.toString()
        }
        return null
    }

    private fun isExpired(event: Map<String, Any?>): Boolean {
        val data = event["data"] as? Map<*, *> ?: return false
        val expires = data["expires_at"]?.toString()?.toLongOrNull() ?: return false
        val now = System.currentTimeMillis() / 1000
        return now > expires
    }

    override fun onListen(arguments: Any?, events: EventChannel.EventSink?) {
        eventSink = events
        pendingEvents.forEach { events?.success(it) }
        pendingEvents.clear()
    }

    override fun onCancel(arguments: Any?) {
        eventSink = null
    }

    override fun onAttachedToActivity(binding: ActivityPluginBinding) {
        activity = binding.activity
        binding.activity.intent?.extras?.let { extras ->
            if (extras.containsKey("mgl_message_id") ||
                extras.containsKey("mgl_event_id") ||
                extras.containsKey("call_id") ||
                extras.containsKey("google.message_id") ||
                extras.containsKey("from")
            ) {
                val data = mutableMapOf<String, String>()
                for (key in extras.keySet()) {
                    extras.get(key)?.let { data[key] = it.toString() }
                }
                if (data.isNotEmpty()) {
                    callback.onNotificationOpened(data)
                }
            }
        }
    }

    override fun onDetachedFromActivityForConfigChanges() {
        activity = null
    }

    override fun onReattachedToActivityForConfigChanges(binding: ActivityPluginBinding) {
        activity = binding.activity
    }

    override fun onDetachedFromActivity() {
        activity = null
    }
}
