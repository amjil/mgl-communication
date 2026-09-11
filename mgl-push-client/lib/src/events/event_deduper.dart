/// Deduplicates push events by event_id and (for calls) call_id (spec §40–41).
class EventDeduper {
  EventDeduper({
    this.maxEntries = 256,
    this.ttl = const Duration(minutes: 10),
  });

  final int maxEntries;
  final Duration ttl;

  final Map<String, DateTime> _seenEvents = {};
  final Map<String, DateTime> _seenCalls = {};

  /// Returns true if this event should be delivered (not a duplicate).
  bool accept({
    required String eventId,
    String? callId,
    String? eventType,
  }) {
    _purge();
    final now = DateTime.now();

    if (eventId.isNotEmpty) {
      if (_seenEvents.containsKey(eventId)) return false;
      _seenEvents[eventId] = now;
    }

    // Incoming call: only one ringing per call_id.
    if (callId != null &&
        callId.isNotEmpty &&
        (eventType == 'incoming-call' || eventType == 'incoming_call')) {
      if (_seenCalls.containsKey(callId)) return false;
      _seenCalls[callId] = now;
    }

    _trim(_seenEvents);
    _trim(_seenCalls);
    return true;
  }

  void _purge() {
    final cutoff = DateTime.now().subtract(ttl);
    _seenEvents.removeWhere((_, t) => t.isBefore(cutoff));
    _seenCalls.removeWhere((_, t) => t.isBefore(cutoff));
  }

  void _trim(Map<String, DateTime> map) {
    while (map.length > maxEntries) {
      final oldest = map.entries.reduce((a, b) => a.value.isBefore(b.value) ? a : b);
      map.remove(oldest.key);
    }
  }
}
