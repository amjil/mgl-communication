import 'package:permission_handler/permission_handler.dart' as ph;

/// Maps platform permission status to mgl-call keywords (as strings).
String permissionStatusString(ph.PermissionStatus status) {
  if (status.isGranted || status.isLimited) return 'authorized';
  if (status.isDenied) return 'denied';
  if (status.isRestricted || status.isPermanentlyDenied) return 'restricted';
  return 'unknown';
}

Future<String> microphonePermission() async {
  return permissionStatusString(await ph.Permission.microphone.status);
}

Future<String> cameraPermission() async {
  return permissionStatusString(await ph.Permission.camera.status);
}

Future<String> requestMicrophonePermission() async {
  return permissionStatusString(await ph.Permission.microphone.request());
}

Future<String> requestCameraPermission() async {
  return permissionStatusString(await ph.Permission.camera.request());
}
