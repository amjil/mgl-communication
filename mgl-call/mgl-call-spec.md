# mgl-call Specification

**Version:** 1.0
**Status:** Draft
**Platform:** Flutter + ClojureDart
**Media Engine:** `flutter_webrtc`
**Backend:** Elixir / Phoenix
**Related Library:** `mgl-push`

---

## 1. Overview

`mgl-call` 是一个面向 Flutter + ClojureDart 应用的可复用音视频通话库。

核心职责：

> **mgl-call = Connect / Stream / Communicate**

它负责：

* WebRTC PeerConnection
* 音频通话
* 视频通话
* 摄像头
* 麦克风
* Speaker / Earpiece
* Mute / Unmute
* Camera On / Off
* Camera Switch
* SDP Offer / Answer
* ICE Candidate
* STUN / TURN
* WebRTC connection state
* 通话生命周期
* 多设备通话状态同步
* 与 `mgl-push` 的来电事件协作

它**不负责**：

* APNs
* FCM
* PushKit
* CallKit
* LiveCommunicationKit
* Android Telecom / ConnectionService
* 系统级 incoming-call UI
* 普通 Push Notification
* WebRTC 服务器端媒体转发

这些职责属于：

> `mgl-push`

---

# 2. Architecture

整体架构：

```text
                    ┌──────────────────────┐
                    │      Flutter App     │
                    │    ClojureDart UI    │
                    └──────────┬───────────┘
                               │
                    ┌──────────▼───────────┐
                    │      mgl-call        │
                    │                      │
                    │ CallSession           │
                    │ Signaling             │
                    │ WebRTC                │
                    │ Media                 │
                    │ Device Control        │
                    └──────────┬───────────┘
                               │
                         flutter_webrtc
                               │
                ┌──────────────┴──────────────┐
                │                             │
          Camera / Mic                   PeerConnection
                │                             │
                └──────────────┬──────────────┘
                               │
                        Internet / Network
                               │
                  ┌────────────▼────────────┐
                  │   STUN / TURN / P2P     │
                  └─────────────────────────┘


                    ┌──────────────────────┐
                    │      mgl-push        │
                    │                      │
                    │ Push / Wake           │
                    │ Incoming Call         │
                    │ CallKit / LCK         │
                    │ Android Telecom      │
                    └──────────┬───────────┘
                               │
                               │ Call Events
                               ▼
                         mgl-call
```

---

# 3. Responsibility Boundary

必须严格保持：

```text
mgl-push
    ↓
通知用户有来电
    ↓
系统来电界面
    ↓
用户 Accept / Reject
    ↓
mgl-call
    ↓
建立 WebRTC
    ↓
音视频通信
```

不要把两个库混在一起。

### mgl-push

```text
Push
Wake
Incoming Call
System Call UI
```

### mgl-call

```text
Signaling
PeerConnection
Media
Audio
Video
Call State
```

一句话：

> `mgl-push` 负责“叫醒你 + 系统来电（Call Control）”，`mgl-call` 负责“和你通话（Media）”。

更精确：

```text
LCK / CallKit = Call Control → mgl-push
flutter_webrtc = Media      → mgl-call
```

---

# 4. Design Goals

## 4.1 Reusable

`mgl-call` 必须可以被多个应用使用：

```text
mgl-mail-app
mgl-notes-app
Nomio
mgl-learning-app
其他 Flutter/ClojureDart App
```

不能依赖任何具体业务。

---

## 4.2 Flutter First

底层媒体能力基于：

```text
flutter_webrtc
```

ClojureDart 负责：

* API
* 状态管理
* CallSession
* 事件
* 业务层集成

Dart 负责必要的 Flutter / WebRTC bridge。

禁止引入 JavaScript / ClojureScript 实现。

---

## 4.3 Platform Independent API

上层 API 不应该暴露：

```text
iOS CallKit
iOS PushKit
Android Telecom
Android ConnectionService
```

这些属于 `mgl-push`。

`mgl-call` API 应该保持跨平台。

---

# 5. Supported Call Types

第一阶段支持：

```text
audio
video
```

Call media type：

```clojure
:audio
:video
```

未来可以扩展：

```text
screen-share
data-channel
group-call
```

但第一版本不要实现群聊。

---

# 6. Call Model

核心对象：

