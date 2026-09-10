import 'package:flutter_test/flutter_test.dart';
import 'package:mgl_push/src/events/event_buffer.dart';
import 'package:mgl_push/src/models/push_event.dart';

void main() {
  test('EventBuffer keeps events until listener attaches', () async {
    final buffer = EventBuffer();
    buffer.add(const TokenChangedEvent(provider: 'fcm', token: 'abc'));

    final events = <PushEvent>[];
    final sub = buffer.stream.listen(events.add);
    await Future<void>.delayed(Duration.zero);

    expect(events, hasLength(1));
    expect(events.first, isA<TokenChangedEvent>());
    await sub.cancel();
    await buffer.close();
  });

  test('parsePushEvent message', () {
    final e = parsePushEvent({
      'type': 'message',
      'message_id': '01K',
      'title': 't',
      'body': 'b',
      'data': {'type': 'comment'},
    });
    expect(e, isA<MessageEvent>());
    final m = (e as MessageEvent).message;
    expect(m.messageId, '01K');
    expect(m.data['type'], 'comment');
  });
}
