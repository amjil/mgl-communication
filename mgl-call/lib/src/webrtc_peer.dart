import 'dart:async';

import 'package:flutter_webrtc/flutter_webrtc.dart';

/// Callback-driven PeerConnection wrapper for ClojureDart interop.
class WebrtcPeer {
  WebrtcPeer({
    this.onIceCandidate,
    this.onIceConnectionState,
    this.onConnectionState,
    this.onSignalingState,
    this.onTrack,
    this.onError,
  });

  RTCPeerConnection? _pc;
  MediaStream? _localStream;
  MediaStream? _remoteStream;

  void Function(Map<String, dynamic> candidate)? onIceCandidate;
  void Function(String state)? onIceConnectionState;
  void Function(String state)? onConnectionState;
  void Function(String state)? onSignalingState;
  void Function(MediaStream stream, MediaStreamTrack track)? onTrack;
  void Function(Object error)? onError;

  RTCPeerConnection? get peerConnection => _pc;
  MediaStream? get localStream => _localStream;
  MediaStream? get remoteStream => _remoteStream;

  Future<void> create({
    required List<Map<String, dynamic>> iceServers,
  }) async {
    final config = <String, dynamic>{
      'iceServers': iceServers,
      'sdpSemantics': 'unified-plan',
    };
    final constraints = <String, dynamic>{
      'mandatory': {},
      'optional': [
        {'DtlsSrtpKeyAgreement': true},
      ],
    };

    final pc = await createPeerConnection(config, constraints);
    _pc = pc;

    pc.onIceCandidate = (RTCIceCandidate? candidate) {
      if (candidate == null || candidate.candidate == null) return;
      onIceCandidate?.call({
        'candidate': candidate.candidate,
        'sdpMid': candidate.sdpMid,
        'sdpMLineIndex': candidate.sdpMLineIndex,
      });
    };

    pc.onIceConnectionState = (RTCIceConnectionState state) {
      onIceConnectionState?.call(_iceConnectionStateName(state));
    };

    pc.onConnectionState = (RTCPeerConnectionState state) {
      onConnectionState?.call(_connectionStateName(state));
    };

    pc.onSignalingState = (RTCSignalingState state) {
      onSignalingState?.call(_signalingStateName(state));
    };

    pc.onTrack = (RTCTrackEvent event) {
      final track = event.track;
      if (event.streams.isNotEmpty) {
        _remoteStream = event.streams.first;
        onTrack?.call(_remoteStream!, track);
      } else {
        createLocalMediaStream('remote').then((stream) {
          stream.addTrack(track);
          _remoteStream = stream;
          onTrack?.call(stream, track);
        });
      }
    };
  }

  Future<void> setLocalStream(MediaStream stream) async {
    _localStream = stream;
    final pc = _requirePc();
    for (final track in stream.getTracks()) {
      await pc.addTrack(track, stream);
    }
  }

  Future<Map<String, dynamic>> createOffer([
    Map<String, dynamic>? constraints,
  ]) async {
    final pc = _requirePc();
    final offer = await pc.createOffer(constraints ?? <String, dynamic>{});
    await pc.setLocalDescription(offer);
    return {
      'type': offer.type,
      'sdp': offer.sdp,
    };
  }

  Future<Map<String, dynamic>> createAnswer([
    Map<String, dynamic>? constraints,
  ]) async {
    final pc = _requirePc();
    final answer = await pc.createAnswer(constraints ?? <String, dynamic>{});
    await pc.setLocalDescription(answer);
    return {
      'type': answer.type,
      'sdp': answer.sdp,
    };
  }

  Future<void> setRemoteDescription(String type, String sdp) async {
    final pc = _requirePc();
    await pc.setRemoteDescription(RTCSessionDescription(sdp, type));
  }