```clojure
{:call-id "call_xxx"
 :direction :outgoing
 :media-type :audio

 :caller
 {:id "user-a"
  :name "User A"
  :avatar-url nil}

 :callee
 {:id "user-b"
  :name "User B"
  :avatar-url nil}

 :state :connecting

 :created-at ...
 :connected-at nil
 :ended-at nil

 :duration 0}
```

---

# 7. Call Direction

支持：

```clojure
:incoming
:outgoing
```

---

# 8. Call State

标准状态：

```clojure
:idle
:creating
:ringing
:connecting
:connected
:reconnecting
:ended
:failed
:rejected
:cancelled
```

状态生命周期：

### Outgoing

```text
idle
 ↓
creating
 ↓
ringing
 ↓
connecting
 ↓
connected
 ↓
ended
```

### Incoming

```text
incoming
 ↓
ringing
 ↓
connecting
 ↓
connected
 ↓
ended
```

### Reject

```text
ringing
 ↓
rejected
```

### Cancel

```text
ringing
 ↓
cancelled
```

### Failure

```text
connecting
 ↓
failed
```

---

# 9. Call State Machine

必须保证状态转换合法。

```text
                 ┌─────────────┐
                 │    idle     │
                 └──────┬──────┘
                        │
                        ▼
                 ┌─────────────┐
                 │  creating   │
                 └──────┬──────┘
                        │
                        ▼
                 ┌─────────────┐
                 │   ringing   │
                 └──────┬──────┘
                        │
                 accept │
                        ▼
                 ┌─────────────┐
                 │ connecting  │
                 └──────┬──────┘
                        │
                        ▼
                 ┌─────────────┐
                 │  connected  │
                 └──────┬──────┘
                        │
                        ▼
                 ┌─────────────┐
                 │    ended    │
                 └─────────────┘
```

任何状态都可能因为：

```text
network failure
remote hangup
server cancellation
```

进入：

```text
failed
ended
cancelled
```

---

# 10. CallSession

`CallSession` 是整个库的核心抽象。

建议使用 map-based state，不使用：

```text
defrecord
deftype
defprotocol
defmulti
```

除非底层 Flutter/Dart interop 确实需要。

示例：

```clojure
{:call-id "call-123"

 :state :connected

 :direction :outgoing

 :media-type :video

 :remote-user
 {:id "user-456"
  :name "User"
  :avatar-url nil}

 :local-audio-enabled? true

 :local-video-enabled? true

 :remote-audio-enabled? true

 :remote-video-enabled? true

 :speaker-enabled? false

 :camera-facing :front

 :connection-state :connected

 :ice-state :connected

 :signaling-state :stable}
```

---

# 11. Public API

命名空间：

```clojure
mgl.call
```

---

## 11.1 Initialization

```clojure
(call/init!
 {:user-id "user-123"
  :signaling-url "wss://example.com/socket"
  :ice-servers [...]})
```

返回：

```clojure
{:ok true}
```

---

# 12. Start Outgoing Call

```clojure
(call/start!
 {:callee-id "user-456"
  :media-type :audio})
```

Video：

```clojure
(call/start!
 {:callee-id "user-456"
  :media-type :video})
```

返回：

```clojure
{:call-id "call-123"}
```

---

# 13. Accept Incoming Call

```clojure
(call/accept!
 "call-123")
```

执行：

```text
Accept
 ↓
Create PeerConnection
 ↓
Get Local Media
 ↓
Create Answer
 ↓
Set Local Description
 ↓
Send Answer
 ↓
ICE Negotiation
 ↓
Connected
```

---

# 14. Reject Incoming Call

```clojure
(call/reject!
 "call-123")
```

---

# 15. Hangup

```clojure
(call/hangup!
 "call-123")
```

必须：

1. 通知 signaling server
2. 停止 local tracks
3. 停止 remote tracks
4. 关闭 PeerConnection
5. 清理 CallSession
6. 更新状态

---

# 16. Mute

```clojure
(call/mute!
 "call-123" true)
```

Unmute：

```clojure
(call/mute!
 "call-123" false)
```

也可以提供：

```clojure
(call/toggle-mute!
 "call-123")
```

Mute 不应该停止 microphone device。

而应该：

```text
audioTrack.enabled = false
```

---

# 17. Video

关闭：

```clojure
(call/set-video-enabled!
 "call-123"
 false)
```

打开：

```clojure
(call/set-video-enabled!
 "call-123"
 true)
```

Toggle：

```clojure
(call/toggle-video!
 "call-123")
```

---

# 18. Camera Switch

