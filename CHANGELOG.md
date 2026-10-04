# Changelog

## v0.2.1

Fixes (found by new offline fake-proxy tests):
- WSS over a `socks5://` / `socks5h://` proxy: the dialer spoke HTTP CONNECT to the SOCKS port. Now runs the
  SOCKS5 handshake (RFC 1928, user/pass auth per RFC 1929); unknown proxy schemes fail loudly.
- WSS ignored `HTTPS_PROXY` / `ALL_PROXY` when no proxy was configured; it now uses the same env fallback as HTTP.
- `Reconnecting` / `Disconnected` were sent non-blocking and could be dropped on a full event buffer;
  lifecycle events are now always delivered while the stream is live, and `Disconnected` exactly once.

Tests: fake CONNECT + SOCKS5 proxies (ttwid, API, WSS, credentials, env), fake webcast WS server
(heartbeat, enter_room, ack log_id + internal_ext, UA / cookies / locale / compress / heartbeat_duration on the wire),
client-level reconnect loop (Reconnecting×N → Disconnected once, rotation, cancel).

## v0.2.0

Breaking:
- `WebcastRoomUserSeqMessage.Total` → `ViewerCount` (tag 3, current viewers); `Contributor.Score`/`Rank` int32 → int64.
- `RoomIDResult` gains `AnchorID` (check_online result shape).
- `connection.BuildWSSURL(..., compress, heartbeatInterval)` and
  `connection.RunWebSocket(ctx, url, cookie, ua, roomID, heartbeatInterval, staleTimeout, ...)` take the heartbeat interval.
- Replay tests fail (instead of skip) when testdata is missing.

Changes:

- ttwid fetch retries up to 8× (750 ms apart) when TikTok omits the cookie; transport errors still propagate.
- Reconnect loop: a ttwid failure is a failed attempt (`EventReconnecting`, backoff) instead of silently ending the stream.
- ttwid + UA are reused across reconnects and rotated only on DEVICE_BLOCKED or a connection that died within 30 s.
- `MaxRetries` counts consecutive failures; a 30 s healthy session resets the count.
- `HeartbeatInterval` builder (default 10 s), also fed into the `heartbeat_duration` WSS URL param (ms).
  `connection.BuildWSSURL` / `connection.RunWebSocket` take the interval; the heartbeat goroutine now stops with the read loop.
- `RoomIDResult.AnchorID` (streamer user ID from `/api-live/user/room`); `RoomInfo.RawJSON`.
- `WebcastRoomUserSeqMessage.Total` renamed to `ViewerCount` (live-go#1: it is the current viewer count, tag 3;
  `TotalUser` is unique viewers over the stream). `Contributor.Score`/`Rank` are now `int64`. New `TopViewers()`.
- Gift helpers on `WebcastGiftMessage`: `IsComboGift()`, `IsStreakOver()`, `DiamondTotal()`.
- `FetchRoomAudience` (full viewer roster, login-gated) + `SessionRequiredError`; `cmd/audience` example.
- Replay tests fail on missing testdata instead of skipping, and pin RoomUserSeq viewer counts.
