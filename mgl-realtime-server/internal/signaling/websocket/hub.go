package websocket

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/auth"
	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/call"
	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/events"
	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/idgen"
	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/metrics"
	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/presence"
	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/ratelimit"
	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/signaling/connection"
	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/signaling/protocol"
	gorilla "github.com/gorilla/websocket"
)

var upgrader = gorilla.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

type CallAPI interface {
	Create(ctx context.Context, in call.CreateInput) (*call.Call, error)
	Accept(ctx context.Context, callID, userID, deviceID string) (*call.Call, error)
	Reject(ctx context.Context, callID, userID string) (*call.Call, error)
	Cancel(ctx context.Context, callID, userID string) (*call.Call, error)
	Join(ctx context.Context, callID, userID, deviceID string) (*call.Call, error)
	Leave(ctx context.Context, callID, userID string) (*call.Call, error)
	Hangup(ctx context.Context, callID, userID string) (*call.Call, error)
	Resume(ctx context.Context, callID, userID, deviceID string) (*call.Call, error)
	ResumeFull(ctx context.Context, callID, userID, deviceID string) (*call.ResumeResult, error)
	MarkConnected(ctx context.Context, callID, userID string) (*call.Call, error)
	Get(ctx context.Context, callID string) (*call.Call, error)
	ListActiveForUser(ctx context.Context, appID, userID string) []*call.Call
}

type IncomingNotifier interface {
	NotifyRinging(ctx context.Context, c *call.Call, callerName string)
	NotifyCancelled(ctx context.Context, c *call.Call, reason string)
	NotifyStopRinging(ctx context.Context, c *call.Call, userID, reason string)
}

type userSub struct {
	cancel  context.CancelFunc
	cleanup func()
}

type Hub struct {
	jwt      *auth.JWTValidator
	presence *presence.Store
	calls    CallAPI
	incoming IncomingNotifier
	bus      *events.Bus
	redisBus *events.RedisBus
	logger   *slog.Logger

	pingInterval time.Duration
	readTimeout  time.Duration

	msgLimiter  *ratelimit.Limiter
	callLimiter *ratelimit.Limiter

	mu       sync.RWMutex
	conns    map[string]*connection.Conn            // connID
	byUser   map[string]map[string]*connection.Conn // app|user -> connID -> conn
	userSubs map[string]*userSub                    // app|user -> Redis subscription (ref by local conns)
}

type HubConfig struct {
	PingInterval     time.Duration
	ReadTimeout      time.Duration
	MsgPerSec        int
	CallsPerMinute   int
}

func NewHub(
	jwt *auth.JWTValidator,
	presenceStore *presence.Store,
	calls CallAPI,
	incoming IncomingNotifier,
	bus *events.Bus,
	redisBus *events.RedisBus,
	logger *slog.Logger,
	cfg HubConfig,
) *Hub {
	if logger == nil {
		logger = slog.Default()
	}
	if cfg.PingInterval <= 0 {
		cfg.PingInterval = 25 * time.Second
	}
	if cfg.ReadTimeout <= 0 {
		cfg.ReadTimeout = 60 * time.Second
	}
	if cfg.MsgPerSec <= 0 {
		cfg.MsgPerSec = 30
	}
	if cfg.CallsPerMinute <= 0 {
		cfg.CallsPerMinute = 20
	}
	h := &Hub{
		jwt:          jwt,
		presence:     presenceStore,
		calls:        calls,
		incoming:     incoming,
		bus:          bus,
		redisBus:     redisBus,
		logger:       logger,
		pingInterval: cfg.PingInterval,
		readTimeout:  cfg.ReadTimeout,
		msgLimiter:   ratelimit.New(cfg.MsgPerSec, time.Second),
		callLimiter:  ratelimit.New(cfg.CallsPerMinute, time.Minute),
		conns:        make(map[string]*connection.Conn),
		byUser:       make(map[string]map[string]*connection.Conn),
		userSubs:     make(map[string]*userSub),
	}
	if bus != nil {
		bus.SubscribeAll(h.onEvent)
	}
	return h
}