```clojure
(call/switch-camera!
 "call-123")
```

支持：

```clojure
:front
:back
```

---

# 19. Speaker

```clojure
(call/set-speaker!
 "call-123"
 true)
```

关闭：

```clojure
(call/set-speaker!
 "call-123"
 false)
```

---

# 20. Call State

获取当前通话：

```clojure
(call/state)
```

返回：

```clojure
{:call-id "call-123"
 :state :connected
 :media-type :video}
```

获取指定 Call：

```clojure
(call/state "call-123")
```

---

# 21. Call Events

支持事件订阅：

```clojure
(call/on
 :call-state-changed
 handler)
```

例如：

```clojure
(call/on
 :call-state-changed
 (fn [event]
   ...))
```

事件：

```text
:call-created
:call-ringing
:call-connecting
:call-connected
:call-reconnecting
:call-ended
:call-rejected
:call-cancelled
:call-failed
```

---

# 22. Media Events

```text
:local-audio-changed
:local-video-changed

:remote-audio-changed
:remote-video-changed

:speaker-changed
:camera-changed
```

---

# 23. WebRTC Events

底层事件统一转换成 mgl-call event。

包括：

```text
:ice-gathering
:ice-connected
:ice-disconnected
:ice-failed

:peer-connecting
:peer-connected
:peer-disconnected

:track-added
:track-removed
```

业务层不能直接依赖 `RTCPeerConnection`。

---

# 24. WebRTC Architecture

核心结构：

```text
CallSession
    │
    ├── Signaling
    │
    ├── PeerConnection
    │
    ├── Local Media
    │   ├── AudioTrack
    │   └── VideoTrack
    │
    └── Remote Media
        ├── AudioTrack
        └── VideoTrack
```

---

# 25. PeerConnection

每个 `CallSession` 默认对应：

```text
1 PeerConnection
```

配置：

```clojure
{:ice-servers
 [{:urls ["stun:stun.example.com:3478"]}
  {:urls ["turn:turn.example.com:3478"]
   :username "..."
   :credential "..."}]}
```

不要在客户端硬编码 TURN credential。

生产环境必须由后端动态提供临时 TURN credential。

---

# 26. STUN / TURN

优先：

```text
P2P
```

失败时：

```text
TURN Relay
```

流程：

```text
Client A
    │
    ├── STUN
    │
    ▼
Direct P2P
```

如果 P2P 失败：

```text
Client A
    │
    ▼
TURN
    │
    ▼
Client B
```

---

# 27. Signaling

`mgl-call` 必须拥有独立的 signaling abstraction。

不要把 Phoenix API 细节直接写入 WebRTC 层。

建议：

```clojure
{:send! ...
 :on-message! ...
 :connect! ...
 :disconnect! ...}
```

例如：

```clojure
(signaling/send!
 {:type :offer
  :call-id "call-123"
  :sdp "..."})
```

---

# 28. Signaling Messages

最小消息集合：

```text
call.create
call.accept
call.reject
call.cancel
call.end

webrtc.offer
webrtc.answer
webrtc.ice-candidate
```

可扩展：

```text
media.audio
media.video

call.mute
call.unmute

call.video-on
call.video-off
```

---

# 29. SDP

Offer：

```text
Caller
 ↓
createOffer
 ↓
setLocalDescription
 ↓
send offer
```

Answer：

```text
Callee
 ↓
setRemoteDescription
 ↓
createAnswer
 ↓
setLocalDescription
 ↓
send answer
```

---

# 30. ICE Candidate

发送：

```clojure
{:type :ice-candidate
 :call-id "call-123"
 :candidate "..."}
```

接收：

```text
remote ICE candidate
        ↓
addIceCandidate
```

ICE candidate 处理必须支持：

```text
candidate
sdpMid
sdpMLineIndex
```

---

# 31. Trickle ICE

第一版本必须使用：

```text
Trickle ICE
```

不要等待所有 ICE candidate 收集完成后才发送 SDP。

流程：

```text
Offer
  ↓
send immediately
  ↓
ICE candidate #1
ICE candidate #2
ICE candidate #3
...
```

---

# 32. Local Media

Audio：

```clojure
{:audio true}
```

Video：

```clojure
{:audio true
 :video true}
```

使用：

```text
navigator.mediaDevices.getUserMedia()
```

获取：

```text
MediaStream
```

---

# 33. Audio Call

Audio Call：

