class PushDevice {
  const PushDevice({
    required this.installationId,
    required this.platform,
    required this.provider,
    required this.token,
    required this.appId,
    required this.appVersion,
    required this.osVersion,
    required this.deviceModel,
    this.locale,
    this.timezone,
  });

  final String installationId;
  final String platform;
  final String provider;
  final String token;
  final String appId;
  final String appVersion;
  final String osVersion;
  final String deviceModel;
  final String? locale;
  final String? timezone;

  factory PushDevice.fromMap(Map<Object?, Object?> map) {
    return PushDevice(
      installationId: map['installationId'] as String? ?? map['installation_id'] as String? ?? '',
      platform: map['platform'] as String? ?? '',
      provider: map['provider'] as String? ?? '',
      token: map['token'] as String? ?? '',
      appId: map['appId'] as String? ?? map['app_id'] as String? ?? '',
      appVersion: map['appVersion'] as String? ?? map['app_version'] as String? ?? '',
      osVersion: map['osVersion'] as String? ?? map['os_version'] as String? ?? '',
      deviceModel: map['deviceModel'] as String? ?? map['device_model'] as String? ?? '',
      locale: map['locale'] as String?,
      timezone: map['timezone'] as String?,
    );
  }

  Map<String, dynamic> toJson() => {
        'installation_id': installationId,
        'platform': platform,
        'provider': provider,
        'token': token,
        'app_id': appId,
        'app_version': appVersion,
        'os_version': osVersion,
        'device_model': deviceModel,
        if (locale != null) 'locale': locale,
        if (timezone != null) 'timezone': timezone,
      };
}
