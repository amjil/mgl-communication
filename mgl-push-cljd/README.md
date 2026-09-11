# mgl-push-cljd

Thin ClojureDart wrapper around [`mgl-push-client`](../mgl-push-client).  
**Do not implement any vendor Provider in this layer.**

## Dependencies

Host `deps.edn`:

```edn
{:paths ["src"]
 :deps {tensegritics/clojuredart
        {:git/url "https://github.com/tensegritics/ClojureDart.git"
         :sha "81b5c03a55cf52b21dc0be8ccfa4827b9889f488"}
        amjil/mgl-push
        {:local/root "../mgl-push/mgl-push-cljd"}}
 :aliases {:cljd {:main-opts ["-m" "cljd.build"]}}
 :cljd/opts {:kind :flutter
             :main your.app.main}}
```

Host `pubspec.yaml` must also depend on the Flutter plugin:

```yaml
dependencies:
  mgl_push:
    path: ../mgl-push/mgl-push-client
```

## API (spec §12)

| Function | Description |
|----------|-------------|
| `(push/init! config)` | Create and initialize |
| `(push/create config)` | Create client only |
| `(push/register!)` | Register device |
| `(push/device)` | Current device |
| `(push/unregister!)` | Unregister |
| `(push/set-user-id! id)` / `(push/clear-user-id!)` | User binding |
| `(push/request-permission!)` | Notification permission |
| `(push/capabilities)` | PushCapabilities |
| `(push/on-event handler)` | Register event handler |
| `(push/remove-handler handler)` | Remove handler |

You can also pass an explicit `client` as the first argument (multi-instance).

## Usage

```clojure
(ns your.app.main
  (:require
   [mgl.push.api :as push]
   ;; Compose mgl-call only at the application layer; do not put it inside mgl-push
   ))

(push/init!
 {:server-url "https://push.example.com"
  :service-token "dev-token"
  :app-id "net.amjil.demo"
  :register-on-initialize true})

(push/on-event
  (fn [event]
    (cond
      (push/incoming-call? event)
      (call/incoming! (push/event-data event))

      (push/call-cancelled? event)
      (call/cancel! (push/call-id event))

      (push/call-ended? event)
      …

      (push/notification? event)
      …

      (push/silent? event)
      …

      (push/token-changed? event)
      …

      (push/error? event)
      …)))
```

Convert a DomainPushEvent to a Clojure map:

```clojure
(push/->map event)
;; => {:version 1 :id "…" :type :incoming-call :timestamp … :data {…}}
```

## Namespaces

| NS | Role |
|----|------|
| `mgl.push.api` | Unified exports |
| `mgl.push.core` | Client lifecycle |
| `mgl.push.events` | Predicates and `->map` |
| `mgl.push.config` | Config map builders |
| `mgl.push.device` / `token` / `notification` / `incoming-call` / `background` | Thin helpers |

## Boundary

```text
mgl-push-cljd  →  Flutter plugin  →  Native SDK
```

Do not depend on `mgl-call` or `flutter_webrtc`.
