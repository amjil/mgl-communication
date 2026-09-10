import '../models/push_device.dart';
import '../models/push_event.dart';
import '../models/push_message.dart';

abstract class MglPush {
  Future<void> initialize();

  Future<PushDevice> register();

  Future<PushDevice?> getDevice();

  Future<void> unregister();

  Future<void> setUserId(String userId);

  Future<void> clearUserId();

  Future<void> requestPermission();

  Stream<PushMessage> get messages;

  Stream<PushEvent> get events;
}
