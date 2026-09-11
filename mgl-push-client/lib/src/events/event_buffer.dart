import 'dart:async';

import '../models/push_event.dart';
import 'event_deduper.dart';

/// Buffers native events until Flutter listeners attach (cold start safe).
/// Also applies event_id / call_id deduplication.
class EventBuffer {
  EventBuffer({EventDeduper? deduper}) : _deduper = deduper ?? EventDeduper() {
    _controller = StreamController<PushEvent>.broadcast(
      onListen: _flush,
      onCancel: _onCancel,
    );
  }

  late final StreamController<PushEvent> _controller;
  final EventDeduper _deduper;
  final List<PushEvent> _pending = [];
  bool _hasListener = false;

  Stream<PushEvent> get stream => _controller.stream;

  void add(PushEvent event) {
    if (!_shouldAccept(event)) return;
    if (_hasListener && !_controller.isClosed) {
      _controller.add(event);
    } else {
      _pending.add(event);
    }
  }

  bool _shouldAccept(PushEvent event) {
    if (event is DomainPushEvent) {
      // Drop expired incoming calls (spec §42).
      if (event.isIncomingCall) {
        final expires = int.tryParse(event.data['expires_at'] ?? '');
        if (expires != null) {
          final now = DateTime.now().millisecondsSinceEpoch ~/ 1000;
          if (now > expires) return false;
        }
      }
      return _deduper.accept(
        eventId: event.id,
        callId: event.callId,
        eventType: event.type,
      );
    }
    return true;
  }

  void _flush() {
    _hasListener = true;
    for (final e in _pending) {
      if (!_controller.isClosed) _controller.add(e);
    }
    _pending.clear();
  }

  void _onCancel() {
    // Broadcast onCancel fires when the last listener detaches.
    // Buffer again so events are not dropped before a new listener attaches.
    _hasListener = false;
  }

  Future<void> close() => _controller.close();
}
