# Client

## Flutter

```dart
final push = createMglPush(MglPushConfig(
  serverUrl: 'https://push.example.com',
  serviceToken: 'your-service-token',
  appId: 'net.amjil.nomio',
));

await push.initialize(); // failures must not block app startup
await push.requestPermission();
final device = await push.register();

push.events.listen((e) { /* token_changed / notification_open / error */ });
push.messages.listen((m) { /* foreground data */ });

await push.setUserId('user_123');
await push.clearUserId();
```

Channels:

- Methods: `net.amjil.mgl_push/methods`
- Events: `net.amjil.mgl_push/events`

## ClojureDart

See `mgl-push-cljd/` (includes `deps.edn`). It only wraps the Flutter API and does not reimplement providers.

```clojure
(ns your.app.main
  (:require [mgl.push.api :as push]))

(def client
  (push/create
   {:server-url "https://push.example.com"
    :service-token "your-service-token"
    :app-id "net.amjil.nomio"}))

(await (push/initialize client))
(await (push/request-permission client))
(def device (await (push/register client)))

(.listen (push/events-stream client)
         (fn [e]
           (cond
             (push/token-changed? e) ...
             (push/message? e) ...
             (push/notification-open? e) ...
             (push/error? e) ...)))
```

The host app must depend on both `mgl-push-cljd` (`deps.edn`) and `mgl-push-client` (`pubspec.yaml`).

## Android FCM (Phase 2)

The host app must provide `google-services.json` and enable the Google Services plugin.  
The plugin will:

1. Detect FirebaseApp + Play Services
2. Fetch the FCM token and emit `token_changed`
3. Forward foreground messages via `MglFirebaseMessagingService` → Flutter `message` events
4. Handle notification tap cold start → `notification_open` / `getInitialNotification`

See [PROVIDERS.md](PROVIDERS.md).

## Android Huawei (Phase 4)

1. Place `agconnect-services.json` under `android/app/`
2. Add the repo `https://developer.huawei.com/repo/`
3. When HMS Core is available, the detector prefers `huawei` over FCM

## Android Xiaomi (Phase 5)

1. Add the official MiPush SDK
2. meta-data: `XIAOMI_APP_ID` / `XIAOMI_APP_KEY`
3. `PushMessageReceiver` → `XiaomiBridge` (see `MglMiPushReceiverDocs.kt` in the plugin)

## Android OPPO (Phase 6)

1. Integrate the HeyTap Push SDK
2. meta-data: `OPPO_APP_KEY` / `OPPO_APP_SECRET`
3. Create notification channel `mgl_default`
4. Forward callbacks via `OppoBridge`

## Android vivo (Phase 7)

1. Integrate the vivo Push SDK
2. meta-data: `VIVO_APP_ID` / `VIVO_APP_KEY`
3. Receiver → `VivoBridge` (see `MglVivoPushDocs.kt`)

## iOS APNs (Phase 3)

1. Xcode → Signing & Capabilities → Push Notifications
2. The plugin swizzles AppDelegate to obtain the device token
3. Foreground messages / notification taps go through `UNUserNotificationCenterDelegate` → Flutter events

Optional manual forward:

```swift
func application(_ application: UIApplication,
                 didRegisterForRemoteNotificationsWithDeviceToken deviceToken: Data) {
  MglPushAppDelegate.didRegisterForRemoteNotifications(deviceToken: deviceToken)
}
```

## installation_id

The client generates a ULID, persists it, and keeps it separate from the vendor token. On token change:

```text
PUT /v1/devices/{installation_id}/token
```
