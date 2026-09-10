package net.amjil.mglpush

interface ProviderCallback {
    fun onTokenChanged(token: String)
    fun onMessage(message: ProviderMessage)
    fun onNotificationOpened(data: Map<String, String>)
    fun onError(code: String, message: String)
}

data class ProviderMessage(
    val messageId: String? = null,
    val title: String? = null,
    val body: String? = null,
    val data: Map<String, String> = emptyMap(),
    val deepLink: String? = null
)

interface PushProvider {
    fun name(): String
    fun isAvailable(): Boolean
    fun initialize(callback: ProviderCallback)
    fun getToken(): String?
    fun unregister()
    fun setAlias(alias: String?)
    fun subscribeTopic(topic: String)
    fun unsubscribeTopic(topic: String)
}
