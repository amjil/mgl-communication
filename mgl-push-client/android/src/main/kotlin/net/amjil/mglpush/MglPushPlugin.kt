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

    private val callback = object : ProviderCallback {
        override fun onTokenChanged(token: String) {
            emit(mapOf(
                "type" to "token_changed",
                "provider" to (activeProvider?.name() ?: "unknown"),
                "token" to token
            ))
        }

        override fun onMessage(message: ProviderMessage) {
            emit(mapOf(
                "type" to "message",
                "message_id" to message.messageId,
                "provider" to (activeProvider?.name() ?: "unknown"),
                "title" to message.title,
                "body" to message.body,
                "data" to message.data,
                "deep_link" to message.deepLink
            ))
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
    }

    override fun onDetachedFromEngine(binding: FlutterPlugin.FlutterPluginBinding) {
        methodChannel.setMethodCallHandler(null)
        eventChannel.setStreamHandler(null)
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
            else -> result.notImplemented()
        }
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
        val available = detector.detect()
        // Prefer first available vendor SDK (Huawei before FCM when HMS is present).
        activeProvider = available.firstOrNull() ?: run {
            val fcm = FcmProvider(ctx)
            callback.onError("PROVIDER_FALLBACK", "No push provider available; FCM will try init")
            fcm
        }
        activeProvider?.initialize(callback)
    }

    private fun emit(event: Map<String, Any?>) {
        val sink = eventSink
        if (sink != null) {
            sink.success(event)
        } else {
            pendingEvents.add(event)
        }
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
