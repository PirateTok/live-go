<p align="center">
  <img src="https://raw.githubusercontent.com/PirateTok/.github/main/profile/assets/og-banner-v2.png" alt="PirateTok" width="640" />
</p>

# PirateTok Go Live

Connect to any TikTok Live stream and receive real-time events in Go. No signing server, no API keys, no authentication required.

```go
package main

import (
    "context"
    "fmt"
    golive "github.com/PirateTok/live-go"
    "github.com/PirateTok/live-go/events"
    pb "github.com/PirateTok/live-go/proto"
)

func main() {
    // Connect returns a channel of decoded events
    ch, err := golive.NewClient("username_here").Connect(context.Background())
    if err != nil {
        panic(err)
    }

    // Events arrive as fully decoded protobuf messages
    for evt := range ch {
        switch evt.Type {
        case events.EventConnected:
            fmt.Println("connected to room:", evt.RoomID)
        case events.EventChat:
            msg := evt.Data.(*pb.WebcastChatMessage)
            fmt.Printf("[chat] %s: %s\n", msg.User.Nickname, msg.Content)
        case events.EventGift:
            msg := evt.Data.(*pb.WebcastGiftMessage)
            giftName := fmt.Sprintf("gift#%d", msg.GiftId)
            if msg.Gift != nil {
                giftName = msg.Gift.Name
            }
            fmt.Printf("[gift] %s sent %s x%d\n",
                msg.User.Nickname, giftName, msg.RepeatCount)
        case events.EventDisconnected:
            fmt.Println("disconnected")
            return
        }
    }
}
```

## Install

```
go get github.com/PirateTok/live-go
```

## Other languages

| Language | Install | Repo |
|:---------|:--------|:-----|
| **Rust** | `cargo add piratetok-live-rs` | [live-rs](https://github.com/PirateTok/live-rs) |
| **Python** | `pip install piratetok-live-py` | [live-py](https://github.com/PirateTok/live-py) |
| **JavaScript** | `npm install piratetok-live-js` | [live-js](https://github.com/PirateTok/live-js) |
| **C#** | `dotnet add package PirateTok.Live` | [live-cs](https://github.com/PirateTok/live-cs) |
| **Java** | `com.piratetok:live` | [live-java](https://github.com/PirateTok/live-java) |
| **Lua** | `luarocks install piratetok-live-lua` | [live-lua](https://github.com/PirateTok/live-lua) |
| **Elixir** | `{:piratetok_live, "~> 0.1"}` | [live-ex](https://github.com/PirateTok/live-ex) |
| **Dart** | `dart pub add piratetok_live` | [live-dart](https://github.com/PirateTok/live-dart) |
| **C** | `#include "piratetok.h"` | [live-c](https://github.com/PirateTok/live-c) |
| **PowerShell** | `Install-Module PirateTok.Live` | [live-ps1](https://github.com/PirateTok/live-ps1) |
| **Shell** | `bpkg install PirateTok/live-sh` | [live-sh](https://github.com/PirateTok/live-sh) |

## Features

- **Zero signing dependency** — no API keys, no signing server, no external auth
- **64 decoded event types** — committed protobuf codegen, no build-time protoc
- **Auto-reconnection** — stale detection, exponential backoff, self-healing auth
- **Full proxy support** — HTTP/HTTPS/SOCKS5 proxy for all HTTP and WSS connections, env var fallback
- **Enriched User data** — badges, gifter level, moderator status, follow info, fan club
- **Sub-routed convenience events** — `EventFollow`, `EventShare`, `EventJoin`, `EventLiveEnded`
- **2 dependencies** — `gobwas/ws` + `google.golang.org/protobuf`

## Configuration

```go
client := golive.NewClient("username_here").
    CdnEU().
    MaxRetries(10).
    StaleTimeout(90 * time.Second)
```

| Method | Default | Description |
|:-------|:--------|:------------|
| `.CdnEU()` | global | Use the EU CDN endpoint |
| `.CdnUS()` | global | Use the US CDN endpoint |
| `.Cdn(host)` | `"webcast-ws.tiktok.com"` | Custom CDN host |
| `.Timeout(d)` | `10s` | HTTP timeout for API calls |
| `.MaxRetries(n)` | `5` | Max reconnection attempts |
| `.StaleTimeout(d)` | `60s` | Close and reconnect after no data for this duration |
| `.UserAgent(ua)` | random pool | Override the random UA pool with a fixed UA |
| `.Cookies(cookies)` | none | Append session cookies alongside ttwid in the WSS cookie header |
| `.Language(lang)` | system locale | Override detected language (e.g. `"pt"`, `"ro"`) |
| `.Region(reg)` | system locale | Override detected region (e.g. `"BR"`, `"RO"`) |
| `.Proxy(url)` | env vars | HTTP/HTTPS/SOCKS5 proxy URL for all HTTP and WSS connections |
| `.Compress(enabled)` | `true` | Disable gzip compression for WSS payloads (trades bandwidth for CPU) |

## Room info (optional, separate call)

```go
info, err := golive.FetchRoomInfo("ROOM_ID", 10*time.Second, "")

// 18+ rooms
info, err := golive.FetchRoomInfo("ROOM_ID", 10*time.Second, "sessionid=abc; sid_tt=abc")
```

## Examples

```bash
go run ./cmd/basic_chat <username>        # connect + print chat events
go run ./cmd/online_check <username>      # check if user is live
go run ./cmd/stream_info <username>       # fetch room metadata + stream URLs
go run ./cmd/gift_tracker <username>      # track gifts with diamond totals
go run ./cmd/gift_streak <username>       # track gift streaks with delta computation
go run ./cmd/profile_lookup [username]    # fetch profile via SIGI scraper + cache
```

## Replay testing

Deterministic cross-lib validation against binary WSS captures. Requires testdata from a separate repo:

```bash
git clone https://github.com/PirateTok/live-testdata testdata
go test -run TestReplay -v
```

Tests skip gracefully if testdata is not found. You can also set `PIRATETOK_TESTDATA` to point to a custom location.

## License

0BSD
