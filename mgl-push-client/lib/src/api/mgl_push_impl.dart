import 'dart:async';
import 'dart:convert';

import 'package:device_info_plus/device_info_plus.dart';
import 'package:http/http.dart' as http;
import 'package:package_info_plus/package_info_plus.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:ulid/ulid.dart';

import '../channel/push_channel.dart';
import '../models/push_capabilities.dart';
import '../models/push_device.dart';
import '../models/push_event.dart';
import '../models/push_message.dart';
import 'mgl_push.dart';
import 'mgl_push_config.dart';

const _prefInstallationId = 'mgl_push.installation_id';
const _prefUserId = 'mgl_push.user_id';

class MglPushImpl implements MglPush {
  MglPushImpl({
    required MglPushConfig config,
    PushChannel? channel,
    http.Client? httpClient,
  })  : _config = config,
        _channel = channel ?? PushChannel(),
        _http = httpClient ?? http.Client();

  final MglPushConfig _config;
  final PushChannel _channel;
  final http.Client _http;

  PushDevice? _device;
  bool _initialized = false;
  StreamSubscription<PushEvent>? _tokenRefreshSub;

  @override
  Stream<PushMessage> get messages => _channel.messages;

  @override
  Stream<PushEvent> get events => _channel.events;

  @override
  Future<PushDevice?> initialize() async {
    if (_initialized) {
      return _device ?? await getDevice();
    }
    await _channel.invoke('initialize');
    // Drain cold-start / pending native events into the event stream.
    final pending = await _channel.invoke('consumePendingEvents');
    if (pending is List) {
      for (final item in pending) {
        if (item is Map) {
          _channel.inject(parsePushEvent(Map<Object?, Object?>.from(item)));
        }
      }
    }
    final initial = await _channel.invokeMap('getInitialNotification');
    if (initial != null && initial.isNotEmpty) {
      _channel.inject(parsePushEvent(initial));
    }
    _listenTokenRefresh();
    if (_config.registerOnInitialize) {
      try {
        await register();
      } catch (e) {
        // Native init succeeded; surface register failure without blocking init.
        _channel.inject(ErrorEvent(
          code: 'REGISTER_FAILED',
          message: e.toString(),
        ));
      }
    }
    _initialized = true;
    return _device ?? await getDevice();
  }

  void _listenTokenRefresh() {
    if (_tokenRefreshSub != null) return;
    _tokenRefreshSub = events.listen((event) async {
      if (event is TokenChangedEvent) {
        final device = _device ?? await getDevice();
        if (device == null) return;
        try {
          await _putJson(
            '/v1/devices/${device.installationId}/token',
            {'provider': event.provider, 'token': event.token},
          );
          // Keep primary device as non-VoIP when VoIP token refreshes.
          if (event.provider != 'apns_voip') {
            _device = PushDevice(
              installationId: device.installationId,
              platform: device.platform,
              provider: event.provider,
              token: event.token,
              appId: device.appId,
              appVersion: device.appVersion,
              osVersion: device.osVersion,
              deviceModel: device.deviceModel,
              locale: device.locale,
              timezone: device.timezone,
            );
          }
        } catch (e) {
          _channel.inject(ErrorEvent(
            code: 'TOKEN_SYNC_FAILED',
            message: e.toString(),
            provider: event.provider,
          ));
        }
      }
    });
  }

  @override
  Future<PushDevice> register() async {
    final installationId = await _ensureInstallationId();
    final native = await _channel.invokeMap('register') ?? {};
    final meta = await _deviceMeta();
    final caps = await getCapabilities();

    final device = PushDevice(
      installationId: installationId,
      platform: (native['platform'] as String?) ?? meta.platform,
      provider: (native['provider'] as String?) ?? 'fcm',
      token: (native['token'] as String?) ?? '',
      appId: _config.appId,
      appVersion: meta.appVersion,
      osVersion: meta.osVersion,
      deviceModel: meta.deviceModel,
      locale: meta.locale,
      timezone: meta.timezone,
    );

    if (device.token.isEmpty) {
      _device = device;
      return device;
    }

    final prefs = await SharedPreferences.getInstance();
    final userId = prefs.getString(_prefUserId);

    final capabilities = [
      if (caps.notification) 'notification',
      if (caps.silentPush) 'silent',
      if (caps.backgroundPush) 'background',
      if (caps.incomingCallPush) 'incoming_call',
    ];

    final body = {
      ...device.toJson(),
      if (userId != null && userId.isNotEmpty) 'user_id': userId,
      'providers': caps.providers,
      'capabilities': capabilities,
    };
    await _postJson('/v1/devices', body);

    // iOS: register PushKit VoIP token as a separate apns_voip device row.
    final voipToken = native['voip_token'] as String?;
    if (device.platform == 'ios' &&
        voipToken != null &&
        voipToken.isNotEmpty) {
      await _postJson('/v1/devices', {
        ...body,
        'provider': 'apns_voip',
        'token': voipToken,
      });
    }

    _device = device;
    return device;
  }

