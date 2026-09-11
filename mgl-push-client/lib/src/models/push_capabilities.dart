/// Device push capabilities (spec §17). Does not include CallKit / WebRTC.
class PushCapabilities {
  const PushCapabilities({
    this.notification = true,
    this.silentPush = true,
    this.backgroundPush = true,
    this.incomingCallPush = false,
    this.providers = const [],
  });

  final bool notification;
  final bool silentPush;
  final bool backgroundPush;
  final bool incomingCallPush;
  final List<String> providers;

  Map<String, dynamic> toJson() => {
        'notification': notification,
        'silent_push': silentPush,
        'background_push': backgroundPush,
        'incoming_call_push': incomingCallPush,
        'providers': providers,
      };

  factory PushCapabilities.fromMap(Map<Object?, Object?> map) {
    final rawProviders = map['providers'];
    final providers = <String>[];
    if (rawProviders is List) {
      for (final p in rawProviders) {
        if (p != null) providers.add(p.toString());
      }
    }
    return PushCapabilities(
      notification: map['notification'] as bool? ?? true,
      silentPush: map['silent_push'] as bool? ?? map['silentPush'] as bool? ?? true,
      backgroundPush:
          map['background_push'] as bool? ?? map['backgroundPush'] as bool? ?? true,
      incomingCallPush: map['incoming_call_push'] as bool? ??
          map['incomingCallPush'] as bool? ??
          false,
      providers: providers,
    );
  }
}
