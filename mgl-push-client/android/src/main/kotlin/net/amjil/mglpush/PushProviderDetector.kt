package net.amjil.mglpush

import android.content.Context
import android.os.Build

/**
 * Detects available push providers by SDK availability (not manufacturer alone).
 * Order of [providers] list defines preference when multiple are available.
 */
class DefaultPushProviderDetector(
    private val context: Context,
    private val providers: List<PushProvider>
) : PushProviderDetector {

    override fun detect(): List<PushProvider> {
        return providers.filter { it.isAvailable() }
    }
}

interface PushProviderDetector {
    fun detect(): List<PushProvider>
}

/** Heuristic helpers — not used alone for selection (spec §10). */
object ManufacturerHints {
    fun manufacturer(): String = Build.MANUFACTURER.lowercase()

    fun likelyHuawei(): Boolean {
        val m = manufacturer()
        return m.contains("huawei") || m.contains("honor")
    }

    fun likelyXiaomi(): Boolean {
        val m = manufacturer()
        return m.contains("xiaomi") || m.contains("redmi") || m.contains("poco")
    }

    fun likelyOppo(): Boolean {
        val m = manufacturer()
        return m.contains("oppo") || m.contains("realme") || m.contains("oneplus")
    }

    fun likelyVivo(): Boolean {
        val m = manufacturer()
        return m.contains("vivo") || m.contains("iqoo")
    }
}