```text
Microphone
    ↓
AudioTrack
    ↓
PeerConnection
    ↓
Network
    ↓
PeerConnection
    ↓
Remote Audio
```

不应该启动 Camera。

---

# 34. Video Call

Video Call：

```text
Camera ──┐
         ├── MediaStream ── PeerConnection
Mic ─────┘
```

接收：

```text
PeerConnection
      ↓
Remote MediaStream
      ↓
VideoRenderer / Audio
```

---

# 35. Flutter Rendering

mgl-call 不应该强制用户使用某一个页面 UI。

可以提供：

```clojure
(call/local-stream)
(call/remote-stream)
```

或者暴露 Flutter WebRTC renderer 所需对象。

UI 由应用负责。

例如：

```text
mgl-call
   │
   ├── local video stream
   └── remote video stream

Application
   │
   ├── CallPage
   ├── VideoView
   ├── MuteButton
   ├── CameraButton
   └── HangupButton
```

---

# 36. Renderer Boundary

不要把：

```text
RTCVideoView
```

作为核心业务 API。

推荐：

```clojure
(call/local-video-source call-id)
(call/remote-video-source call-id)
```

由 Flutter 层将 source 绑定到：

```text
RTCVideoRenderer
```

这样可以避免业务层和 WebRTC renderer 强耦合。

---

# 37. Permissions

mgl-call 必须提供权限检查能力：

```clojure
(call/microphone-permission)
(call/camera-permission)
```

状态：

```text
:unknown
:denied
:restricted
:authorized
```

申请：

```clojure
(call/request-microphone-permission!)
(call/request-camera-permission!)
```

但是：

> 系统权限 UI 不属于 CallSession。

---

# 38. Incoming Call Integration

Incoming Call 本身由：

```text
mgl-push
```

负责。

流程：

```text
Phoenix
   │
   │ incoming-call
   ▼
mgl-push
   │
   ├── iOS PushKit
   ├── CallKit / LCK
   │
   └── Android Telecom
   │
   ▼
User Accept
   │
   ▼
Application
   │
   ▼
mgl-call/accept!
```

mgl-call 不直接监听 APNs / PushKit / FCM。

---

# 39. mgl-push → mgl-call

两个库通过标准事件连接。

例如：

```clojure
(push/on
 :incoming-call
 (fn [event]
   ...))
```

Application：

```clojure
(push/on
 :incoming-call
 (fn [{:keys [call-id]}]
   (call/incoming!
    {:call-id call-id})))
```

---

# 40. Incoming Call API

mgl-call 提供：

```clojure
(call/incoming!
 {:call-id "call-123"
  :caller-id "user-123"
  :media-type :video})
```

该 API：

* 创建 CallSession
* 设置 `:direction :incoming`
* 设置 `:state :ringing`
* 等待 Accept / Reject

它不负责：

```text
Push
CallKit
LCK
Android Telecom
```

---

# 41. Incoming Call Lifecycle

```text
mgl-push
    │
    │ incoming-call
    ▼
Application
    │
    ▼
mgl-call/incoming!
    │
    ▼
:ringing
    │
    ├──── reject ────► :rejected
    │
    └──── accept ────► :connecting
                           │
                           ▼
                       :connected
```

---

# 42. Call Cancellation

当主叫方取消：

```text
Caller
   │
   ▼
Phoenix
   │
   ▼
callee
```

mgl-call 收到：

```text
call.cancel
```

然后：

```text
:ringing
   ↓
:cancelled
```

同时：

```text
mgl-push
```

负责关闭系统来电界面。

---

# 43. Multi-device

一个用户可能有：

```text
iPhone
iPad
Android
Mac
```

同一个 incoming call 可能同时发送到多个设备。

例如：

```text
User B
 ├── iPhone
 ├── iPad
 └── Mac
```

所有设备可能收到：

```text
call-123
```

但只能有一个设备 Accept。

---

# 44. Server-side Call Claim

必须由 Phoenix 保证：

```text
first accepted device wins
```

例如：

```text
Device A → accept
Device B → accept
```

服务器：

```text
Device A = accepted
Device B = rejected
```

Device B 收到：

```text
call.cancelled
```

然后：

```text
mgl-push
```

关闭 Device B 的系统来电界面。

---

# 45. Call ID

所有通话必须拥有唯一：

```text
call_id
```

例如：

```text
call_01J...
```

Call ID 是：

```text
mgl-push
mgl-call
Phoenix signaling
```

