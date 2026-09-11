import Foundation

#if canImport(LiveCommunicationKit)
import LiveCommunicationKit
import AVFoundation

/// LiveCommunicationKit (iOS 17.4+) Call Control path.
/// Prefer when the framework is present; CallKit remains the PushKit-compliant fallback.
@available(iOS 17.4, *)
final class LiveCommunicationController: NSObject, ConversationManagerDelegate {
  static let shared = LiveCommunicationController()

  private var manager: ConversationManager?
  private var callIdToUUID: [String: UUID] = [:]
  private var uuidToPayload: [UUID: [String: String]] = [:]
  private var uuidToEventMeta: [UUID: (eventId: String, provider: String)] = [:]
  private var answeredUUIDs: Set<UUID> = []

  var onEvent: (([String: Any?]) -> Void)?

  var isReady: Bool { ensureManager() != nil }

  @discardableResult
  private func ensureManager() -> ConversationManager? {
    if let manager { return manager }
    let config = ConversationManager.Configuration(
      ringtoneName: nil,
      iconTemplateImageData: nil,
      maximumConversationGroups: 1,
      maximumConversationsPerConversationGroup: 1,
      includesConversationInRecents: true,
      supportsVideo: true,
      supportedHandleTypes: [.phoneNumber]
    )
    let m = ConversationManager(configuration: config)
    m.delegate = self
    manager = m
    return m
  }

  func reportIncoming(
    data: [String: String],
    eventId: String,
    providerName: String,
    completion: @escaping (Bool) -> Void
  ) {
    guard let manager = ensureManager() else {
      completion(false)
      return
    }
    let callId = data["call_id"] ?? data["callId"] ?? eventId
    guard !callId.isEmpty else {
      completion(false)
      return
    }
    if callIdToUUID[callId] != nil {
      completion(true)
      return
    }

    let uuid = UUID()
    callIdToUUID[callId] = uuid
    uuidToPayload[uuid] = data
    uuidToEventMeta[uuid] = (eventId, providerName)

    let handleValue = data["caller_display_name"]
      ?? data["caller_id"]
      ?? data["callerId"]
      ?? "Incoming Call"
    let media = (data["media_type"] ?? data["mediaType"] ?? "audio").lowercased()
    let handle = Handle(type: .phoneNumber, value: handleValue)
    let capabilities: [Conversation.Capability] = media == "video" ? [.video] : []
    let update = Conversation.Update(members: [handle], capabilities: capabilities)

    Task { @MainActor in
      do {
        try await manager.reportNewIncomingConversation(uuid: uuid, update: update)
        if self.isExpired(data) {
          self.endCall(callId: callId)
        }
        completion(true)
      } catch {
        self.cleanup(uuid: uuid)
        self.onEvent?([
          "type": "error",
          "provider": providerName,
          "code": "LCK_REPORT_FAILED",
          "message": error.localizedDescription
        ])
        completion(false)
      }
    }
  }

  func endCall(callId: String) {
    guard let uuid = callIdToUUID[callId] else { return }
    // Best-effort dismiss; exact Conversation.Event cases vary by SDK.
    if let manager,
       let conversation = manager.conversations.first(where: { $0.uuid == uuid }) {
      // Prefer ending via system when an API is available; local cleanup always runs.
      _ = conversation
      _ = manager
    }
    cleanup(uuid: uuid)
  }

  func endAll() {
    for id in Array(callIdToUUID.keys) {
      endCall(callId: id)
    }
  }

  func conversationManager(_ manager: ConversationManager, conversationChanged conversation: Conversation) {}

  func conversationManagerDidBegin(_ manager: ConversationManager) {}

  func conversationManagerDidReset(_ manager: ConversationManager) {
    callIdToUUID.removeAll()
    uuidToPayload.removeAll()
    uuidToEventMeta.removeAll()
    answeredUUIDs.removeAll()
  }

  func conversationManager(_ manager: ConversationManager, perform action: ConversationAction) {
    let uuid = action.conversationUUID
    if action is JoinConversationAction {
      answeredUUIDs.insert(uuid)
      emitAction(uuid: uuid, action: "accepted")
      action.fulfill()
    } else if action is EndConversationAction {
      if !answeredUUIDs.contains(uuid) {
        emitAction(uuid: uuid, action: "rejected")
      } else {
        emitCallEnded(uuid: uuid, reason: "user-ended")
      }
      cleanup(uuid: uuid)
      action.fulfill()
    } else {
      action.fulfill()
    }
  }

  func conversationManager(_ manager: ConversationManager, timedOutPerforming action: ConversationAction) {
    action.fail()
  }

  func conversationManager(_ manager: ConversationManager, didActivate audioSession: AVAudioSession) {
    try? audioSession.setCategory(.playAndRecord, mode: .voiceChat, options: [.allowBluetooth])
    try? audioSession.setActive(true)
  }

  func conversationManager(_ manager: ConversationManager, didDeactivate audioSession: AVAudioSession) {
    try? audioSession.setActive(false)
  }

  private func emitAction(uuid: UUID, action: String) {
    guard let data = uuidToPayload[uuid] else { return }
    let meta = uuidToEventMeta[uuid]
    var mutable = data
    mutable["action"] = action
    let callId = data["call_id"] ?? data["callId"] ?? ""
    let baseId = meta?.eventId ?? callId
    onEvent?([
      "version": 1,
      "type": "incoming-call",
      "id": "\(baseId):\(action)",
      "timestamp": Int64(Date().timeIntervalSince1970),
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
}
#endif
