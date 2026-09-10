class PushMessage {
  const PushMessage({
    this.messageId,
    this.provider,
    this.title,
    this.body,
    this.data = const {},
    this.deepLink,
  });

  final String? messageId;
  final String? provider;
  final String? title;
  final String? body;
  final Map<String, String> data;
  final String? deepLink;

  factory PushMessage.fromMap(Map<Object?, Object?> map) {
    final rawData = map['data'];
    final data = <String, String>{};
    if (rawData is Map) {
      rawData.forEach((k, v) {
        if (k != null && v != null) {
          data[k.toString()] = v.toString();
        }
      });
    }
    return PushMessage(
      messageId: map['message_id'] as String? ?? map['messageId'] as String?,
      provider: map['provider'] as String?,
      title: map['title'] as String?,
      body: map['body'] as String?,
      data: data,
      deepLink: map['deep_link'] as String? ?? map['deepLink'] as String?,
    );
  }
}
