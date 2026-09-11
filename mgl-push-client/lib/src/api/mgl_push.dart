import '../models/push_capabilities.dart';
import '../models/push_device.dart';
import '../models/push_event.dart';
import '../models/push_message.dart';

abstract class MglPush {
  /// Initialize native push and optionally register. Returns current device when known.
  Future<PushDevice?> initialize();

  Future<PushDevice> register();

  Future<void> unregister();

  Future<PushDevice?> getDevice();

  Future<void> setUserId(String userId);

  Future<void> clearUserId();

  Future<void> requestPermission();

  Future<PushCapabilities> getCapabilities();

  /// Dismiss / end System Call UI for [callId] (CallKit / Android Incoming Call).
  /// When [callId] is null, ends all active system calls.
  Future<void> endSystemCall([String? callId]);

  /// Domain + lifecycle events (notification, silent, incoming-call, token_changed, …).
  Stream<PushEvent> get events;

  /// Convenience stream of legacy [PushMessage] wrappers for notification-like data.
  @Deprecated('Use events and DomainPushEvent instead')
  Stream<PushMessage> get messages;
}