之间的核心关联 ID。

---

# 46. Call ID Rules

Call ID：

* 必须全局唯一
* 不可重复使用
* 不包含敏感信息
* 不作为身份认证凭证
* 不应该直接暴露 SDP / TURN credentials

---

# 47. Security

禁止在 incoming-call push payload 中传输：

```text
SDP
ICE candidate
TURN password
authentication token
media URL
secret
```

只传：

```clojure
{:call-id "call-123"
 :caller-id "user-123"
 :callee-id "user-456"
 :media-type :audio
 :timestamp 123456789}
```

真正 signaling 必须通过：

```text
authenticated WebSocket / HTTPS
```

完成。

---

# 48. Signaling Authentication

建立 signaling connection 时使用：

```text
application authentication token
```

例如：

```clojure
{:access-token "..."}
```

服务器必须验证：

```text
user identity
device identity
call membership
```

不能因为客户端知道：

```text
call-id
```

就允许加入通话。

---

# 49. Call Authorization

Phoenix 必须验证：

```text
caller ∈ call
callee ∈ call
```

例如：

```text
User A
   │
   └── call-123
        │
        ├── caller = A
        └── callee = B
```

User C 不允许加入：

```text
call-123
```

---

# 50. WebRTC Security

WebRTC 默认使用：

```text
DTLS-SRTP
```

媒体流不得经过普通 HTTP。

生产环境：

```text
HTTPS
WSS
TURN over TLS
```

优先。

---

# 51. Network Recovery

必须支持：

```text
connected
   ↓
network interruption
   ↓
reconnecting
   ↓
connected
```

例如：

```text
Wi-Fi
 ↓
4G
```

或者：

```text
4G
 ↓
Wi-Fi
```

mgl-call 应监听：

```text
ICE connection state
PeerConnection state
```

并自动进入：

```text
:reconnecting
```

---

# 52. Reconnection

重连策略：

```text
immediate
 ↓
1s
 ↓
2s
 ↓
4s
 ↓
8s
```

最大重试次数可配置。

例如：

```clojure
{:reconnect
 {:enabled true
  :max-attempts 5
  :backoff [1000 2000 4000 8000]}}
```

---

# 53. Call Timeout

Outgoing ringing 必须有超时。

默认：

```text
60 seconds
```

例如：

```clojure
{:ring-timeout-ms 60000}
```

超时：

```text
:ringing
 ↓
:cancelled
```

---

# 54. Connection Timeout

WebRTC connecting 也必须有超时。

例如：

```text
30 seconds
```

超时：

```text
:connecting
 ↓
:failed
```

---

# 55. Cleanup

任何终态：

```text
:ended
:failed
:rejected
:cancelled
```

必须释放：

```text
MediaStream
MediaStreamTrack
PeerConnection
Renderer
Signaling resources
Timers
Event handlers
```

避免：

```text
camera remains active
microphone remains active
memory leak
WebSocket leak
```

---

# 56. Resource Lifecycle

正确流程：

```text
call/start!
      │
      ▼
create session
      │
      ▼
create peer connection
      │
      ▼
getUserMedia
      │
      ▼
add tracks
      │
      ▼
signaling
      │
      ▼
connected
      │
      ▼
hangup
      │
      ▼
stop tracks
      │
      ▼
close peer connection
      │
      ▼
cleanup session
```

---

# 57. Audio Route

至少支持：

```text
earpiece
speaker
bluetooth
wired headset
```

API 不直接暴露平台实现。

统一：

```clojure
(call/audio-route)
(call/set-audio-route!)
```

例如：

```clojure
(call/set-audio-route!
 "call-123"
 :speaker)
```

---

# 58. Background

`mgl-call` 本身不负责唤醒应用。

后台来电：

```text
mgl-push
```

负责。

用户接听之后：

```text
mgl-call
```

开始：

```text
signaling
media
peer connection
```

---

# 59. App Lifecycle

必须处理：

```text
foreground
background
inactive
resumed
```

特别是：

```text
active call
+
application background
```

不能因为 Flutter UI 不可见而错误销毁：

```text
PeerConnection
MediaStream
CallSession
```

具体后台策略由平台层和 `mgl-push` 协作决定。

---

# 60. Concurrent Calls

第一版本：

```text
1 device = 1 active call
```

不支持：

```text
call A active
call B active
```

当已有通话：

```clojure
(call/state)
```

不是 `:idle` 时，新 incoming call 应该被拒绝或交给应用策略处理。

