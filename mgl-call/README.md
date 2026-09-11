# mgl-call

Reusable audio/video calling library for Flutter + ClojureDart.

> **mgl-call = Connect / Stream / Communicate**

Owns WebRTC PeerConnection, A/V media, call lifecycle, and signaling abstractions.  
**Does not** own Push / CallKit / PushKit / Android Telecom — those belong to [`mgl-push`](../mgl-push-client).

## Responsibility boundary

```text
mgl-push  →  wake you (incoming-call notification / system call UI)
mgl-call  →  talk with you (Signaling / WebRTC / Media)
```

They cooperate via the app layer using **Call ID** and events; neither hard-depends on the other.

## Dependencies

Host `deps.edn`:

```edn
{:paths ["src"]
 :deps {tensegritics/clojuredart
        {:git/url "https://github.com/tensegritics/ClojureDart.git"
         :sha "81b5c03a55cf52b21dc0be8ccfa4827b9889f488"}
        amjil/mgl-call
        {:local/root "../mgl-call"}}
 :aliases {:cljd {:main-opts ["-m" "cljd.build"]}}
 :cljd/opts {:kind :flutter
             :main your.app.main}}
```

Host `pubspec.yaml`:

```yaml
dependencies:
  mgl_call:
    path: ../mgl-call
```

## Quick start

```clojure
(ns your.app.main
  (:require [mgl.call :as call]))

(call/init!
 {:user-id "user-123"
  :access-token "..."          ; JWT; sent as authenticate after WS open
  :device-id "install_xxx"     ; optional; maps to server device_id
  :signaling-url "wss://example.com/ws"
  :ice-servers [{:urls ["stun:stun.l.google.com:19302"]}]
  :log-level :info})

(call/on :call-state-changed
  (fn [event] ...))

;; Or subscribe to specific events
(call/on :call-connected
  (fn [{:keys [call-id]}] ...))

;; Outgoing call
(call/start! {:callee-id "user-456" :media-type :video})

;; Incoming call (usually forwarded from mgl-push events at the app layer)
(call/incoming! {:call-id "call_xxx"
                 :caller-id "user-123"
                 :media-type :audio})

(call/accept! "call_xxx")
(call/reject! "call_xxx")
(call/hangup! "call_xxx")

(call/mute! "call_xxx" true)
(call/set-video-enabled! "call_xxx" false)
(call/switch-camera! "call_xxx")
(call/set-speaker! "call_xxx" true)
```

## Working with mgl-push

```clojure
(push/on-event ::mgl-call
  (fn [event]
    (cond
      (push/incoming-call? event)
      (call/incoming! (push/event-data event))

      (push/call-cancelled? event)
      (call/hangup! (push/call-id event)))))
```

## Custom signaling

WebSocket JSON signaling is provided by default. You can also inject a custom transport:

```clojure
(call/init!
 {:user-id "..."
  :signaling {:connect! (fn [] ...)
              :disconnect! (fn [] ...)
              :send! (fn [msg] ...)
              :on-message! (fn [handler] ...)}})
```

Handshake: after connect, send `authenticate`, wait for `authenticated`.  
Minimal message set: `call.create` / `call.accept` / `call.reject` / `call.cancel` / `call.hangup` / `webrtc.offer` / `webrtc.answer` / `webrtc.ice_candidate`.

## Rendering

The library does not ship a Call UI. Bind streams in the app:

```clojure
(call/local-video-source call-id)  ;; MediaStream
(call/remote-video-source call-id)
```

## Layout

```text
lib/          Dart WebRTC / permission bridge
src/mgl/call  ClojureDart API / session / signaling / media
test/         Unit tests for state machine and permission mapping
```

Full design: [`mgl-call-spec.md`](./mgl-call-spec.md).