func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("websocket upgrade failed", "error", err)
		return
	}
	conn := connection.New("conn_"+idgen.New(), ws, 128)
	h.register(conn)
	metrics.WebSocketConnections.Inc()

	go h.writePump(conn)
	h.readPump(conn)
}

func (h *Hub) register(c *connection.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.conns[c.ID] = c
}

func (h *Hub) unregister(c *connection.Conn) {
	h.mu.Lock()
	delete(h.conns, c.ID)
	var subToClose *userSub
	if c.UserID != "" {
		uk := c.AppID + "|" + c.UserID
		if m := h.byUser[uk]; m != nil {
			delete(m, c.ID)
			if len(m) == 0 {
				delete(h.byUser, uk)
				// Last local connection for this user: drop Redis subscription.
				if sub := h.userSubs[uk]; sub != nil {
					delete(h.userSubs, uk)
					subToClose = sub
				}
			}
		}
	}
	h.mu.Unlock()

	if subToClose != nil {
		// Placeholder may still be empty while Redis Subscribe is in flight.
		if subToClose.cancel != nil {
			subToClose.cancel()
		}
		if subToClose.cleanup != nil {
			subToClose.cleanup()
		}
	}

	if c.Authenticated() && h.presence != nil {
		up := h.presence.SetOffline(c.AppID, c.UserID, c.DeviceID)
		h.broadcastPresence(up)
	}
	if h.bus != nil && c.Authenticated() {
		h.bus.Publish(events.Event{
			Type:     events.DeviceDisconnected,
			AppID:    c.AppID,
			UserID:   c.UserID,
			DeviceID: c.DeviceID,
		})
	}
	metrics.WebSocketConnections.Dec()
	c.Close()
}

func (h *Hub) bindUser(c *connection.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	uk := c.AppID + "|" + c.UserID
	if h.byUser[uk] == nil {
		h.byUser[uk] = make(map[string]*connection.Conn)
	}
	h.byUser[uk][c.ID] = c
}

// ensureUserSubscription starts a Redis user-topic subscription when this node
// gets the first local WebSocket for (appID, userID). Multi-device on the same
// node shares one subscription.
//
// Redis Subscribe is done outside h.mu so lock contention does not stall all
// Hub operations on network I/O. An empty placeholder prevents duplicate
// concurrent subscriptions for the same user.
func (h *Hub) ensureUserSubscription(appID, userID string) {
	if h.redisBus == nil {
		return
	}
	uk := appID + "|" + userID

	h.mu.Lock()
	if _, ok := h.userSubs[uk]; ok {
		h.mu.Unlock()
		return
	}
	// 1. Placeholder under lock: block other conns from double-subscribing.
	placeholder := &userSub{}
	h.userSubs[uk] = placeholder
	h.mu.Unlock()

	// 2. Network I/O outside the Hub lock.
	subCtx, cancel := context.WithCancel(context.Background())
	eventCh, cleanup := h.redisBus.SubscribeUserTopic(subCtx, appID, userID)

	h.mu.Lock()
	// 3. Fill only if our placeholder is still present (not deleted/replaced).
	if sub, ok := h.userSubs[uk]; ok && sub == placeholder {
		sub.cancel = cancel
		sub.cleanup = cleanup
	} else {
		// Unregister removed the slot, or another conn replaced it after a
		// disconnect race. Roll back this Subscribe.
		h.mu.Unlock()
		cancel()
		cleanup()
		return
	}
	h.mu.Unlock()

	// 4. Start fan-out after controllers are installed.
	go h.dispatchUserEvents(subCtx, appID, userID, eventCh)
}

// dispatchUserEvents fans Redis user-topic events out to local WebSocket conns.
func (h *Hub) dispatchUserEvents(ctx context.Context, appID, userID string, eventCh <-chan events.Event) {
	for {
		select {
		case <-ctx.Done():
			return
		case e, ok := <-eventCh:
			if !ok {
				return
			}
			if e.Type != events.WSDeliver {
				continue
			}
			env, err := envelopeFromDeliverEvent(e)
			if err != nil {
				h.logger.Warn("failed to decode ws.deliver envelope",
					"app_id", appID, "user_id", userID, "error", err)
				continue
			}
			h.deliverLocal(appID, userID, e.DeviceID, env)
		}
	}
}

