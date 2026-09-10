import 'push_message.dart';

sealed class PushEvent {
  const PushEvent({required this.type});
  final String type;
}

class TokenChangedEvent extends PushEvent {
  const TokenChangedEvent({required this.provider, required this.token})
      : super(type: 'token_changed');

  final String provider;
  final String token;
}

class MessageEvent extends PushEvent {
  const MessageEvent({required this.message}) : super(type: 'message');
  final PushMessage message;
}

class NotificationOpenEvent extends PushEvent {
  const NotificationOpenEvent({
    this.messageId,
    this.provider,
    this.data = const {},
    this.deepLink,
  }) : super(type: 'notification_open');

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
  }) : super(type: 'error');

  final String? provider;
  final String code;
  final String message;
}

PushEvent parsePushEvent(Map<Object?, Object?> map) {
  final type = map['type'] as String? ?? '';
  switch (type) {
    case 'token_changed':
      return TokenChangedEvent(
        provider: map['provider'] as String? ?? '',
        token: map['token'] as String? ?? '',
      );
    case 'message':
      return MessageEvent(message: PushMessage.fromMap(map));
    case 'notification_open':
      final rawData = map['data'];
      final data = <String, String>{};
      if (rawData is Map) {
        rawData.forEach((k, v) {
          if (k != null && v != null) data[k.toString()] = v.toString();
        });
      }
      return NotificationOpenEvent(
        messageId: map['message_id'] as String?,
        provider: map['provider'] as String?,
        data: data,
        deepLink: map['deep_link'] as String?,
      );
    case 'error':
      return ErrorEvent(
        provider: map['provider'] as String?,
        code: map['code'] as String? ?? 'UNKNOWN',
        message: map['message'] as String? ?? '',
      );
    default:
      return ErrorEvent(code: 'UNKNOWN_EVENT', message: 'Unknown event type: $type');
  }
}
