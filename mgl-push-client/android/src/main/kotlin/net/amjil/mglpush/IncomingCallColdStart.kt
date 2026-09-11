package net.amjil.mglpush

/**
 * Shared cold-start Incoming Call Call Control when Flutter plugin is not yet attached.
 */
object IncomingCallColdStart {
    fun handleDomainEvent(context: android.content.Context?, event: Map<String, Any?>) {
        val ctx = context ?: return
        when (event["type"] as? String) {
            "incoming-call" -> {
                val data = event["data"]
                val action = if (data is Map<*, *>) data["action"]?.toString() else null
                if (action.isNullOrEmpty() || action == "ringing") {
                    IncomingCallNotifier.show(ctx, event)
                }
            }
            "call-cancelled", "call-ended" -> {
                val data = event["data"]
                val callId = if (data is Map<*, *>) {
                    data["call_id"]?.toString() ?: data["callId"]?.toString()
                } else null
                IncomingCallNotifier.dismiss(ctx, callId)
            }
        }
    }

    fun saveAndMaybeShow(context: android.content.Context?, provider: String, message: ProviderMessage) {
        val event = PendingNativeStore.toDomainEventMap(provider, message)
        PendingNativeStore.save(context, event)
        handleDomainEvent(context, event)
    }
}