  Future<void> addIceCandidate(Map<String, dynamic> candidate) async {
    final pc = _requirePc();
    await pc.addCandidate(
      RTCIceCandidate(
        candidate['candidate'] as String?,
        candidate['sdpMid'] as String?,
        candidate['sdpMLineIndex'] as int?,
      ),
    );
  }

  Future<List<StatsReport>> getStats() async {
    final pc = _pc;
    if (pc == null) return const [];
    return pc.getStats();
  }

  String? get connectionState {
    final pc = _pc;
    if (pc == null) return null;
    return _connectionStateName(pc.connectionState);
  }

  String? get iceConnectionState {
    final pc = _pc;
    if (pc == null) return null;
    return _iceConnectionStateName(pc.iceConnectionState);
  }

  String? get signalingState {
    final pc = _pc;
    if (pc == null) return null;
    return _signalingStateName(pc.signalingState);
  }

  Future<void> close() async {
    try {
      if (_localStream != null) {
        for (final track in _localStream!.getTracks()) {
          await track.stop();
        }
        await _localStream!.dispose();
      }
    } catch (_) {}
    _localStream = null;

    try {
      if (_remoteStream != null) {
        for (final track in _remoteStream!.getTracks()) {
          await track.stop();
        }
        await _remoteStream!.dispose();
      }
    } catch (_) {}
    _remoteStream = null;

    try {
      await _pc?.close();
      await _pc?.dispose();
    } catch (_) {}
    _pc = null;
  }

  RTCPeerConnection _requirePc() {
    final pc = _pc;
    if (pc == null) {
      throw StateError('PeerConnection not created');
    }
    return pc;
  }

  static String _connectionStateName(RTCPeerConnectionState? state) {
    switch (state) {
      case RTCPeerConnectionState.RTCPeerConnectionStateNew:
        return 'new';
      case RTCPeerConnectionState.RTCPeerConnectionStateConnecting:
        return 'connecting';
      case RTCPeerConnectionState.RTCPeerConnectionStateConnected:
        return 'connected';
      case RTCPeerConnectionState.RTCPeerConnectionStateDisconnected:
        return 'disconnected';
      case RTCPeerConnectionState.RTCPeerConnectionStateFailed:
        return 'failed';
      case RTCPeerConnectionState.RTCPeerConnectionStateClosed:
        return 'closed';
      default:
        return 'unknown';
    }
  }

  static String _iceConnectionStateName(RTCIceConnectionState? state) {
    switch (state) {
      case RTCIceConnectionState.RTCIceConnectionStateNew:
        return 'new';
      case RTCIceConnectionState.RTCIceConnectionStateChecking:
        return 'checking';
      case RTCIceConnectionState.RTCIceConnectionStateConnected:
        return 'connected';
      case RTCIceConnectionState.RTCIceConnectionStateCompleted:
        return 'completed';
      case RTCIceConnectionState.RTCIceConnectionStateFailed:
        return 'failed';
      case RTCIceConnectionState.RTCIceConnectionStateDisconnected:
        return 'disconnected';
      case RTCIceConnectionState.RTCIceConnectionStateClosed:
        return 'closed';
      case RTCIceConnectionState.RTCIceConnectionStateCount:
        return 'count';
      default:
        return 'unknown';
    }
  }

  static String _signalingStateName(RTCSignalingState? state) {
    switch (state) {
      case RTCSignalingState.RTCSignalingStateStable:
        return 'stable';
      case RTCSignalingState.RTCSignalingStateHaveLocalOffer:
        return 'have-local-offer';
      case RTCSignalingState.RTCSignalingStateHaveRemoteOffer:
        return 'have-remote-offer';
      case RTCSignalingState.RTCSignalingStateHaveLocalPrAnswer:
        return 'have-local-pranswer';
      case RTCSignalingState.RTCSignalingStateHaveRemotePrAnswer:
        return 'have-remote-pranswer';
      case RTCSignalingState.RTCSignalingStateClosed:
        return 'closed';
      default:
        return 'unknown';
    }
  }
}
