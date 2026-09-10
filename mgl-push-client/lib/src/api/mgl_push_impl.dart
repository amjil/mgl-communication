import 'dart:convert';

import 'package:device_info_plus/device_info_plus.dart';
import 'package:http/http.dart' as http;
import 'package:package_info_plus/package_info_plus.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:ulid/ulid.dart';

import '../channel/push_channel.dart';
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

  @override
  Stream<PushMessage> get messages => _channel.messages;

  @override
  Stream<PushEvent> get events => _channel.events;

  @override
  Future<void> initialize() async {
    if (_initialized) return;
    try {
      await _channel.invoke('initialize');
      // Drain cold-start notification if present.
      final initial = await _channel.invokeMap('getInitialNotification');
      if (initial != null && initial.isNotEmpty) {
        // Native also emits via EventChannel; getInitialNotification is a safety net.
      }
      _listenTokenRefresh();
      if (_config.registerOnInitialize) {
        // Non-blocking for callers who don't await deeply; still awaited here.
        try {
          await register();
        } catch (_) {
          // Push init must never crash the app.
        }
      }
      _initialized = true;
    } catch (_) {
      // Spec §54: initialization failure must not fail app startup.
      _initialized = true;
    }
  }

  void _listenTokenRefresh() {
    events.listen((event) async {
      if (event is TokenChangedEvent) {
        final device = _device ?? await getDevice();
        if (device == null) return;
        try {
          await _putJson(
            '/v1/devices/${device.installationId}/token',
            {'provider': event.provider, 'token': event.token},
          );
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
        } catch (_) {}
      }
    });
  }

  @override
  Future<PushDevice> register() async {
    final installationId = await _ensureInstallationId();
    final native = await _channel.invokeMap('register') ?? {};
    final meta = await _deviceMeta();

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
      // Phase 1 stub may return empty token on unsupported platforms (e.g. desktop).
      _device = device;
      return device;
    }

    final prefs = await SharedPreferences.getInstance();
    final userId = prefs.getString(_prefUserId);

    final body = {
      ...device.toJson(),
      if (userId != null && userId.isNotEmpty) 'user_id': userId,
    };
    await _postJson('/v1/devices', body);
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
