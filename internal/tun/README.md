<!-- Independent implementation: ir_spoof — https://t.me/ir_spoof -->
# Linux TUN ownership

`Open(context.Context, Config)` creates a new exclusive, nonpersistent TUN.
`Config` contains `Name`, local CIDR `Local`, optional peer address `Peer`, `MTU`,
`Queues` and additional `Routes`. The default MTU is 1320 and queue count is one.
`Device.Read` and `Write` use queue zero; `Queue(index)` exposes each queue for
callers implementing concurrent multiqueue processing.

Names, addresses, MTUs, queue counts and route prefixes are validated before
device creation. An existing name is rejected, including the kernel race check
through `IFF_TUN_EXCL`. Routes are added without `replace` and carry protocol
marker 253. Failed setup closes the new device without replacing an unrelated
route. Closing all owned descriptors removes the interface and its device-bound
routes; cancellation also closes it and wakes pending reads.

The implementation calls only conventional system `iproute2` paths with separate
arguments and a bounded timeout. It changes no global forwarding, firewall or
system tuning settings. Linux tests require an explicitly admitted disposable
network namespace and `BACKHAUL_TUN_LAB=1` before any privileged fixture runs.

The low-level ownership implementation is reused from the team's independent
MIT-licensed Netrix reconstruction, with a new context-aware generic API for
this project. Attribution: ir_spoof, https://t.me/ir_spoof.
