import Foundation
import UIKit
import UserNotifications

/// APNs provider (Phase 3).
final class ApnsProvider: NSObject {
  private var token: String?
  private var onEvent: (([String: Any?]) -> Void)?

  func initialize(onEvent: @escaping ([String: Any?]) -> Void) {
    self.onEvent = onEvent
    MglPushBridge.apnsProvider = self
    DispatchQueue.main.async {
      UIApplication.shared.registerForRemoteNotifications()
    }
  }

  func requestPermission(completion: @escaping (Bool) -> Void) {
    UNUserNotificationCenter.current().requestAuthorization(options: [.alert, .badge, .sound]) { granted, error in
      if let error = error {
        self.onEvent?([
          "type": "error",
          "provider": "apns",
          "code": "PERMISSION_ERROR",
          "message": error.localizedDescription
        ])
      }
      DispatchQueue.main.async {
        if granted {
          UIApplication.shared.registerForRemoteNotifications()
        }
        completion(granted)
      }
    }
  }

  func setDeviceToken(_ deviceToken: Data) {
    let token = deviceToken.map { String(format: "%02.2hhx", $0) }.joined()
    self.token = token
    onEvent?([
      "type": "token_changed",
      "provider": "apns",
      "token": token
    ])
  }

  func handleRegistrationError(_ error: Error) {
    onEvent?([
      "type": "error",
      "provider": "apns",
      "code": "TOKEN_ERROR",
      "message": error.localizedDescription
    ])
  }

  func getToken() -> String? { token }

  func unregister() {
    DispatchQueue.main.async {
      UIApplication.shared.unregisterForRemoteNotifications()
    }
    token = nil
  }
}

/// Bridge for AppDelegate / swizzle callbacks.
enum MglPushBridge {
  static weak var plugin: MglPushPlugin?
  static var apnsProvider: ApnsProvider?

  static func setDeviceToken(_ deviceToken: Data) {
    apnsProvider?.setDeviceToken(deviceToken)
  }

  static func handleRegistrationError(_ error: Error) {
    apnsProvider?.handleRegistrationError(error)
  }
}

/// ObjC-visible helpers for host AppDelegate (optional manual forwarding).
@objc public class MglPushAppDelegate: NSObject {
  @objc public static func didRegisterForRemoteNotifications(deviceToken: Data) {
    MglPushBridge.setDeviceToken(deviceToken)
  }

  @objc public static func didFailToRegisterForRemoteNotifications(error: Error) {
    MglPushBridge.handleRegistrationError(error)
  }
}
