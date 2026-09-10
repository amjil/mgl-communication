import 'dart:async';

import '../models/push_event.dart';

/// Buffers native events until Flutter listeners attach (cold start safe).
class EventBuffer {
  EventBuffer() {
    _controller = StreamController<PushEvent>.broadcast(
      onListen: _flush,
    );
  }

  late final StreamController<PushEvent> _controller;
  final List<PushEvent> _pending = [];
  bool _hasListener = false;

  Stream<PushEvent> get stream => _controller.stream;

  void add(PushEvent event) {
    if (_hasListener && !_controller.isClosed) {
      _controller.add(event);
    } else {
      _pending.add(event);
    }
  }

  void _flush() {
    _hasListener = true;
    for (final e in _pending) {
      if (!_controller.isClosed) _controller.add(e);
    }
    _pending.clear();
  }

  Future<void> close() => _controller.close();
}