---

# 61. Call Statistics

提供：

```clojure
(call/stats "call-123")
```

返回：

```clojure
{:rtt-ms 50
 :packet-loss 0.01
 :jitter-ms 8
 :audio-bitrate 32000
 :video-bitrate 800000
 :connection-type :wifi}
```

第一阶段可以只实现：

```text
RTT
packet loss
jitter
bitrate
```

---

# 62. Diagnostics

提供：

```clojure
(call/diagnostics)
```

用于开发环境。

例如：

```clojure
{:peer-connection-state :connected
 :ice-state :connected
 :signaling-state :stable
 :local-tracks 2
 :remote-tracks 2}
```

禁止在生产日志中输出：

```text
access token
TURN password
SDP
完整 ICE credentials
```

---

# 63. Logging

支持：

```clojure
:debug
:info
:warn
:error
```

默认：

```text
:info
```

生产环境禁止输出敏感 signaling payload。

---

# 64. Public API Summary

核心 API：

```clojure
(call/init!)

(call/start!)

(call/incoming!)

(call/accept!)

(call/reject!)

(call/hangup!)

(call/mute!)

(call/toggle-mute!)

(call/set-video-enabled!)

(call/toggle-video!)

(call/switch-camera!)

(call/set-speaker!)

(call/state)

(call/stats)

(call/diagnostics)

(call/on)

(call/off)
```

---

# 65. Suggested Source Structure

项目结构：

```text
mgl-call/
├── lib/
│   ├── ...
│   └── ...
│
├── src/
│   └── mgl/
│       └── call/
│           ├── core.cljd
│           ├── config.cljd
│           ├── state.cljd
│           ├── session.cljd
│           ├── event.cljd
│           ├── signaling.cljd
│           ├── webrtc.cljd
│           ├── media.cljd
│           ├── audio.cljd
│           ├── video.cljd
│           ├── permissions.cljd
│           ├── stats.cljd
│           ├── diagnostics.cljd
│           └── lifecycle.cljd
│
├── test/
│   └── ...
│
├── pubspec.yaml
├── README.md
└── mgl-call-spec.md
```

---

# 66. Layering

推荐：

```text
API
 │
 ▼
Call Core
 │
 ├── Session
 ├── State
 └── Events
 │
 ▼
Signaling
 │
 ▼
WebRTC
 │
 ├── PeerConnection
 ├── MediaStream
 ├── AudioTrack
 └── VideoTrack
 │
 ▼
flutter_webrtc
 │
 ▼
Native Platform
```

---

# 67. Signaling Interface

不要直接依赖 Phoenix Channel。

定义抽象：

```clojure
{:connect! ...
 :disconnect! ...
 :send! ...
 :on-message! ...}
```

然后可以实现：

```text
Phoenix WebSocket
Phoenix Channel
WebSocket
custom signaling
```

---

# 68. Phoenix Integration

推荐服务器架构：

```text
Flutter
   │
   ├── HTTPS
   │
   ├── Push registration
   │
   └── WebSocket
          │
          ▼
      Phoenix
          │
          ├── Auth
          ├── Call
          ├── Signaling
          └── Push coordination
```

Phoenix 不传输音视频媒体。

---

# 69. Phoenix Call Context

服务器维护：

```text
Call
```

至少包括：

```text
call_id
caller_id
callee_id
media_type
status
created_at
accepted_at
ended_at
accepted_device_id
```

状态：

```text
pending
ringing
accepted
connected
ended
rejected
cancelled
failed
```

---

# 70. Server Call State vs Client Call State

必须区分：

```text
Server Call State
```

和：

```text
WebRTC Connection State
```

例如：

```text
Server:
accepted

Client:
connecting
```

这是合法状态。

不要强行把两套状态合并。

---

# 71. Example: Outgoing Call

```text
User A
 │
 │ call/start!
 ▼
mgl-call
 │
 │ call.create
 ▼
Phoenix
 │
 │ incoming-call
 ▼
mgl-push
 │
 ▼
User B
 │
 │ Accept
 ▼
mgl-call
 │
 │ signaling
 ▼
Phoenix
 │
 ├── offer
 ├── answer
 └── ICE
 │
 ▼
WebRTC
 │
 ▼
Connected
```

---

# 72. Example: Incoming Call

