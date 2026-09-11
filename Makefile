.PHONY: test-server build-server test-client up

test-server:
	cd mgl-realtime-server && GOPROXY=https://goproxy.cn,direct GOTOOLCHAIN=local go test ./...

build-server:
	cd mgl-realtime-server && GOPROXY=https://goproxy.cn,direct GOTOOLCHAIN=local go build -o bin/mgl-push ./cmd/mgl-push

test-client:
	cd mgl-push-client && flutter test && flutter analyze

up:
	docker compose up --build
