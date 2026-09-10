class MglPushConfig {
  const MglPushConfig({
    required this.serverUrl,
    required this.serviceToken,
    required this.appId,
    this.registerOnInitialize = true,
  });

  /// Base URL of mgl-push-server, e.g. https://push.example.com
  final String serverUrl;

  /// Bearer service token (same as server MGL_PUSH_SERVICE_TOKENS).
  final String serviceToken;

  /// Application id, e.g. net.amjil.nomio
  final String appId;

  /// When true, [MglPush.initialize] also registers the device.
  final bool registerOnInitialize;
}
