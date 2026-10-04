# Changelog

## v0.2.0

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