func (h *Hub) writePump(c *connection.Conn) {
	ticker := time.NewTicker(h.pingInterval)
	defer ticker.Stop()
	for {
		select {
		case msg, ok := <-c.SendChan():
			if !ok {
				_ = c.WriteMessage(gorilla.CloseMessage, []byte{})
				return
			}
			if err := c.WriteMessage(gorilla.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			deadline := time.Now().Add(10 * time.Second)
			if err := c.WriteControl(gorilla.PingMessage, []byte("ping"), deadline); err != nil {
				return
			}
		}
	}
}

func (h *Hub) readPump(c *connection.Conn) {
	defer h.unregister(c)
	_ = c.SetReadDeadline(time.Now().Add(h.readTimeout))
	c.SetPongHandler(func(string) error {
		c.Touch()
		_ = c.SetReadDeadline(time.Now().Add(h.readTimeout))
		return nil
	})
	for {
		_, data, err := c.ReadMessage()
		if err != nil {
			return
		}
		c.Touch()
		_ = c.SetReadDeadline(time.Now().Add(h.readTimeout))
		var env protocol.Envelope
		if err := json.Unmarshal(data, &env); err != nil {
			c.Send(protocol.New(protocol.TypeError, "", "", protocol.ErrorData{
				Code: "INVALID_JSON", Message: "invalid message",
			}))
			continue
		}
		h.handle(c, env, data)
	}
}

func (h *Hub) handle(c *connection.Conn, env protocol.Envelope, raw []byte) {
	switch env.Type {
	case protocol.TypeAuthenticate:
		h.handleAuth(c, raw)
	case protocol.TypePing:
		c.Send(protocol.New(protocol.TypePong, env.CallID, c.UserID, nil))
	default:
		if !c.Authenticated() {
			c.Send(protocol.New(protocol.TypeError, env.CallID, "", protocol.ErrorData{
				Code: "UNAUTHORIZED", Message: "authenticate first",
			}))
			return
		}
		if env.Type != protocol.TypeSessionResume && !h.msgLimiter.Allow(c.ID) {
			c.Send(protocol.New(protocol.TypeError, env.CallID, c.UserID, protocol.ErrorData{
				Code: "RATE_LIMITED", Message: "too many signaling messages",
			}))
			return
		}
		h.handleAuthed(c, env)
	}
}

func (h *Hub) handleAuth(c *connection.Conn, raw []byte) {
	var data protocol.AuthenticateData
	_ = json.Unmarshal(raw, &data)
	if data.Token == "" {
		var env protocol.Envelope
		if err := json.Unmarshal(raw, &env); err == nil {
			_ = env.DecodeData(&data)
		}
	}
	claims, err := h.jwt.Validate(data.Token)
	if err != nil {
		c.Send(protocol.New(protocol.TypeError, "", "", protocol.ErrorData{
			Code: "UNAUTHORIZED", Message: "invalid token",
		}))
		return
	}
	deviceID := data.DeviceID
	if deviceID == "" {
		deviceID = claims.DeviceID
	}
	if deviceID == "" {
		deviceID = "device_" + c.ID
	}
	appID := claims.App
	if appID == "" {
		appID = "unknown"
	}
	c.SetIdentity(claims.Subject, appID, deviceID)
	h.bindUser(c)
	h.ensureUserSubscription(appID, claims.Subject)

	if data.Reconnect {
		metrics.WebSocketReconnects.Inc()
	}

	if h.presence != nil {
		up := h.presence.SetOnline(appID, claims.Subject, deviceID)
		h.broadcastPresence(up)
	}
	if h.bus != nil {
		h.bus.Publish(events.Event{
			Type:     events.DeviceConnected,
			AppID:    appID,
			UserID:   claims.Subject,
			DeviceID: deviceID,
		})
	}

	var active any
	if h.calls != nil {
		active = h.calls.ListActiveForUser(context.Background(), appID, claims.Subject)
	}
	c.Send(protocol.New(protocol.TypeAuthenticated, "", claims.Subject, protocol.AuthenticatedData{
		UserID:      claims.Subject,
		AppID:       appID,
		DeviceID:    deviceID,
		Reconnect:   data.Reconnect,
		ActiveCalls: active,
	}))
}

func (h *Hub) handleAuthed(c *connection.Conn, env protocol.Envelope) {
	ctx := context.Background()
	switch env.Type {
	case protocol.TypeCallCreate:
		h.handleCallCreate(c, env)
	case protocol.TypeCallAccept:
		h.handleAccept(c, env)
	case protocol.TypeCallReject:
		h.wrapCall(ctx, c, env, func(ctx context.Context, callID string) (*call.Call, error) {
			return h.calls.Reject(ctx, callID, c.UserID)
		})
	case protocol.TypeCallCancel:
		h.wrapCall(ctx, c, env, func(ctx context.Context, callID string) (*call.Call, error) {
			c2, err := h.calls.Cancel(ctx, callID, c.UserID)
			if err == nil && h.incoming != nil {
				h.incoming.NotifyCancelled(context.Background(), c2, "cancelled")
			}
			return c2, err
		})
	case protocol.TypeCallJoin:
		h.wrapCall(ctx, c, env, func(ctx context.Context, callID string) (*call.Call, error) {
			return h.calls.Join(ctx, callID, c.UserID, c.DeviceID)
		})
	case protocol.TypeCallLeave:
		h.wrapCall(ctx, c, env, func(ctx context.Context, callID string) (*call.Call, error) {
			return h.calls.Leave(ctx, callID, c.UserID)
		})
	case protocol.TypeCallHangup:
		h.wrapCall(ctx, c, env, func(ctx context.Context, callID string) (*call.Call, error) {
			return h.calls.Hangup(ctx, callID, c.UserID)
		})
	case protocol.TypeCallResume, protocol.TypeSessionResume:
		h.handleResume(c, env)
	case protocol.TypeWebRTCOffer, protocol.TypeWebRTCAnswer, protocol.TypeWebRTCICECandidate,
		protocol.TypeMediaMute, protocol.TypeMediaUnmute, protocol.TypeMediaCameraOn, protocol.TypeMediaCameraOff:
		h.relaySignaling(ctx, c, env)
	default:
		c.Send(protocol.New(protocol.TypeError, env.CallID, c.UserID, protocol.ErrorData{
			Code: "UNKNOWN_TYPE", Message: "unknown message type",
		}))
	}
}

type createCallData struct {
	CalleeIDs  []string `json:"callee_ids"`
	CalleeID   string   `json:"callee_id"`
	Type       string   `json:"type"`
	Mode       string   `json:"mode"`
	CallerName string   `json:"caller_name"`
}

func (h *Hub) handleCallCreate(c *connection.Conn, env protocol.Envelope) {
	if !h.callLimiter.Allow(c.AppID + "|" + c.UserID) {
		c.Send(protocol.New(protocol.TypeError, env.CallID, c.UserID, protocol.ErrorData{
			Code: "RATE_LIMITED", Message: "too many call creations",
		}))
		return
	}
	var data createCallData
	_ = env.DecodeData(&data)
	callees := data.CalleeIDs
	if len(callees) == 0 && data.CalleeID != "" {
		callees = []string{data.CalleeID}
	}
	created, err := h.calls.Create(context.Background(), call.CreateInput{
		AppID:        c.AppID,
		CallerID:     c.UserID,
		CallerDevice: c.DeviceID,
		CalleeIDs:    callees,
		Type:         data.Type,
		Mode:         data.Mode,
		CallerName:   data.CallerName,
	})
	if err != nil {
		h.sendErr(c, env.CallID, err)
		return
	}
	metrics.CallsCreatedTotal.WithLabelValues(c.AppID).Inc()
	h.broadcastCall(created, protocol.TypeCallCreated, c.UserID)
	h.broadcastCall(created, protocol.TypeCallRinging, c.UserID)
	if h.incoming != nil {
		go h.incoming.NotifyRinging(context.Background(), created, data.CallerName)
	}
}

func (h *Hub) handleAccept(c *connection.Conn, env protocol.Envelope) {
	callID := h.callIDFrom(env)
	if callID == "" {
		h.sendErr(c, "", errMsg("call_id required"))
		return
	}
	updated, err := h.calls.Accept(context.Background(), callID, c.UserID, c.DeviceID)
	if err != nil {
		h.sendErr(c, callID, err)
		return
	}
	h.broadcastCall(updated, protocol.TypeCallAccepted, c.UserID)
	// Multi-device: stop ringing on other devices of the accepting user.
	stop := protocol.New(protocol.TypeCallStopRinging, updated.ID, c.UserID, protocol.StopRingingData{
		CallID:         updated.ID,
		AcceptedDevice: c.DeviceID,
		Reason:         "accepted_elsewhere",
	})
	h.sendToUserExceptDevice(updated.AppID, c.UserID, c.DeviceID, stop)
	if h.incoming != nil {
		go h.incoming.NotifyStopRinging(context.Background(), updated, c.UserID, "accepted_elsewhere")
	}
}

func (h *Hub) handleResume(c *connection.Conn, env protocol.Envelope) {
	callID := h.callIDFrom(env)
	if callID == "" {
		h.sendErr(c, "", errMsg("call_id required"))
		return
	}
	metrics.WebSocketReconnects.Inc()
	snapshot, err := h.calls.ResumeFull(context.Background(), callID, c.UserID, c.DeviceID)
	if err != nil {
		h.sendErr(c, callID, err)
		return
	}
	c.Send(protocol.New(protocol.TypeCallState, snapshot.CallID, c.UserID, snapshot))
	if snapshot.Call != nil {
		h.broadcastCall(snapshot.Call, protocol.TypeCallParticipantJoined, c.UserID)
	}
}

func (h *Hub) wrapCall(ctx context.Context, c *connection.Conn, env protocol.Envelope, fn func(context.Context, string) (*call.Call, error)) {
	callID := h.callIDFrom(env)
	if callID == "" {
		h.sendErr(c, "", errMsg("call_id required"))
		return
	}
	updated, err := fn(ctx, callID)
	if err != nil {
		h.sendErr(c, callID, err)
		return
	}
	typ := mapCallEvent(env.Type)
	h.broadcastCall(updated, typ, c.UserID)
}

func (h *Hub) callIDFrom(env protocol.Envelope) string {
	callID := env.CallID
	if callID == "" {
		var d struct {
			CallID string `json:"call_id"`
		}
		_ = env.DecodeData(&d)
		callID = d.CallID
	}
	return callID
}

func mapCallEvent(clientType string) string {
	switch clientType {
	case protocol.TypeCallAccept:
		return protocol.TypeCallAccepted
	case protocol.TypeCallReject:
		return protocol.TypeCallRejected
	case protocol.TypeCallCancel:
		return protocol.TypeCallCancelled
	case protocol.TypeCallJoin:
		return protocol.TypeCallParticipantJoined
	case protocol.TypeCallLeave:
		return protocol.TypeCallParticipantLeft
	case protocol.TypeCallHangup:
		return protocol.TypeCallEnded
	default:
		return clientType
	}
}

func (h *Hub) relaySignaling(ctx context.Context, c *connection.Conn, env protocol.Envelope) {
	if env.CallID == "" || h.calls == nil {
		h.sendErr(c, env.CallID, errMsg("call_id required"))
		return
	}
	cl, err := h.calls.Get(ctx, env.CallID)
	if err != nil {
		h.sendErr(c, env.CallID, err)
		return
	}
	if cl.FindParticipant(c.UserID) == nil {
		h.sendErr(c, env.CallID, errMsg("not a participant"))
		return
	}
	out := protocol.New(env.Type, env.CallID, c.UserID, json.RawMessage(env.Data))
	for _, p := range cl.Participants {
		if p.UserID == c.UserID {
			continue
		}
		h.sendToUser(cl.AppID, p.UserID, out)
	}
}

func (h *Hub) broadcastCall(c *call.Call, typ, sender string) {
	if c == nil {
		return
	}
	env := protocol.New(typ, c.ID, sender, c)
	for _, p := range c.Participants {
		h.sendToUser(c.AppID, p.UserID, env)
	}
}

func (h *Hub) broadcastPresence(up presence.UserPresence) {
	env := protocol.New(protocol.TypePresenceUpdated, "", up.UserID, up)
	h.sendToUser(up.AppID, up.UserID, env)
}

func (h *Hub) sendToUser(appID, userID string, env protocol.Envelope) {
	h.routeToUser(appID, userID, "", env)
}

func (h *Hub) sendToUserExceptDevice(appID, userID, exceptDevice string, env protocol.Envelope) {
	h.routeToUser(appID, userID, exceptDevice, env)
}

// routeToUser publishes to Redis user topic when configured; otherwise delivers locally.
// Redis Pub/Sub echoes to all subscribers (including this node), so local devices still receive.
func (h *Hub) routeToUser(appID, userID, exceptDevice string, env protocol.Envelope) {
	if h.redisBus == nil {
		h.deliverLocal(appID, userID, exceptDevice, env)
		return
	}
	envBytes, err := json.Marshal(env)
	if err != nil {
		h.logger.Error("marshal envelope for redis failed", "error", err)
		h.deliverLocal(appID, userID, exceptDevice, env)
		return
	}
	e := events.Event{
		Type:     events.WSDeliver,
		AppID:    appID,
		UserID:   userID,
		DeviceID: exceptDevice, // except-device filter for multi-device stop-ringing
		Payload: map[string]any{
			"envelope": string(envBytes),
		},
	}
	if err := h.redisBus.Publish(context.Background(), events.UserTopic(appID, userID), e); err != nil {
		h.logger.Error("redis publish failed; falling back to local delivery",
			"app_id", appID, "user_id", userID, "error", err)
		h.deliverLocal(appID, userID, exceptDevice, env)
	}
}

func (h *Hub) deliverLocal(appID, userID, exceptDevice string, env protocol.Envelope) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, c := range h.byUser[appID+"|"+userID] {
		if exceptDevice != "" && c.DeviceID == exceptDevice {
			continue
		}
		c.Send(env)
	}
}