```text
Phoenix
 │
 │ incoming-call
 ▼
mgl-push
 │
 ├── PushKit / APNs
 ├── CallKit / LCK
 └── Android Telecom
 │
 ▼
User
 │
 │ Accept
 ▼
mgl-call
 │
 ├── create PeerConnection
 ├── getUserMedia
 ├── send answer
 └── ICE
 │
 ▼
Connected
```

---

# 73. Example: Reject

```text
Incoming Call
     │
     ▼
mgl-push
     │
     ▼
User Reject
     │
     ▼
mgl-call/reject!
     │
     ▼
Phoenix
     │
     ▼
Caller
     │
     ▼
call.rejected
```

---

# 74. Example: Caller Cancels

```text
Caller
  │
  ▼
hangup/cancel
  │
  ▼
Phoenix
  │
  ▼
Callee
  │
  ├── mgl-push closes system call
  │
  └── mgl-call → :cancelled
```

---

# 75. Error Model

统一错误：

```clojure
{:error
 {:code :permission-denied
  :message "Microphone permission denied"}}
```

错误类型：

```text
:permission-denied
:camera-permission-denied
:microphone-permission-denied

:signaling-failed
:signaling-timeout

:peer-connection-failed
:ice-failed

:call-not-found
:call-expired
:call-already-ended

:already-in-call
:unsupported-platform
:media-device-error
```

---

# 76. No UI Policy

`mgl-call` 不提供完整的：

```text
CallPage
IncomingCallPage
DialPage
```

因为这些属于应用。

可以提供少量：

```text
VideoRenderer helper
```

但不能让 UI 成为核心依赖。

---

# 77. No Push Dependency

`mgl-call` 不依赖：

```text
mgl-push
```

即：

```text
mgl-call
```

应该可以单独运行。

例如：

```text
Web App
Desktop App
internal test application
```

可以只使用：

```text
mgl-call
```

进行 WebRTC 通话。

---

# 78. Optional mgl-push Integration

应用层可以同时依赖：

```text
mgl-push
mgl-call
```

关系：

```text
Application
   │
   ├──────────► mgl-push
   │                │
   │                ▼
   │          incoming-call
   │
   └──────────► mgl-call
                    │
                    ▼
                 WebRTC
```

不是：

```text
mgl-push
    ↓
mgl-call
```

直接强依赖。

---

# 79. Dependency Rules

## mgl-call depends on

```text
flutter_webrtc
```

以及必要的 Flutter/Dart runtime。

## mgl-call must NOT depend on

```text
mgl-push
CallKit
PushKit
LiveCommunicationKit
Android Telecom
FCM
APNs
```

## mgl-push must NOT depend on

```text
flutter_webrtc
mgl-call
```

---

# 80. Platform Strategy

### iOS

```text
mgl-push
 ├── APNs
 ├── PushKit
 ├── CallKit / LiveCommunicationKit
 └── system call

mgl-call
 └── flutter_webrtc
```

### Android

```text
mgl-push
 ├── FCM / vendor push
 └── Android Telecom

mgl-call
 └── flutter_webrtc
```

### macOS

```text
mgl-call
 └── flutter_webrtc
```

系统电话集成以后单独扩展。

### Windows

```text
mgl-call
 └── flutter_webrtc
```

### Linux

```text
mgl-call
 └── flutter_webrtc
```

### Web

```text
mgl-call
 └── WebRTC
```

---

# 81. MVP

第一阶段只实现：

```text
Audio Call
Video Call

Outgoing Call
Incoming Call

Accept
Reject
Hangup

Mute
Unmute

Camera On
Camera Off
Switch Camera

Speaker

Offer
Answer
ICE

STUN
TURN

Connection State

Reconnection

Call Timeout
```

暂不实现：

```text
Group Call
Screen Share
Recording
E2EE custom layer
Call Transfer
Hold
Conference
Call Waiting
```

---

# 82. Phase 2

可以增加：

```text
Screen Share
Call Waiting
Hold
Bluetooth route control
Advanced audio route
Network quality indicator
Call statistics UI
```

---

# 83. Phase 3

未来：

```text
Group Call
SFU
Multi-party conference
Screen sharing
Recording
E2EE
Call transfer
Device handoff
```

如果实现群聊：

```text
mgl-call
```

客户端仍然负责 WebRTC。

服务端可以增加：

```text
SFU
```

例如：

```text
LiveKit
Janus
mediasoup
Pion
```

但不要把 SFU 实现放入第一版。

---

# 84. Testing

必须至少测试：

## Unit Test

