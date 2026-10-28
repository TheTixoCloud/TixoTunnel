<!-- Independent Backhaul reconstruction by ir_spoof - https://t.me/ir_spoof -->

# Physical carriers

This package creates connections for the independent core. The core adds peer
authentication, encrypted records and session multiplexing after connection
establishment. Matching a carrier does not establish compatibility with the
original licensed application protocol.

| Kind | Physical connection |
| --- | --- |
| `tcp`, `tcpmux`, `xtcpmux`, `tun` | TCP stream |
| `ws`, `wss` | RFC 6455 binary message stream, plain TCP or verified TLS, `/tunnel` |
| `wsmux`, `wssmux` | HTTP/1.1 WebSocket Upgrade at `/tunnel`, then an **unframed** byte stream, plain TCP or verified TLS |
| `xwsmux` | HTTP/1.1 WebSocket Upgrade at `/data`, then an **unframed** byte stream |
| `anytls` | Verified browser-shaped TLS using uTLS Chrome; this is Backhaul's label and does not implement the separate AnyTLS application protocol |

`Listen(context.Context, Options)` returns a listener; `Dial(context.Context,
Options)` returns a connection. `Options.LocalAddr` accepts a literal local IP
or IP with a source port (`IP:port`, `[IPv6]:port`). Every dial checks that the IP
is assigned to an active interface. This binds a real local address; raw IPX
packet source spoofing belongs to the separate IPX package.

`EdgeIP` changes only the physical dial destination. The configured peer keeps
its HTTP Host and TLS identity. No environment HTTP proxy, vendor lookup,
license endpoint or telemetry endpoint is used. Resolving a configured peer or
edge hostname is normal operator-requested DNS activity.

TLS verification defaults on. The operator may provide a local CA/certificate
file for a self-signed deployment. Certificate/key files are required on TLS
listeners. Handshakes default to a 10-second limit. Listeners permit at most 64
concurrent carrier handshakes and retain at most 64 completed connections for
the application's accept loop; slow handshakes do not serialize that loop.

Upgrade request and response headers are limited to 16 KiB, require HTTP/1.1,
validate the WebSocket challenge and path, and preserve all buffered application
bytes. Framed WebSockets require binary messages and correct role masking,
handle continuation and ping/pong/close frames, and reject compression and text
payloads. Writes are serialized into messages of at most 32 KiB. Incoming
messages are limited to 16 MiB and streamed without whole-message allocation.
Framing errors terminate the connection. Carrier establishment has its own
deadlines; application record deadlines and lifetime are the core's concern.

The tests use independently generated certificates, loopback sockets and
Gorilla WebSocket peers as an interoperability oracle for framing, fragmentation,
ping/pong, masking and close behavior. They never execute the supplied premium
binary.
