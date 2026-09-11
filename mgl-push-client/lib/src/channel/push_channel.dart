import 'dart:async';

import 'package:flutter/services.dart';

import '../events/event_buffer.dart';
import '../models/push_event.dart';
import '../models/push_message.dart';

const methodChannelName = 'net.amjil.mgl_push/methods';
const eventChannelName = 'net.amjil.mgl_push/events';

class PushChannel {
  PushChannel({
    MethodChannel? methods,
    EventChannel? events,
  })  : _methods = methods ?? const MethodChannel(methodChannelName),
        _events = events ?? const EventChannel(eventChannelName);

  final MethodChannel _methods;
  final EventChannel _events;
  final EventBuffer _buffer = EventBuffer();
  StreamSubscription<dynamic>? _sub;
  bool _listening = false;

  Future<T?> invoke<T>(String method, [Map<String, dynamic>? args]) {
    return _methods.invokeMethod<T>(method, args);
  }

  Future<Map<Object?, Object?>?> invokeMap(String method, [Map<String, dynamic>? args]) async {
    final result = await _methods.invokeMethod<dynamic>(method, args);
    if (result == null) return null;
    return Map<Object?, Object?>.from(result as Map);
  }

  Stream<PushEvent> get events {
    _ensureListening();
    return _buffer.stream;
  }

  Stream<PushMessage> get messages =>
      events.where((e) => e is DomainPushEvent).cast<DomainPushEvent>().map((e) {
        return PushMessage(
          messageId: e.id,
          provider: e.provider,
          title: e.data['title'],
          body: e.data['body'],
          data: e.data,
          deepLink: e.data['deep_link'],
        );
      });

  /// Inject a parsed event (e.g. from consumePendingEvents / getInitialNotification).
  void inject(PushEvent event) => _buffer.add(event);

  void _ensureListening() {
    if (_listening) return;
    _listening = true;
    _sub = _events.receiveBroadcastStream().listen((event) {
      if (event is Map) {
        _buffer.add(parsePushEvent(Map<Object?, Object?>.from(event)));
      }
    }, onError: (Object e) {
      _buffer.add(ErrorEvent(code: 'CHANNEL_ERROR', message: e.toString()));
    });
  }

  Future<void> dispose() async {
    await _sub?.cancel();
    await _buffer.close();
  }
}