func envelopeFromDeliverEvent(e events.Event) (protocol.Envelope, error) {
	raw, ok := e.Payload["envelope"]
	if !ok || raw == nil {
		return protocol.Envelope{}, errMsg("missing envelope in ws.deliver payload")
	}
	var b []byte
	switch v := raw.(type) {
	case string:
		b = []byte(v)
	case json.RawMessage:
		b = []byte(v)
	case []byte:
		b = v
	default:
		var err error
		b, err = json.Marshal(v)
		if err != nil {
			return protocol.Envelope{}, err
		}
	}
	var env protocol.Envelope
	if err := json.Unmarshal(b, &env); err != nil {
		return protocol.Envelope{}, err
	}
	return env, nil
}

func (h *Hub) onEvent(e events.Event) {
	switch e.Type {
	case events.CallEnded:
		metrics.CallsEndedTotal.WithLabelValues(e.AppID).Inc()
	case events.CallConnected:
		metrics.CallsConnectedTotal.WithLabelValues(e.AppID).Inc()
	case events.CallFailed:
		metrics.CallsFailedTotal.WithLabelValues(e.AppID).Inc()
	case events.CallAccepted:
		// Hub handles stop-ringing on the accept path; event reserved for metrics/extensions.
	}
}

func (h *Hub) sendErr(c *connection.Conn, callID string, err error) {
	code, msg := "ERROR", err.Error()
	c.Send(protocol.New(protocol.TypeError, callID, c.UserID, protocol.ErrorData{Code: code, Message: msg}))
}

type simpleErr string

func (e simpleErr) Error() string { return string(e) }
func errMsg(s string) error       { return simpleErr(s) }

func (h *Hub) ActiveConnections() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.conns)
}
