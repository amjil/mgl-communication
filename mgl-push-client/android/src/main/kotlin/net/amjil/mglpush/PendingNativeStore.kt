package net.amjil.mglpush

import android.content.Context
import android.content.SharedPreferences
import org.json.JSONArray
import org.json.JSONObject

/**
 * Shared short-lived pending event store for cold start (spec §38–39).
 * Used when Flutter / provider Bridge is not yet initialized.
 */
object PendingNativeStore {
    private const val PREFS = "mgl_push_pending"
    private const val KEY_EVENTS = "events"
    const val DEFAULT_TTL_MS = 120_000L
    private const val MAX_EVENTS = 32

    fun save(context: Context?, event: Map<String, Any?>) {
        val ctx = context ?: return
        val type = event["type"] as? String ?: return
        if (type !in setOf(
                "incoming-call", "call-cancelled", "call-ended",
                "silent", "background", "notification"
            )
        ) {
            return
        }
        try {
            val p = prefs(ctx)
            val arr = JSONArray(p.getString(KEY_EVENTS, "[]"))
            val wrap = JSONObject()
            wrap.put("received_at", System.currentTimeMillis())
            wrap.put("event", mapToJson(event))
            arr.put(wrap)
            while (arr.length() > MAX_EVENTS) {
                arr.remove(0)
            }
            p.edit().putString(KEY_EVENTS, arr.toString()).apply()
        } catch (_: Exception) {
        }
    }

    fun saveProviderMessage(context: Context?, provider: String, message: ProviderMessage) {
        save(context, toDomainEventMap(provider, message))
    }

    fun load(context: Context?, ttlMs: Long = DEFAULT_TTL_MS): List<Map<String, Any?>> {
        val ctx = context ?: return emptyList()
        return try {
            val raw = prefs(ctx).getString(KEY_EVENTS, "[]") ?: return emptyList()
            val arr = JSONArray(raw)
            val now = System.currentTimeMillis()
            val out = mutableListOf<Map<String, Any?>>()
            for (i in 0 until arr.length()) {
                val wrap = arr.getJSONObject(i)
                val receivedAt = wrap.optLong("received_at", 0)
                if (now - receivedAt > ttlMs) continue
                val eventObj = wrap.optJSONObject("event") ?: continue
                out.add(jsonToMap(eventObj))
            }
            out
        } catch (_: Exception) {
            emptyList()
        }
    }

    fun clear(context: Context?) {
        context ?: return
        try {
            prefs(context).edit().remove(KEY_EVENTS).apply()
        } catch (_: Exception) {
        }
    }

    fun toDomainEventMap(provider: String, message: ProviderMessage): Map<String, Any?> {
        val data = message.data.toMutableMap()
        if (!message.title.isNullOrEmpty()) data["title"] = message.title
        if (!message.body.isNullOrEmpty()) data["body"] = message.body
        if (!message.deepLink.isNullOrEmpty()) data["deep_link"] = message.deepLink

        val rawType = data["mgl_event_type"].orEmpty()
        val eventType = normalizeEventType(
            rawType.ifEmpty {
                if (message.title.isNullOrEmpty() && message.body.isNullOrEmpty()) "silent" else "notification"
            }
        )
        val id = message.messageId
            ?: data["mgl_event_id"]
            ?: data["mgl_message_id"]
            ?: ""
        val ts = data["mgl_timestamp"]?.toLongOrNull()
            ?: (System.currentTimeMillis() / 1000)

        return mapOf(
            "version" to 1,
            "type" to eventType,
            "id" to id,
            "timestamp" to ts,
            "provider" to provider,
            "data" to data
        )
    }

    fun normalizeEventType(raw: String): String = when (raw) {
        "incoming_call", "incoming-call" -> "incoming-call"
        "call_cancelled", "call-cancelled" -> "call-cancelled"
        "call_ended", "call-ended" -> "call-ended"
        "message" -> "notification"
        else -> raw.ifEmpty { "notification" }
    }

    private fun prefs(ctx: Context): SharedPreferences =
        ctx.getSharedPreferences(PREFS, Context.MODE_PRIVATE)

    private fun mapToJson(map: Map<*, *>): JSONObject {
        val obj = JSONObject()
        for ((k, v) in map) {
            if (k == null) continue
            val key = k.toString()
            when (v) {
                null -> obj.put(key, JSONObject.NULL)
                is Map<*, *> -> obj.put(key, mapToJson(v))
                is List<*> -> {
                    val a = JSONArray()
                    v.forEach { a.put(it) }
                    obj.put(key, a)
                }
                else -> obj.put(key, v)
            }
        }
        return obj
    }

    private fun jsonToMap(obj: JSONObject): Map<String, Any?> {
        val map = mutableMapOf<String, Any?>()
        val keys = obj.keys()
        while (keys.hasNext()) {
            val k = keys.next()
            val v = obj.get(k)
            map[k] = when (v) {
                is JSONObject -> jsonToMap(v)
                JSONObject.NULL -> null
                else -> v
            }
        }
        return map
    }
}
