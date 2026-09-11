import Flutter
import PushKit
import UIKit
import UserNotifications
import ObjectiveC
import CallKit

public class MglPushPlugin: NSObject, FlutterPlugin, FlutterStreamHandler, UNUserNotificationCenterDelegate, PKPushRegistryDelegate {
  private var methodChannel: FlutterMethodChannel?
  private var eventChannel: FlutterEventChannel?
  private var eventSink: FlutterEventSink?
  private var pendingEvents: [[String: Any?]] = []
  private var apnsProvider = ApnsProvider()
  private var initialNotification: [String: Any?]?
  private var voipRegistry: PKPushRegistry?
  private var voipToken: String?
  private let pendingStore = PendingEventStore()
  private let incomingCalls = IncomingCallManager.shared

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
      incomingCalls.onEvent = { [weak self] event in
        self?.emit(event)
      }
      setupPushKit()
      result(nil)
    case "register":
      let token = apnsProvider.getToken() ?? ""
      result([
        "platform": "ios",
        "provider": "apns",
        "token": token,
        "voip_token": voipToken as Any
      ])
    case "getDevice":
      result([
        "platform": "ios",
        "provider": "apns",
        "token": apnsProvider.getToken() ?? "",
        "voip_token": voipToken as Any
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
    case "consumePendingEvents":
      var all = pendingStore.consume()
      all.append(contentsOf: pendingEvents)
      pendingEvents.removeAll()
      result(all)
    case "getCapabilities":
      result([
        "notification": true,
        "silent_push": true,
        "background_push": true,
        "incoming_call_push": true,
        "callkit": incomingCalls.callKitAvailable,
        "live_communication_kit": incomingCalls.liveCommunicationKitAvailable,
        "telecom": false,
        "providers": ["apns", "apns_voip"]
      ])
    case "endSystemCall":
      let args = call.arguments as? [String: Any]
      let callId = args?["callId"] as? String ?? args?["call_id"] as? String
      if let callId, !callId.isEmpty {
        incomingCalls.endCall(callId: callId, reason: .remoteEnded)
      } else {
        incomingCalls.endAll(reason: .remoteEnded)
      }
      result(nil)
    default:
      result(FlutterMethodNotImplemented)
    }
  }

  private func setupPushKit() {
    let registry = PKPushRegistry(queue: DispatchQueue.main)
    registry.delegate = self
    registry.desiredPushTypes = [.voIP]
    voipRegistry = registry
  }

  // MARK: - PKPushRegistryDelegate

  public func pushRegistry(_ registry: PKPushRegistry, didUpdate pushCredentials: PKPushCredentials, for type: PKPushType) {
    guard type == .voIP else { return }
    let token = pushCredentials.token.map { String(format: "%02x", $0) }.joined()
    voipToken = token
    emit([
      "type": "token_changed",
      "provider": "apns_voip",
      "token": token
    ])
  }

  public func pushRegistry(_ registry: PKPushRegistry, didInvalidatePushTokenFor type: PKPushType) {
    if type == .voIP {
      voipToken = nil
    }
  }

  public func pushRegistry(
    _ registry: PKPushRegistry,
    didReceiveIncomingPushWith payload: PKPushPayload,
    for type: PKPushType,
    completion: @escaping () -> Void
  ) {
    guard type == .voIP else {
      completion()
      return
    }

    let event = domainEvent(from: payload.dictionaryPayload, defaultType: "incoming-call", provider: "apns_voip")
    let eventType = event["type"] as? String ?? "incoming-call"
    let data = (event["data"] as? [String: String]) ?? [:]
    let eventId = (event["id"] as? String) ?? ""

    switch eventType {
    case "call-cancelled", "call-ended":
      // Apple requires every VoIP push to report a CallKit call. If a cancel/end
      // arrives on apns_voip (e.g. race with hangup), flash-report then hang up.
      let callId = data["call_id"] ?? data["callId"] ?? eventId
      incomingCalls.reportIncoming(
        data: data,
        eventId: eventId,
        providerName: "apns_voip"
      ) { [weak self] in
        let reason: CXCallEndedReason = eventType == "call-cancelled" ? .unanswered : .remoteEnded
        self?.incomingCalls.endCall(callId: callId, reason: reason)
        self?.emit(event)
        completion()
      }
    default:
      // Call Control: report System Call UI before PushKit completion (Apple requirement).
      emit(event)
      incomingCalls.reportIncoming(
        data: data,
        eventId: eventId,
        providerName: "apns_voip",
        completion: completion
      )
    }
  }

