// Bounded datagram forwarding by ir_spoof. https://t.me/ir_spoof
package core

import (
	"errors"
	"net"
	"sync"
	"time"

	"backhaul.local/ir_spoof/internal/wire"
)

type udpFlow struct {
	conn   net.Conn
	source *net.UDPAddr
}

func (e *Engine) startUDP(mapping Mapping, overTUN bool) error {
	address, err := net.ResolveUDPAddr("udp", mapping.Bind)
	if err != nil {
		return err
	}
	listener, err := net.ListenUDP("udp", address)
	if err != nil {
		return errors.New("UDP forwarding listener could not start")
	}
	e.track(listener)
	var mu sync.Mutex
	flows := map[string]*udpFlow{}
	e.launch(func() {
		buffer := make([]byte, 65507)
		for {
			n, source, err := listener.ReadFromUDP(buffer)
			if err != nil {
				return
			}
			key := source.String()
			mu.Lock()
			flow := flows[key]
			full := len(flows) >= e.cfg.Tuning.MaxConnections
			mu.Unlock()
			if flow == nil {
				if full {
					e.stats.Rejected.Add(1)
					continue
				}
				select {
				case e.slots <- struct{}{}:
				default:
					e.stats.Rejected.Add(1)
					continue
				}
				var c net.Conn
				if overTUN {
					c, err = e.backend("udp", e.tunTarget(mapping.Target))
					if err == nil {
						e.track(c)
					}
				} else {
					c, err = e.open(command{Kind: "udp", Target: mapping.Target})
				}
				if err != nil {
					<-e.slots
					continue
				}
				flow = &udpFlow{conn: c, source: source}
				mu.Lock()
				flows[key] = flow
				mu.Unlock()
				current := flow
				e.launch(func() {
					defer func() {
						<-e.slots
						e.release(current.conn)
						mu.Lock()
						if flows[key] == current {
							delete(flows, key)
						}
						mu.Unlock()
					}()
					replyBuffer := make([]byte, 65507)
					for {
						current.conn.SetReadDeadline(time.Now().Add(time.Duration(e.cfg.UDP.PeerIdleTimeout) * time.Second))
						var reply []byte
						var err error
						if overTUN {
							var count int
							count, err = current.conn.Read(replyBuffer)
							reply = replyBuffer[:count]
						} else {
							reply, err = wire.ReadFrame(current.conn, 65507)
						}
						if err != nil {
							return
						}
						if _, err = listener.WriteToUDP(reply, current.source); err != nil {
							return
						}
					}
				})
			}
			// The timeout belongs only to this UDP flow; it never changes a global route.
			flow.conn.SetReadDeadline(time.Now().Add(time.Duration(e.cfg.UDP.PeerIdleTimeout) * time.Second))
			flow.conn.SetWriteDeadline(time.Now().Add(time.Duration(e.cfg.Dialer.DialTimeout) * time.Second))
			if overTUN {
				_, err = flow.conn.Write(buffer[:n])
			} else {
				err = wire.WriteFrame(flow.conn, buffer[:n])
			}
			flow.conn.SetWriteDeadline(time.Time{})
			if err != nil {
				e.release(flow.conn)
				mu.Lock()
				if flows[key] == flow {
					delete(flows, key)
				}
				mu.Unlock()
			}
		}
	})
	return nil
}
