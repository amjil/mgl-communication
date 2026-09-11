package net.amjil.mglpush

import android.app.Activity
import android.content.Context
import android.content.Intent
import android.graphics.Color
import android.os.Build
import android.os.Bundle
import android.view.Gravity
import android.view.WindowManager
import android.widget.Button
import android.widget.LinearLayout
import android.widget.TextView
import org.json.JSONObject

/**
 * Full-screen Incoming Call UI (Android Call Control).
 * Accept → emit incoming-call action=accepted; Reject → action=rejected.
 */
class IncomingCallActivity : Activity() {

    private var callId: String = ""
    private var eventId: String = ""
    private var provider: String? = null
    private var payload: Map<String, String> = emptyMap()
    private var finishedByAction = false

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        turnScreenOn()
        parseIntent(intent)

        val caller = intent.getStringExtra(IncomingCallNotifier.EXTRA_CALLER_NAME) ?: "Incoming Call"
        val media = intent.getStringExtra(IncomingCallNotifier.EXTRA_MEDIA_TYPE) ?: "audio"

        val root = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            setBackgroundColor(Color.parseColor("#121212"))
            gravity = Gravity.CENTER
            setPadding(48, 48, 48, 48)
        }

        val title = TextView(this).apply {
            text = caller
            setTextColor(Color.WHITE)
            textSize = 28f
            gravity = Gravity.CENTER
        }
        val subtitle = TextView(this).apply {
            text = if (media == "video") "Video call" else "Audio call"
            setTextColor(Color.parseColor("#B0B0B0"))
            textSize = 16f
            gravity = Gravity.CENTER
            setPadding(0, 24, 0, 72)
        }

        val buttons = LinearLayout(this).apply {
            orientation = LinearLayout.HORIZONTAL
            gravity = Gravity.CENTER
        }

        val reject = Button(this).apply {
            text = "Reject"
            setBackgroundColor(Color.parseColor("#C62828"))
            setTextColor(Color.WHITE)
            setOnClickListener { complete("rejected") }
        }
        val accept = Button(this).apply {
            text = "Accept"
            setBackgroundColor(Color.parseColor("#2E7D32"))
            setTextColor(Color.WHITE)
            setOnClickListener { complete("accepted") }
        }

        val lp = LinearLayout.LayoutParams(0, LinearLayout.LayoutParams.WRAP_CONTENT, 1f).apply {
            marginStart = 16
            marginEnd = 16
        }
        buttons.addView(reject, lp)
        buttons.addView(accept, lp)

        root.addView(title)
        root.addView(subtitle)
        root.addView(buttons)
        setContentView(root)
    }

    override fun onNewIntent(intent: Intent?) {
        super.onNewIntent(intent)
        if (intent != null) {
            setIntent(intent)
            parseIntent(intent)
        }
    }

    override fun onDestroy() {
        if (!isChangingConfigurations && !finishedByAction && callId.isNotEmpty()) {
            // Back / system dismiss while ringing → treat as reject.
            IncomingCallActionBridge.emitAction(
                applicationContext,
                callId = callId,
                eventId = eventId,
                action = "rejected",
                payload = payload,
                provider = provider
            )
            IncomingCallNotifier.dismiss(applicationContext, callId)
        }
        super.onDestroy()
    }

    private fun complete(action: String) {
        finishedByAction = true
        IncomingCallActionBridge.emitAction(
            applicationContext,
            callId = callId,
            eventId = eventId,
            action = action,
            payload = payload,
            provider = provider
        )
        IncomingCallNotifier.dismiss(applicationContext, callId)
        finish()
    }

    private fun parseIntent(intent: Intent) {
        callId = intent.getStringExtra(IncomingCallNotifier.EXTRA_CALL_ID).orEmpty()
        eventId = intent.getStringExtra(IncomingCallNotifier.EXTRA_EVENT_ID).orEmpty()
        provider = intent.getStringExtra("provider")
        val json = intent.getStringExtra(IncomingCallNotifier.EXTRA_PAYLOAD).orEmpty()
        payload = parseJsonMap(json)
    }

    private fun turnScreenOn() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O_MR1) {
            setShowWhenLocked(true)
            setTurnScreenOn(true)
        } else {
            @Suppress("DEPRECATION")
            window.addFlags(
                WindowManager.LayoutParams.FLAG_SHOW_WHEN_LOCKED or
                    WindowManager.LayoutParams.FLAG_TURN_SCREEN_ON or
                    WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON
            )
        }
        window.addFlags(WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON)
    }

    companion object {
        fun intent(
            context: Context,
            callId: String,
            eventId: String,
            callerName: String,
            mediaType: String,
            payloadJson: String,
            provider: String?
        ): Intent {
            return Intent(context, IncomingCallActivity::class.java).apply {
                putExtra(IncomingCallNotifier.EXTRA_CALL_ID, callId)
                putExtra(IncomingCallNotifier.EXTRA_EVENT_ID, eventId)
                putExtra(IncomingCallNotifier.EXTRA_CALLER_NAME, callerName)
                putExtra(IncomingCallNotifier.EXTRA_MEDIA_TYPE, mediaType)
                putExtra(IncomingCallNotifier.EXTRA_PAYLOAD, payloadJson)
                putExtra("provider", provider)
                addFlags(Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_CLEAR_TOP or Intent.FLAG_ACTIVITY_SINGLE_TOP)
            }
        }

        fun parseJsonMap(json: String): Map<String, String> {
            if (json.isBlank()) return emptyMap()
            return try {
                val obj = JSONObject(json)
                val out = mutableMapOf<String, String>()
                val keys = obj.keys()
                while (keys.hasNext()) {
                    val k = keys.next()
                    out[k] = obj.optString(k)
                }
                out
            } catch (_: Exception) {
                emptyMap()
            }
        }
    }
}
