/// Unified domain push event (spec §24).
///
/// Protocol shape:
/// ```json
/// {"version":1,"id":"evt_…","type":"incoming-call","timestamp":…,"data":{…}}
/// ```
sealed class PushEvent {
  const PushEvent();
}

/// Domain event delivered via push transport.
class DomainPushEvent extends PushEvent {
  const DomainPushEvent({
    required this.id,
    required this.type,
    required this.timestamp,
    this.version = 1,
    this.data = const {},
    this.provider,
  });

  final int version;
  final String id;
  final String type;
  final int timestamp;
  final Map<String, String> data;
  final String? provider;

  bool get isNotification => type == PushEventType.notification;
  bool get isSilent => type == PushEventType.silent;
  bool get isBackground => type == PushEventType.background;
  bool get isIncomingCall => type == PushEventType.incomingCall;
  bool get isCallCancelled => type == PushEventType.callCancelled;
  bool get isCallEnded => type == PushEventType.callEnded;

  String? get callId => data['call_id'] ?? data['callId'];
}

/// Transport / lifecycle events (not part of the business PushEvent contract).
class TokenChangedEvent extends PushEvent {
  const TokenChangedEvent({required this.provider, required this.token});

  final String provider;
  final String token;
}

class NotificationOpenEvent extends PushEvent {
  const NotificationOpenEvent({
    this.messageId,
    this.provider,
    this.data = const {},
    this.deepLink,
  });

  final String? messageId;
  final String? provider;
  final Map<String, String> data;
  final String? deepLink;
}

class ErrorEvent extends PushEvent {
  const ErrorEvent({
    this.provider,
    required this.code,
    required this.message,
  });

  final String? provider;
  final String code;
  final String message;
}

/// Canonical event type strings used on the wire (kebab-case for call events).
abstract final class PushEventType {
  static const notification = 'notification';
  static const silent = 'silent';
  static const background = 'background';
  static const incomingCall = 'incoming-call';
  static const callCancelled = 'call-cancelled';
  static const callEnded = 'call-ended';

  /// Map server `mgl_event_type` / MessageType to client event type.
  static String normalize(String raw) {
    switch (raw) {
      case 'incoming_call':
      case 'incoming-call':
        return incomingCall;
      case 'call_cancelled':
      case 'call-cancelled':
        return callCancelled;
      case 'call_ended':
      case 'call-ended':
        return callEnded;
      case 'notification':
      case 'silent':
      case 'background':
        return raw;
      case 'message':
        return notification;
      default:
        return raw.isEmpty ? notification : raw;
    }
  }
}

Map<String, String> _stringMap(Object? raw) {
  final data = <String, String>{};
  if (raw is Map) {
    raw.forEach((k, v) {
      if (k != null && v != null) data[k.toString()] = v.toString();
    });
  }
  return data;
}

PushEvent parsePushEvent(Map<Object?, Object?> map) {
  final type = map['type'] as String? ?? '';
  switch (type) {
    case 'token_changed':
      return TokenChangedEvent(
        provider: map['provider'] as String? ?? '',
        token: map['token'] as String? ?? '',
      );
    case 'notification_open':
      return NotificationOpenEvent(
        messageId: map['message_id'] as String?,
        provider: map['provider'] as String?,
        data: _stringMap(map['data']),
        deepLink: map['deep_link'] as String?,
      );
    case 'error':
      return ErrorEvent(
        provider: map['provider'] as String?,
        code: map['code'] as String? ?? 'UNKNOWN',
        message: map['message'] as String? ?? '',
      );
    case 'message':
      // Legacy native emission → domain notification/silent.
      return _domainFromLegacyMessage(map);
    case PushEventType.notification:
    case PushEventType.silent:
    case PushEventType.background:
    case PushEventType.incomingCall:
    case PushEventType.callCancelled:
    case PushEventType.callEnded:
    case 'incoming_call':
    case 'call_cancelled':
    case 'call_ended':
      return _domainFromMap(map, PushEventType.normalize(type));
    default:
      // Unknown: try domain shape with id/data.
      if (map.containsKey('id') || map.containsKey('data')) {
        return _domainFromMap(map, PushEventType.normalize(type));
      }
      return ErrorEvent(code: 'UNKNOWN_EVENT', message: 'Unknown event type: $type');
  }
}

DomainPushEvent _domainFromLegacyMessage(Map<Object?, Object?> map) {
  final data = _stringMap(map['data']);
  final title = map['title'] as String?;
  final body = map['body'] as String?;
  if (title != null && title.isNotEmpty) data['title'] = title;
  if (body != null && body.isNotEmpty) data['body'] = body;
  final deepLink = map['deep_link'] as String?;
  if (deepLink != null && deepLink.isNotEmpty) data['deep_link'] = deepLink;

  final rawType = data['mgl_event_type'] ?? '';
  final eventType = rawType.isNotEmpty
      ? PushEventType.normalize(rawType)
      : ((title == null || title.isEmpty) && (body == null || body.isEmpty)
          ? PushEventType.silent
          : PushEventType.notification);

  final id = map['message_id'] as String? ??
      data['mgl_event_id'] ??
      data['mgl_message_id'] ??
      data['event_id'] ??
      '';
  final ts = int.tryParse(data['mgl_timestamp'] ?? '') ??
      DateTime.now().millisecondsSinceEpoch ~/ 1000;

  return DomainPushEvent(
    id: id,
    type: eventType,
    timestamp: ts,
    data: data,
    provider: map['provider'] as String?,
  );
}

DomainPushEvent _domainFromMap(Map<Object?, Object?> map, String type) {
  final data = _stringMap(map['data']);
  final id = map['id'] as String? ??
      map['message_id'] as String? ??
      data['mgl_event_id'] ??
      data['mgl_message_id'] ??
      '';
  final ts = map['timestamp'] is int
      ? map['timestamp'] as int
      : int.tryParse('${map['timestamp'] ?? data['mgl_timestamp'] ?? ''}') ??
          DateTime.now().millisecondsSinceEpoch ~/ 1000;
  final version = map['version'] is int ? map['version'] as int : 1;
  return DomainPushEvent(
    version: version,
    id: id,
    type: type,
    timestamp: ts,
    data: data,
    provider: map['provider'] as String?,
  );
}
