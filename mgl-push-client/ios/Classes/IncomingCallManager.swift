import AVFoundation
import CallKit
import Foundation
import UIKit

/// System Call Control for Incoming Call (spec: LCK preferred, CallKit fallback).
/// CallKit remains the PushKit-compliant path when LCK is unavailable or fails.
final class IncomingCallManager: NSObject, CXProviderDelegate {
  static let shared = IncomingCallManager()

  private let provider: CXProvider
  private let callController = CXCallController()
  private var callIdToUUID: [String: UUID] = [:]
  private var uuidToPayload: [UUID: [String: String]] = [:]
  private var uuidToEventMeta: [UUID: (eventId: String, provider: String)] = [:]
  private var answeredUUIDs: Set<UUID> = []
  /// callIds currently owned by LiveCommunicationKit (skip CallKit end for these).
  private var lckOwnedCallIds: Set<String> = []

  /// Emits DomainPushEvent maps (incoming-call with action, etc.).
  var onEvent: (([String: Any?]) -> Void)? {
    didSet {
      #if canImport(LiveCommunicationKit)
      if #available(iOS 17.4, *) {
        LiveCommunicationController.shared.onEvent = onEvent
      }
      #endif
    }
  }

  private override init() {
    let config = CXProviderConfiguration(localizedName: IncomingCallManager.appDisplayName())
    config.supportsVideo = true
    config.maximumCallsPerCallGroup = 1
    config.maximumCallGroups = 1
    config.supportedHandleTypes = [.generic]
    if #available(iOS 14.0, *) {
      config.includesCallsInRecents = true
    }
    provider = CXProvider(configuration: config)
    super.init()
    provider.setDelegate(self, queue: DispatchQueue.main)
  }

  var callKitAvailable: Bool { true }

  /// LiveCommunicationKit is preferred when available; CallKit remains the VoIP-compliant fallback.
  var liveCommunicationKitAvailable: Bool {
    #if canImport(LiveCommunicationKit)
    if #available(iOS 17.4, *) {
      return LiveCommunicationController.shared.isReady
    }
    #endif
    return false
  }

  // MARK: - Public API

  /// Report system incoming call from VoIP Push. Must finish before PushKit completion.
  func reportIncoming(
    data: [String: String],
    eventId: String,
    providerName: String,
    completion: @escaping () -> Void
  ) {
    let callId = data["call_id"] ?? data["callId"] ?? eventId
    guard !callId.isEmpty else {
      completion()
      return
    }

    if callIdToUUID[callId] != nil || lckOwnedCallIds.contains(callId) {
      completion()
      return
    }

    #if canImport(LiveCommunicationKit)
    if #available(iOS 17.4, *), liveCommunicationKitAvailable {
      LiveCommunicationController.shared.reportIncoming(
        data: data,
        eventId: eventId,
        providerName: providerName
      ) { [weak self] ok in
        if ok {
          self?.lckOwnedCallIds.insert(callId)
          completion()
        } else {
          self?.reportIncomingCallKit(
            data: data,
            eventId: eventId,
            providerName: providerName,
            callId: callId,
            completion: completion
          )
        }
      }
      return
    }
    #endif

    reportIncomingCallKit(
      data: data,
      eventId: eventId,
      providerName: providerName,
      callId: callId,
      completion: completion
    )
  }

  func endCall(callId: String, reason: CXCallEndedReason = .remoteEnded) {
    if lckOwnedCallIds.contains(callId) {
      #if canImport(LiveCommunicationKit)
      if #available(iOS 17.4, *) {
        LiveCommunicationController.shared.endCall(callId: callId)
      }
      #endif
      lckOwnedCallIds.remove(callId)
      return
    }
    guard let uuid = callIdToUUID[callId] else { return }
    provider.reportCall(with: uuid, endedAt: Date(), reason: reason)
    cleanup(uuid: uuid)
  }

  func endAll(reason: CXCallEndedReason = .remoteEnded) {
    #if canImport(LiveCommunicationKit)
    if #available(iOS 17.4, *) {
      LiveCommunicationController.shared.endAll()
    }
    #endif
    lckOwnedCallIds.removeAll()
    let ids = Array(callIdToUUID.keys)
    for id in ids {
      endCall(callId: id, reason: reason)
    }
  }

  // MARK: - CallKit path

  private func reportIncomingCallKit(
    data: [String: String],
    eventId: String,
    providerName: String,
    callId: String,
    completion: @escaping () -> Void
  ) {
    let uuid = UUID()
    callIdToUUID[callId] = uuid
    uuidToPayload[uuid] = data
    uuidToEventMeta[uuid] = (eventId, providerName)

    let update = CXCallUpdate()
    let handleValue = data["caller_display_name"]
      ?? data["caller_id"]
      ?? data["callerId"]
      ?? "Incoming Call"
    update.remoteHandle = CXHandle(type: .generic, value: handleValue)
    update.localizedCallerName = handleValue
    let media = (data["media_type"] ?? data["mediaType"] ?? "audio").lowercased()
    update.hasVideo = media == "video"
    update.supportsHolding = false
    update.supportsGrouping = false
    update.supportsUngrouping = false
    update.supportsDTMF = false

    provider.reportNewIncomingCall(with: uuid, update: update) { [weak self] error in
      if let error = error {
        self?.cleanup(uuid: uuid)
        self?.onEvent?([
          "type": "error",
          "provider": providerName,
          "code": "CALLKIT_REPORT_FAILED",
          "message": error.localizedDescription
        ])
      } else if self?.isExpired(data) == true {
        self?.endCall(callId: callId, reason: .failed)
      }
      completion()
    }
  }

  // MARK: - CXProviderDelegate

  func providerDidReset(_ provider: CXProvider) {
    callIdToUUID.removeAll()
    uuidToPayload.removeAll()
    uuidToEventMeta.removeAll()
    answeredUUIDs.removeAll()
  }

  func provider(_ provider: CXProvider, perform action: CXAnswerCallAction) {
    answeredUUIDs.insert(action.callUUID)
    emitAction(uuid: action.callUUID, action: "accepted")
    action.fulfill()
  }

  func provider(_ provider: CXProvider, perform action: CXEndCallAction) {
    if !answeredUUIDs.contains(action.callUUID) {
      emitAction(uuid: action.callUUID, action: "rejected")
    } else {
      emitCallEnded(uuid: action.callUUID, reason: "user-ended")
    }
    cleanup(uuid: action.callUUID)
    action.fulfill()
  }

  func provider(_ provider: CXProvider, perform action: CXSetMutedCallAction) {
    action.fulfill()
  }

  func provider(_ provider: CXProvider, didActivate audioSession: AVAudioSession) {
    try? audioSession.setCategory(.playAndRecord, mode: .voiceChat, options: [.allowBluetooth])
    try? audioSession.setActive(true)
  }

  func provider(_ provider: CXProvider, didDeactivate audioSession: AVAudioSession) {
    try? audioSession.setActive(false)
  }

  // MARK: - Helpers

  private func emitAction(uuid: UUID, action: String) {
    guard let data = uuidToPayload[uuid] else { return }
    let meta = uuidToEventMeta[uuid]
    var mutable = data
    mutable["action"] = action
    let callId = data["call_id"] ?? data["callId"] ?? ""
    let baseId = meta?.eventId ?? callId
    let eventId = action == "accepted" || action == "rejected" || action == "timeout"
      ? "\(baseId):\(action)"
      : baseId
    let ts = Int64(Date().timeIntervalSince1970)
    onEvent?([
      "version": 1,
      "type": "incoming-call",
      "id": eventId,
      "timestamp": ts,
      "provider": meta?.provider ?? "apns_voip",
      "data": mutable
    ])
  }

  private func emitCallEnded(uuid: UUID, reason: String) {
    guard let data = uuidToPayload[uuid] else { return }
    let meta = uuidToEventMeta[uuid]
    var mutable = data
    mutable["reason"] = reason
    let callId = data["call_id"] ?? data["callId"] ?? ""
    let baseId = meta?.eventId ?? callId
    onEvent?([
      "version": 1,
      "type": "call-ended",
      "id": "\(baseId):ended",
      "timestamp": Int64(Date().timeIntervalSince1970),
      "provider": meta?.provider ?? "apns_voip",
      "data": mutable
    ])
  }

  private func cleanup(uuid: UUID) {
    if let callId = callIdToUUID.first(where: { $0.value == uuid })?.key {
      callIdToUUID.removeValue(forKey: callId)
    }
    uuidToPayload.removeValue(forKey: uuid)
    uuidToEventMeta.removeValue(forKey: uuid)
    answeredUUIDs.remove(uuid)
  }

  private func isExpired(_ data: [String: String]) -> Bool {
    guard let raw = data["expires_at"] ?? data["expiresAt"],
          let expires = Int64(raw) else { return false }
    return Int64(Date().timeIntervalSince1970) > expires
  }

  private static func appDisplayName() -> String {
    Bundle.main.object(forInfoDictionaryKey: "CFBundleDisplayName") as? String
      ?? Bundle.main.object(forInfoDictionaryKey: "CFBundleName") as? String
      ?? "Call"
  }
}
