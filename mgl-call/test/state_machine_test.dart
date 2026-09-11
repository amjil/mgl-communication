/// Pure Dart unit tests for call state machine rules mirrored from
/// `mgl.call.state/transitions`. Keep in sync with the ClojureDart source.
library;

import 'package:flutter_test/flutter_test.dart';

const transitions = <String, Set<String>>{
  'idle': {'creating', 'ringing'},
  'creating': {'ringing', 'connecting', 'cancelled', 'failed', 'ended'},
  'ringing': {'connecting', 'rejected', 'cancelled', 'failed', 'ended'},
  'connecting': {'connected', 'reconnecting', 'failed', 'ended', 'cancelled'},
  'connected': {'reconnecting', 'ended', 'failed'},
  'reconnecting': {'connected', 'failed', 'ended'},
  'ended': {},
  'failed': {},
  'rejected': {},
  'cancelled': {},
};

bool canTransition(String from, String to) =>
    transitions[from]?.contains(to) ?? false;

void main() {
  group('call state machine', () {
    test('outgoing happy path', () {
      expect(canTransition('idle', 'creating'), isTrue);
      expect(canTransition('creating', 'ringing'), isTrue);
      expect(canTransition('ringing', 'connecting'), isTrue);
      expect(canTransition('connecting', 'connected'), isTrue);
      expect(canTransition('connected', 'ended'), isTrue);
    });

    test('incoming happy path', () {
      expect(canTransition('idle', 'ringing'), isTrue);
      expect(canTransition('ringing', 'connecting'), isTrue);
      expect(canTransition('connecting', 'connected'), isTrue);
    });

    test('reject / cancel from ringing', () {
      expect(canTransition('ringing', 'rejected'), isTrue);
      expect(canTransition('ringing', 'cancelled'), isTrue);
    });

    test('failure from connecting', () {
      expect(canTransition('connecting', 'failed'), isTrue);
    });

    test('reconnect cycle', () {
      expect(canTransition('connected', 'reconnecting'), isTrue);
      expect(canTransition('reconnecting', 'connected'), isTrue);
      expect(canTransition('reconnecting', 'failed'), isTrue);
    });

    test('illegal transitions', () {
      expect(canTransition('idle', 'connected'), isFalse);
      expect(canTransition('ended', 'connected'), isFalse);
      expect(canTransition('rejected', 'ringing'), isFalse);
      expect(canTransition('connected', 'ringing'), isFalse);
    });

    test('terminal states have no outgoing edges', () {
      for (final t in ['ended', 'failed', 'rejected', 'cancelled']) {
        expect(transitions[t], isEmpty);
      }
    });
  });
}
