import 'package:flutter_webrtc/flutter_webrtc.dart';

/// Local media helpers used by ClojureDart.
class MediaHelper {
  MediaHelper._();

  static Future<MediaStream> getUserMedia({
    required bool audio,
    required bool video,
    String facingMode = 'user',
  }) {
    final constraints = <String, dynamic>{
      'audio': audio,
      if (video)
        'video': {
          'facingMode': facingMode,
        }
      else
        'video': false,
    };
    return navigator.mediaDevices.getUserMedia(constraints);
  }

  static Future<void> setSpeakerphoneOn(bool enabled) {
    return Helper.setSpeakerphoneOn(enabled);
  }

  static Future<bool> switchCamera(MediaStreamTrack track) {
    return Helper.switchCamera(track);
  }

  static void setTrackEnabled(MediaStreamTrack track, bool enabled) {
    track.enabled = enabled;
  }

  static Future<void> stopStream(MediaStream? stream) async {
    if (stream == null) return;
    for (final track in stream.getTracks()) {
      await track.stop();
    }
    await stream.dispose();
  }
}