  private func emit(_ event: [String: Any?]) {
    if let type = event["type"] as? String,
       type == "call-cancelled" || type == "call-ended",
       let data = event["data"] as? [String: String],
       let callId = data["call_id"] ?? data["callId"] {
      let reason: CXCallEndedReason = type == "call-cancelled" ? .unanswered : .remoteEnded
      incomingCalls.endCall(callId: callId, reason: reason)
    }

    pendingStore.save(event)
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
    emit(domainEvent(from: notification.request.content.userInfo, defaultType: "notification", provider: "apns"))
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

  @objc public func handleRemoteNotification(_ userInfo: [AnyHashable: Any], fetchCompletionHandler completionHandler: @escaping (UIBackgroundFetchResult) -> Void) {
    emit(domainEvent(from: userInfo, defaultType: "silent", provider: "apns"))
    completionHandler(.newData)
  }

  private func domainEvent(from userInfo: [AnyHashable: Any], defaultType: String, provider: String) -> [String: Any?] {
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
    var mutable = data
    if let title { mutable["title"] = title }
    if let body { mutable["body"] = body }

    let rawType = mutable["mgl_event_type"] ?? ""
    let eventType = normalizeEventType(rawType.isEmpty ? defaultType : rawType)
    let id = mutable["mgl_event_id"] ?? mutable["mgl_message_id"] ?? mutable["event_id"] ?? ""
    let ts = Int64(mutable["mgl_timestamp"] ?? "") ?? Int64(Date().timeIntervalSince1970)

    return [
      "version": 1,
      "type": eventType,
      "id": id,
      "timestamp": ts,
      "provider": provider,
      "data": mutable
    ]
  }

  private func normalizeEventType(_ raw: String) -> String {
    switch raw {
    case "incoming_call", "incoming-call": return "incoming-call"
    case "call_cancelled", "call-cancelled": return "call-cancelled"
    case "call_ended", "call-ended": return "call-ended"
    case "message": return "notification"
    default: return raw.isEmpty ? "notification" : raw
    }
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

/// Short-lived pending queue for cold start (spec §38–39). TTL 30–120s.
final class PendingEventStore {
  private let key = "mgl_push.pending_events"
  private let ttl: TimeInterval = 120

  func save(_ event: [String: Any?]) {
    guard let type = event["type"] as? String else { return }
    let keep = ["incoming-call", "call-cancelled", "call-ended", "silent", "background", "notification"]
    guard keep.contains(type) else { return }
    var items = loadRaw()
    let payload: [String: Any] = [
      "received_at": Date().timeIntervalSince1970,
      "event": event.compactMapValues { $0 }
    ]
    items.append(payload)
    UserDefaults.standard.set(items, forKey: key)
  }

  func consume() -> [[String: Any?]] {
    let now = Date().timeIntervalSince1970
    let items = loadRaw()
    UserDefaults.standard.removeObject(forKey: key)
    return items.compactMap { item in
      guard let receivedAt = item["received_at"] as? TimeInterval,
            now - receivedAt <= ttl,
            let event = item["event"] as? [String: Any] else { return nil }
      return event.mapValues { $0 as Any? }
    }
  }

  private func loadRaw() -> [[String: Any]] {
    (UserDefaults.standard.array(forKey: key) as? [[String: Any]]) ?? []
  }
}

// MARK: - AppDelegate swizzle

enum AppDelegateSwizzler {
  private static var didSwizzle = false
  private static var originalDidRegister: IMP?
  private static var originalDidFail: IMP?
  private static var originalDidReceiveRemote: IMP?

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
    let remoteSel = #selector(UIApplicationDelegate.application(_:didReceiveRemoteNotification:fetchCompletionHandler:))

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
    let remoteBlock: @convention(block) (AnyObject, UIApplication, [AnyHashable: Any], @escaping (UIBackgroundFetchResult) -> Void) -> Void = { target, app, userInfo, completion in
      MglPushBridge.plugin?.handleRemoteNotification(userInfo, fetchCompletionHandler: completion)
      if let imp = AppDelegateSwizzler.originalDidReceiveRemote {
        typealias Fn = @convention(c) (AnyObject, Selector, UIApplication, [AnyHashable: Any], @escaping (UIBackgroundFetchResult) -> Void) -> Void
        unsafeBitCast(imp, to: Fn.self)(target, remoteSel, app, userInfo, completion)
      }
    }

    replace(cls, selector: regSel, block: regBlock, store: &originalDidRegister)
    replace(cls, selector: failSel, block: failBlock, store: &originalDidFail)
    replace(cls, selector: remoteSel, block: remoteBlock, store: &originalDidReceiveRemote)
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
