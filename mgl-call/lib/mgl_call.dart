/// Thin Flutter/WebRTC bridge for mgl-call.
///
/// Business state, signaling, and call lifecycle live in ClojureDart.
/// This layer only wraps PeerConnection / MediaStream / Helper APIs.
library;

export 'src/webrtc_peer.dart';
export 'src/media_helper.dart';
export 'src/permission_helper.dart';
