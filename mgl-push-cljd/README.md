# mgl-push-cljd

Thin ClojureDart wrapper around [`mgl-push-client`](../mgl-push-client). No vendor providers are implemented in this layer.

## Dependencies

Host app `deps.edn`:

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

The host app `pubspec.yaml` must also depend on the Flutter plugin:

```yaml
dependencies:
  mgl_push:
    path: ../mgl-push/mgl-push-client
```

## Usage

```clojure
(ns your.app.main
  (:require
   [mgl.push.api :as push]))

(def client
  (push/create
   {:server-url "https://push.example.com"
    :service-token "dev-token"
    :app-id "net.amjil.demo"}))

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
