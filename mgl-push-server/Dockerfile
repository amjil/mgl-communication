FROM golang:1.25-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/mgl-push ./cmd/mgl-push

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=build /out/mgl-push /app/mgl-push
EXPOSE 8080
ENTRYPOINT ["/app/mgl-push"]
