// Independent Linux TUN ownership and cancellation by ir_spoof; https://t.me/ir_spoof.
package tun

// Config creates an exclusive nonpersistent TUN. Peer is a literal IP or prefix;
// its host route is added without replacing any existing route. Queues defaults
// to one; generic Read and Write use queue zero.
type Config struct {
	Name, Local, Peer string
	MTU, Queues       int
	Routes            []string
}
