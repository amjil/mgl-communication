# Client

## Flutter

Depend on `mgl-push-client` (pubspec path / git).

```dart
final push = createMglPush(MglPushConfig(
  serverUrl: 'https://push.example.com',
  serviceToken: 'your-service-token',
  appId: 'net.amjil.nomio',
  registerOnInitialize: true, // default
));

final device = await push.initialize(); // must not crash app startup on failure
await push.requestPermission();
final caps = await push.getCapabilities();

push.events.listen((e) {
  switch (e) {
    case DomainPushEvent(:final type, :final data) when e.isIncomingCall:
      // → mgl-call
      break;
    case DomainPushEvent() when e.isCallCancelled:
      break;
    case TokenChangedEvent(:final provider, :final token):
      break;
    case NotificationOpenEvent():
      break;
    case ErrorEvent():
      break;
    default:
      break;
  }
});

await push.setUserId('user_123');
await push.clearUserId();
await push.unregister();
```

### Channels

| Channel | Name |
|---------|------|
| Methods | `net.amjil.mgl_push/methods` |
| Events | `net.amjil.mgl_push/events` |

Common methods: `initialize` · `register` · `getDevice` · `getCapabilities` · `consumePendingEvents` · `getInitialNotification` · `requestPermission` · `setUserId` · `clearUserId` · `unregister`

### Domain events

| Wire `type` | Dart |
|-------------|------|
| `notification` / `silent` / `background` / `incoming-call` / … | `DomainPushEvent` |
| `token_changed` | `TokenChangedEvent` |
| `notification_open` | `NotificationOpenEvent` |
| `error` | `ErrorEvent` |

Dedupe: `EventDeduper` (`event_id` + incoming-call `call_id`).  
Expiry: drop incoming-call events whose `expires_at` has passed.

Cold start: `initialize` calls `consumePendingEvents` and injects results into the event stream.

## ClojureDart

See [`mgl-push-cljd/README.md`](../mgl-push-cljd/README.md). The host must depend on both the Flutter plugin and the CLJD package.

```clojure
(require '[mgl.push.api :as push])

(push/init! {:server-url "…" :service-token "…" :app-id "net.amjil.nomio"})
(push/capabilities)
;; Unique key → overwrites on hot reload; returns cleanup fn
(push/on-event ::mgl-call
  (fn [e]
    (cond
      (push/incoming-call? e) (call/incoming! (push/event-data e))
      (push/call-cancelled? e) (call/cancel! (push/call-id e))
      (push/notification? e) …
      (push/silent? e) …
      (push/token-changed? e) …)))
```

Namespaces: `mgl.push.api` · `core` · `events` · `config` · `device` · `token` · `notification` · `incoming-call` · `background`

## Android FCM (Phase 1)

1. Host provides `google-services.json` and enables the Google Services plugin
2. Detector: fall back to FCM when no CN vendor SDK is present
3. `MglFirebaseMessagingService` is declared in the plugin Manifest
4. Cold start: `PendingNativeStore` / `getInitialNotification`

## Android Huawei (Phase 2)

1. `android/app/agconnect-services.json`
2. Repo: `https://developer.huawei.com/repo/`
3. When HMS is available, Detector **prefers Huawei** (then FCM)
4. `MglHmsMessageService` is declared in the plugin Manifest
5. Messages arrive at `PendingNativeStore` when the Bridge is not ready

## Android Xiaomi (Phase 2)

1. Host adds the official MiPush SDK AAR
2. Manifest meta-data: `XIAOMI_APP_ID` / `XIAOMI_APP_KEY`
3. Copy [`android/examples/AppMiPushReceiver.kt`](../mgl-push-client/android/examples/AppMiPushReceiver.kt) into the host and register the Receiver
4. **Server** must set `MGL_PUSH_XIAOMI_PACKAGE_NAME` (Android package name, not business `app_id`)

## Android OPPO (Phase 3)

1. Integrate HeyTap Push SDK
2. meta-data: `OPPO_APP_KEY` / `OPPO_APP_SECRET`
3. Create notification channel `mgl_default` (or match Category)
4. Forward callbacks to `OppoBridge` (supports cold-start persistence)
5. For data-only such as incoming call: server synthesizes a notification shell; real fields live in `action_parameters`

## Android vivo (Phase 3)

1. Integrate vivo Push SDK
2. meta-data: `VIVO_APP_ID` / `VIVO_APP_KEY` (or official `com.vivo.push.*`)
3. Receiver → `VivoBridge` (see `MglVivoPushDocs.kt`)

## iOS APNs + PushKit (Phase 1)

1. Xcode → Push Notifications
2. Incoming Call: enable **VoIP** capability (PushKit)
3. Plugin swizzles AppDelegate and registers `PKPushRegistry`
4. VoIP push → LCK (when available on iOS 17.4+) or CallKit system call UI → `incoming-call` DomainPushEvent; after Accept, `data.action=accepted`, then hand off to mgl-call for media setup
5. Silent / background: `content-available` + AppDelegate remote-notification forwarding
6. App may call `endSystemCall(callId)` to dismiss the system call UI (e.g. call-ended)
7. iOS `register()` registers both `apns` and `apns_voip` device rows (same `installation_id`)

Optional manual forwarding:

```swift
func application(_ application: UIApplication,
                 didRegisterForRemoteNotificationsWithDeviceToken deviceToken: Data) {
  MglPushAppDelegate.didRegisterForRemoteNotifications(deviceToken: deviceToken)
}
```

## installation_id

The client generates a ULID and persists it, separate from the vendor token. On token refresh:

```text
PUT /v1/devices/{installation_id}/token
{ "provider": "fcm", "token": "…" }
```

iOS VoIP token changes also emit `token_changed` (`provider: apns_voip`).

## Protocol

JSON Schema: [`mgl-push-protocol/`](../mgl-push-protocol/)  
(`event` · `notification` · `incoming-call` · `device`)