```text
Call state machine
Call lifecycle
Event dispatch
Timeout
Error handling
```

## Integration Test

```text
Outgoing audio call
Outgoing video call
Incoming audio call
Incoming video call

Accept
Reject
Cancel
Hangup

Mute
Camera switch
```

## Network Test

```text
Wi-Fi → 4G
4G → Wi-Fi
Network offline
Network recovery
High latency
Packet loss
```

## Multi-device Test

```text
iPhone + iPad
iPhone + Android
Multiple devices receiving same call
First device accept
Second device cancellation
```

---

# 85. Logging Requirements

开发模式允许：

```text
Call ID
State
ICE state
Peer state
Signaling type
Network type
```

生产环境禁止：

```text
Access Token
SDP
TURN Credential
Private user data
```

---

# 86. Performance Requirements

Audio call：

```text
low CPU
low memory
stable background audio
```

Video call：

```text
adaptive bitrate
adaptive resolution
network-aware recovery
```

不能因为：

```text
Flutter UI rebuild
```

而重新创建：

```text
PeerConnection
MediaStream
MediaTrack
```

---

# 87. State Management

内部状态建议使用：

```clojure
atom
```

例如：

```clojure
(defonce state
  (atom
   {:calls {}
    :active-call-id nil
    :initialized? false}))
```

更新：

```clojure
(swap! state ...)
```

不要引入大型状态管理框架。

---

# 88. ClojureDart Style

优先：

```clojure
map
vector
atom
fn
let
cond
case
```

避免不必要的：

```clojure
defrecord
deftype
defprotocol
defmulti
```

Flutter interop 仅在必要位置使用。

---

# 89. Native Boundary

Native / Flutter / WebRTC bridge 必须保持薄。

原则：

```text
ClojureDart
   │
   ▼
Dart
   │
   ▼
flutter_webrtc
   │
   ▼
Native/WebRTC
```

不要把业务状态塞入 native 层。

---

# 90. Important Design Principle

不要把：

```text
CallSession
PeerConnection
System Call
Push
```

设计成一个巨大对象。

应该保持：

```text
Push Layer
    │
    ▼
Incoming Call Event
    │
    ▼
Call Layer
    │
    ▼
WebRTC Session
```

三个层次：

```text
Push
Call
WebRTC
```

---

# 91. Final Architecture

最终推荐：

```text
                    Application
                         │
             ┌───────────┴───────────┐
             │                       │
             ▼                       ▼
         mgl-push                mgl-call
             │                       │
      ┌──────┼──────┐          ┌─────┼─────┐
      │      │      │          │     │     │
     APNs   FCM   System      Call  Media WebRTC
             │      Call       │
             │                 │
             └───────event─────┘
                         │
                         ▼
                     Phoenix
                         │
                    Signaling
                         │
                         ▼
                    WebRTC Network
                         │
                 ┌───────┴───────┐
                 │               │
              Client A        Client B
```

---

# 92. Non-goals

`mgl-call` 第一版本明确不负责：

* Push notification
* Device token
* APNs
* FCM
* PushKit
* CallKit
* LiveCommunicationKit
* Android Telecom
* System incoming-call UI
* User authentication
* User profile
* Contact list
* Chat
* Message history
* Call recording
* Billing
* Group conference
* SFU server
* TURN server implementation

---

# 93. Final Principle

整个通话系统必须遵循：

```text
mgl-push
    =
Notify / Wake / Present

mgl-call
    =
Connect / Stream / Communicate
```

也就是：

> **mgl-push 负责让用户知道“有人打电话给你”。**

> **mgl-call 负责真正建立“你和对方之间的音视频连接”。**

两者通过：

```text
Call ID
Call Events
Application Layer
```

连接，而不是互相强依赖。

最终形成：

```text
             ┌────────────────────┐
             │    User / App      │
             └─────────┬──────────┘
                       │
              ┌────────┴────────┐
              │                 │
              ▼                 ▼
          mgl-push          mgl-call
              │                 │
       Push / System       WebRTC / Media
              │                 │
              └────────┬────────┘
                       │
                    Phoenix
                       │
                  Signaling
```

**核心原则：**

> `mgl-push = 叫醒你`
> `mgl-call = 和你通话`

这样 `mgl-push` 可以独立成为一个设备通信基础库，而 `mgl-call` 则可以独立成为一个 WebRTC 通话基础库，两者组合后形成完整的音视频通话能力。
