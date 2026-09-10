import Flutter
import UIKit
import UserNotifications
import ObjectiveC

public class MglPushPlugin: NSObject, FlutterPlugin, FlutterStreamHandler, UNUserNotificationCenterDelegate {
  private var methodChannel: FlutterMethodChannel?
  private var eventChannel: FlutterEventChannel?
  private var eventSink: FlutterEventSink?
  private var pendingEvents: [[String: Any?]] = []
  private var apnsProvider = ApnsProvider()
  private var initialNotification: [String: Any?]?

  public static func register(with registrar: FlutterPluginRegistrar) {
    let instance = MglPushPlugin()
    let messenger = registrar.messenger()

    let methods = FlutterMethodChannel(name: "net.amjil.mgl_push/methods", binaryMessenger: messenger)
    methods.setMethodCallHandler(instance.handle)
    instance.methodChannel = methods

    let events = FlutterEventChannel(name: "net.amjil.mgl_push/events", binaryMessenger: messenger)
    events.setStreamHandler(instance)
    instance.eventChannel = events

    UNUserNotificationCenter.current().delegate = instance
    MglPushBridge.plugin = instance
    AppDelegateSwizzler.swizzleIfNeeded()
  }

  public func handle(_ call: FlutterMethodCall, result: @escaping FlutterResult) {
    switch call.method {
    case "initialize":
      apnsProvider.initialize { [weak self] event in
        self?.emit(event)
      }
      result(nil)
    case "register":
      result([
        "platform": "ios",
        "provider": "apns",
        "token": apnsProvider.getToken() ?? ""
      ])
    case "getDevice":
      result([
        "platform": "ios",
        "provider": "apns",
        "token": apnsProvider.getToken() ?? ""
      ])
    case "unregister":
      apnsProvider.unregister()
      result(nil)
    case "setUserId", "clearUserId":
      result(nil)
    case "requestPermission":
      apnsProvider.requestPermission { granted in
        result(granted)
      }
    case "getInitialNotification":
      let n = initialNotification
      initialNotification = nil
      result(n)
    default:
      result(FlutterMethodNotImplemented)
    }
  }

  private func emit(_ event: [String: Any?]) {
    if let sink = eventSink {
      sink(event)
    } else {
      pendingEvents.append(event)
      if event["type"] as? String == "notification_open" {
        initialNotification = event
      }
    }
  }

  public func onListen(withArguments arguments: Any?, eventSink events: @escaping FlutterEventSink) -> FlutterError? {
    eventSink = events
    pendingEvents.forEach { events($0) }
    pendingEvents.removeAll()
    return nil
  }

  public func onCancel(withArguments arguments: Any?) -> FlutterError? {
    eventSink = nil
    return nil
  }

  // MARK: - UNUserNotificationCenterDelegate

  public func userNotificationCenter(
    _ center: UNUserNotificationCenter,
    willPresent notification: UNNotification,
    withCompletionHandler completionHandler: @escaping (UNNotificationPresentationOptions) -> Void
  ) {
    emit(messageEvent(from: notification.request.content.userInfo))
    // Foreground: Flutter decides whether to show a local notification.
    completionHandler([])
  }

  public func userNotificationCenter(
    _ center: UNUserNotificationCenter,
    didReceive response: UNNotificationResponse,
    withCompletionHandler completionHandler: @escaping () -> Void
  ) {
    let data = stringify(response.notification.request.content.userInfo)
    emit([
      "type": "notification_open",
      "message_id": data["mgl_message_id"] ?? data["message_id"],
      "provider": "apns",
      "data": data,
      "deep_link": data["deep_link"]
    ])
    completionHandler()
  }

  private func messageEvent(from userInfo: [AnyHashable: Any]) -> [String: Any?] {
    let data = stringify(userInfo)
    let aps = userInfo["aps"] as? [String: Any]
    var title: String?
    var body: String?
    if let alert = aps?["alert"] as? [String: Any] {
      title = alert["title"] as? String
      body = alert["body"] as? String
    } else if let alert = aps?["alert"] as? String {
      body = alert
    }
    return [
      "type": "message",
      "message_id": data["mgl_message_id"] ?? data["message_id"],
      "provider": "apns",
      "title": title,
      "body": body,
      "data": data,
      "deep_link": data["deep_link"]
    ]
  }

  private func stringify(_ userInfo: [AnyHashable: Any]) -> [String: String] {
    var out: [String: String] = [:]
    for (k, v) in userInfo {
      if let key = k as? String, key != "aps" {
        out[key] = "\(v)"
      }
    }
    return out
  }
}

// MARK: - AppDelegate swizzle

enum AppDelegateSwizzler {
  private static var didSwizzle = false
  private static var originalDidRegister: IMP?
  private static var originalDidFail: IMP?

  static func swizzleIfNeeded() {
    guard !didSwizzle else { return }
    guard let appDelegate = UIApplication.shared.delegate else {
      DispatchQueue.main.asyncAfter(deadline: .now() + 0.1) { swizzleIfNeeded() }
      return
    }
    didSwizzle = true
    let cls: AnyClass = type(of: appDelegate)

    let regSel = #selector(UIApplicationDelegate.application(_:didRegisterForRemoteNotificationsWithDeviceToken:))
    let failSel = #selector(UIApplicationDelegate.application(_:didFailToRegisterForRemoteNotificationsWithError:))

    let regBlock: @convention(block) (AnyObject, UIApplication, Data) -> Void = { target, app, token in
      MglPushBridge.setDeviceToken(token)
      if let imp = AppDelegateSwizzler.originalDidRegister {
        typealias Fn = @convention(c) (AnyObject, Selector, UIApplication, Data) -> Void
        unsafeBitCast(imp, to: Fn.self)(target, regSel, app, token)
      }
    }
    let failBlock: @convention(block) (AnyObject, UIApplication, Error) -> Void = { target, app, error in
      MglPushBridge.handleRegistrationError(error)
      if let imp = AppDelegateSwizzler.originalDidFail {
        typealias Fn = @convention(c) (AnyObject, Selector, UIApplication, Error) -> Void
        unsafeBitCast(imp, to: Fn.self)(target, failSel, app, error)
      }
    }

    replace(cls, selector: regSel, block: regBlock, store: &originalDidRegister)
    replace(cls, selector: failSel, block: failBlock, store: &originalDidFail)
  }

  private static func replace(_ cls: AnyClass, selector: Selector, block: Any, store: inout IMP?) {
    let imp = imp_implementationWithBlock(block)
    if let method = class_getInstanceMethod(cls, selector) {
      store = method_getImplementation(method)
      method_setImplementation(method, imp)
    } else {
      class_addMethod(cls, selector, imp, "v@:@@")
    }
  }
}
