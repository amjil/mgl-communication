# mgl-call

面向 Flutter + ClojureDart 的可复用音视频通话库。

> **mgl-call = Connect / Stream / Communicate**

负责 WebRTC PeerConnection、音视频媒体、通话生命周期与信令抽象。  
**不负责** Push / CallKit / PushKit / Android Telecom —— 那些属于 [`mgl-push`](../mgl-push-client)。

## 职责边界

```text
mgl-push  →  叫醒你（来电通知 / 系统来电 UI）
mgl-call  →  和你通话（Signaling / WebRTC / Media）
```

两者通过应用层用 **Call ID** 与事件协作，互不强依赖。

## 依赖

Host `deps.edn`：

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

Host `pubspec.yaml`：

```yaml
dependencies:
  mgl_call:
    path: ../mgl-call
```

## 快速开始

```clojure
(ns your.app.main
  (:require [mgl.call :as call]))

(call/init!
 {:user-id "user-123"
  :access-token "..."
  :signaling-url "wss://example.com/socket"
  :ice-servers [{:urls ["stun:stun.l.google.com:19302"]}]
  :log-level :info})

(call/on :call-state-changed
  (fn [event] ...))

;; 也可以订阅具体事件
(call/on :call-connected
  (fn [{:keys [call-id]}] ...))

;; 主叫
(call/start! {:callee-id "user-456" :media-type :video})

;; 来电（通常由 mgl-push 事件在应用层转发）
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

## 与 mgl-push 协作

```clojure
(push/on-event ::mgl-call
  (fn [event]
    (cond
      (push/incoming-call? event)
      (call/incoming! (push/event-data event))

      (push/call-cancelled? event)
      (call/hangup! (push/call-id event)))))
```

## 自定义信令

默认提供 WebSocket JSON 信令。也可注入自定义 transport：

```clojure
(call/init!
 {:user-id "..."
  :signaling {:connect! (fn [] ...)
              :disconnect! (fn [] ...)
              :send! (fn [msg] ...)
              :on-message! (fn [handler] ...)}})
```

最小消息集合：`call.create` / `call.accept` / `call.reject` / `call.cancel` / `call.end` / `webrtc.offer` / `webrtc.answer` / `webrtc.ice-candidate`。

## 渲染

库不提供 Call UI。应用自行绑定：

```clojure
(call/local-video-source call-id)  ;; MediaStream
(call/remote-video-source call-id)
```

## 结构

```text
lib/          Dart WebRTC / permission bridge
src/mgl/call  ClojureDart API / session / signaling / media
test/         状态机与权限映射单测
```

完整设计见 [`mgl-call-spec.md`](./mgl-call-spec.md)。
