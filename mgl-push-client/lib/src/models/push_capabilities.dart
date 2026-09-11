/// Device push + Incoming Call capabilities (spec §17 / §95).
class PushCapabilities {
  const PushCapabilities({
    this.notification = true,
    this.silentPush = true,
    this.backgroundPush = true,
    this.incomingCallPush = false,
    this.callkit = false,
    this.liveCommunicationKit = false,
    this.telecom = false,
    this.fullScreenIntent = true,
    this.providers = const [],
  });

  final bool notification;
  final bool silentPush;
  final bool backgroundPush;
  final bool incomingCallPush;

  /// iOS CallKit available (Call Control).
  final bool callkit;

  /// iOS LiveCommunicationKit available when present.
  final bool liveCommunicationKit;

  /// Android Incoming Call UI / Telecom-style Call Control.
  final bool telecom;

  /// Android 14+ USE_FULL_SCREEN_INTENT grant status (true on older APIs / iOS).
  final bool fullScreenIntent;

  final List<String> providers;

  Map<String, dynamic> toJson() => {
        'notification': notification,
        'silent_push': silentPush,
        'background_push': backgroundPush,
        'incoming_call_push': incomingCallPush,
        'callkit': callkit,
        'live_communication_kit': liveCommunicationKit,
        'telecom': telecom,
        'full_screen_intent': fullScreenIntent,
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
      callkit: map['callkit'] as bool? ?? false,
      liveCommunicationKit: map['live_communication_kit'] as bool? ??
          map['liveCommunicationKit'] as bool? ??
          false,
      telecom: map['telecom'] as bool? ?? false,
      fullScreenIntent: map['full_screen_intent'] as bool? ??
          map['fullScreenIntent'] as bool? ??
          true,
      providers: providers,
    );
  }
}