  @override
  Future<PushDevice?> getDevice() async {
    if (_device != null) return _device;
    final native = await _channel.invokeMap('getDevice');
    if (native == null || native.isEmpty) return null;
    final installationId = await _ensureInstallationId();
    final meta = await _deviceMeta();
    _device = PushDevice(
      installationId: installationId,
      platform: (native['platform'] as String?) ?? meta.platform,
      provider: (native['provider'] as String?) ?? '',
      token: (native['token'] as String?) ?? '',
      appId: _config.appId,
      appVersion: meta.appVersion,
      osVersion: meta.osVersion,
      deviceModel: meta.deviceModel,
      locale: meta.locale,
      timezone: meta.timezone,
    );
    return _device;
  }

  @override
  Future<void> unregister() async {
    final device = await getDevice();
    await _channel.invoke('unregister');
    if (device != null) {
      await _delete('/v1/devices/${device.installationId}');
    }
    _device = null;
  }

  @override
  Future<void> setUserId(String userId) async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(_prefUserId, userId);
    await _channel.invoke('setUserId', {'userId': userId});
    final device = await getDevice();
    if (device != null) {
      await _putJson('/v1/devices/${device.installationId}/user', {'user_id': userId});
    }
  }

  @override
  Future<void> clearUserId() async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.remove(_prefUserId);
    await _channel.invoke('clearUserId');
    final device = await getDevice();
    if (device != null) {
      await _delete('/v1/devices/${device.installationId}/user');
    }
  }

  @override
  Future<void> requestPermission() async {
    await _channel.invoke('requestPermission');
  }

  @override
  Future<void> endSystemCall([String? callId]) async {
    await _channel.invoke('endSystemCall', {
      if (callId != null && callId.isNotEmpty) 'callId': callId,
    });
  }

  @override
  Future<PushCapabilities> getCapabilities() async {
    try {
      final map = await _channel.invokeMap('getCapabilities');
      if (map != null && map.isNotEmpty) {
        return PushCapabilities.fromMap(map);
      }
    } catch (_) {}
    // Sensible defaults when native method is unavailable.
    final device = _device ?? await getDevice();
    final platform = device?.platform ?? '';
    final provider = device?.provider ?? '';
    final providers = <String>[];
    if (provider.isNotEmpty) providers.add(provider);
    if (platform == 'ios') {
      if (!providers.contains('apns')) providers.add('apns');
      return PushCapabilities(
        notification: true,
        silentPush: true,
        backgroundPush: true,
        incomingCallPush: true,
        providers: providers.isEmpty ? ['apns', 'apns_voip'] : [...providers, 'apns_voip'],
      );
    }
    return PushCapabilities(
      notification: true,
      silentPush: true,
      backgroundPush: true,
      incomingCallPush: true,
      providers: providers.isEmpty ? ['fcm'] : providers,
    );
  }

  Future<String> _ensureInstallationId() async {
    final prefs = await SharedPreferences.getInstance();
    var id = prefs.getString(_prefInstallationId);
    if (id == null || id.isEmpty) {
      id = Ulid().toString();
      await prefs.setString(_prefInstallationId, id);
    }
    return id;
  }

  Future<_DeviceMeta> _deviceMeta() async {
    final info = await PackageInfo.fromPlatform();
    final deviceInfo = DeviceInfoPlugin();
    String platform = 'android';
    String osVersion = '';
    String deviceModel = '';
    try {
      final android = await deviceInfo.androidInfo;
      platform = 'android';
      osVersion = android.version.release;
      deviceModel = android.model;
    } catch (_) {
      try {
        final ios = await deviceInfo.iosInfo;
        platform = 'ios';
        osVersion = ios.systemVersion;
        deviceModel = ios.utsname.machine;
      } catch (_) {
        platform = 'unknown';
      }
    }
    return _DeviceMeta(
      platform: platform,
      appVersion: info.version,
      osVersion: osVersion,
      deviceModel: deviceModel,
      locale: null,
      timezone: DateTime.now().timeZoneName,
    );
  }

  Uri _uri(String path) {
    final base = _config.serverUrl.replaceAll(RegExp(r'/$'), '');
    return Uri.parse('$base$path');
  }

  Map<String, String> get _headers => {
        'Authorization': 'Bearer ${_config.serviceToken}',
        'Content-Type': 'application/json',
        'Accept': 'application/json',
      };

  Future<void> _postJson(String path, Map<String, dynamic> body) async {
    final res = await _http.post(_uri(path), headers: _headers, body: jsonEncode(body));
    _ensureOk(res);
  }

  Future<void> _putJson(String path, Map<String, dynamic> body) async {
    final res = await _http.put(_uri(path), headers: _headers, body: jsonEncode(body));
    _ensureOk(res);
  }

  Future<void> _delete(String path) async {
    final res = await _http.delete(_uri(path), headers: _headers);
    _ensureOk(res);
  }

  void _ensureOk(http.Response res) {
    if (res.statusCode >= 200 && res.statusCode < 300) return;
    throw StateError('mgl-push API ${res.statusCode}: ${res.body}');
  }
}

class _DeviceMeta {
  _DeviceMeta({
    required this.platform,
    required this.appVersion,
    required this.osVersion,
    required this.deviceModel,
    this.locale,
    this.timezone,
  });

  final String platform;
  final String appVersion;
  final String osVersion;
  final String deviceModel;
  final String? locale;
  final String? timezone;
}

/// Default factory used by apps.
MglPush createMglPush(MglPushConfig config) => MglPushImpl(config: config);
