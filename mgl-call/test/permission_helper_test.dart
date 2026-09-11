import 'package:flutter_test/flutter_test.dart';
import 'package:mgl_call/src/permission_helper.dart';
import 'package:permission_handler/permission_handler.dart';

void main() {
  group('permissionStatusString', () {
    test('maps granted to authorized', () {
      expect(permissionStatusString(PermissionStatus.granted), 'authorized');
    });

    test('maps limited to authorized', () {
      expect(permissionStatusString(PermissionStatus.limited), 'authorized');
    });

    test('maps denied to denied', () {
      expect(permissionStatusString(PermissionStatus.denied), 'denied');
    });

    test('maps restricted to restricted', () {
      expect(permissionStatusString(PermissionStatus.restricted), 'restricted');
    });

    test('maps permanentlyDenied to restricted', () {
      expect(
        permissionStatusString(PermissionStatus.permanentlyDenied),
        'restricted',
      );
    });
  });
}
