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
  ///
  /// Incoming-call with `action` (accepted / rejected / timeout) is allowed
  /// after the ringing event for the same call_id.
  bool accept({
    required String eventId,
    String? callId,
    String? eventType,
    String? callAction,
  }) {
    _purge();
    final now = DateTime.now();

    if (eventId.isNotEmpty) {
      if (_seenEvents.containsKey(eventId)) return false;
      _seenEvents[eventId] = now;
    }

    final isIncoming =
        eventType == 'incoming-call' || eventType == 'incoming_call';
    final isRinging = callAction == null ||
        callAction.isEmpty ||
        callAction == 'ringing';

    // Incoming call: only one ringing per call_id.
    if (callId != null && callId.isNotEmpty && isIncoming && isRinging) {
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
