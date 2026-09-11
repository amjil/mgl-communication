import 'package:flutter_test/flutter_test.dart';
import 'package:mgl_push/src/events/event_buffer.dart';
import 'package:mgl_push/src/events/event_deduper.dart';
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

  test('parsePushEvent legacy message → domain notification', () {
    final e = parsePushEvent({
      'type': 'message',
      'message_id': '01K',
      'title': 't',
      'body': 'b',
      'data': {'type': 'comment'},
    });
    expect(e, isA<DomainPushEvent>());
    final d = e as DomainPushEvent;
    expect(d.id, '01K');
    expect(d.type, PushEventType.notification);
    expect(d.data['type'], 'comment');
  });

  test('parsePushEvent incoming-call', () {
    final e = parsePushEvent({
      'version': 1,
      'type': 'incoming-call',
      'id': 'evt_1',
      'timestamp': 1757500000,
      'data': {
        'call_id': 'call_1',
        'caller_id': 'a',
        'callee_id': 'b',
        'media_type': 'video',
        'expires_at': '9999999999',
      },
    });
    expect(e, isA<DomainPushEvent>());
    final d = e as DomainPushEvent;
    expect(d.isIncomingCall, isTrue);
    expect(d.callId, 'call_1');
  });

  test('EventDeduper drops duplicate event_id and call_id', () {
    final d = EventDeduper();
    expect(d.accept(eventId: 'e1', callId: 'c1', eventType: 'incoming-call'), isTrue);
    expect(d.accept(eventId: 'e1', callId: 'c1', eventType: 'incoming-call'), isFalse);
    expect(d.accept(eventId: 'e2', callId: 'c1', eventType: 'incoming-call'), isFalse);
    expect(d.accept(eventId: 'e3', callId: 'c2', eventType: 'incoming-call'), isTrue);
  });

  test('EventBuffer drops expired incoming-call', () async {
    final buffer = EventBuffer();
    buffer.add(DomainPushEvent(
      id: 'evt_old',
      type: PushEventType.incomingCall,
      timestamp: 1,
      data: {'call_id': 'c', 'expires_at': '1'},
    ));
    final events = <PushEvent>[];
    final sub = buffer.stream.listen(events.add);
    await Future<void>.delayed(Duration.zero);
    expect(events, isEmpty);
    await sub.cancel();
    await buffer.close();
  });

  test('EventBuffer re-buffers after last listener cancels', () async {
    final buffer = EventBuffer();
    final first = <PushEvent>[];
    final sub = buffer.stream.listen(first.add);
    await Future<void>.delayed(Duration.zero);

    buffer.add(const TokenChangedEvent(provider: 'fcm', token: 't1'));
    await Future<void>.delayed(Duration.zero);
    expect(first, hasLength(1));

    await sub.cancel();
    await Future<void>.delayed(Duration.zero);

    // No listeners: must buffer, not drop.
    buffer.add(const TokenChangedEvent(provider: 'fcm', token: 't2'));

    final second = <PushEvent>[];
    final sub2 = buffer.stream.listen(second.add);
    await Future<void>.delayed(Duration.zero);

    expect(second, hasLength(1));
    expect((second.first as TokenChangedEvent).token, 't2');
    await sub2.cancel();
    await buffer.close();
  });
}
