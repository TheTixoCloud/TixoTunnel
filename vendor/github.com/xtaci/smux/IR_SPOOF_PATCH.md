<!-- Local dependency repair by ir_spoof; https://t.me/ir_spoof. -->
# Local SMUX repair

The vendored v1.5.57 stream implementation retains unread response buffers when
both peers send FIN. Cleanup occurs after Read or WriteTo drains them. Without
this repair, a delayed reader can lose every byte of a response after closing
its request direction. WriteTo also reports a drained EOF as successful copy
completion. The frame format is unchanged. The original dependency
license and author credit are preserved. The owned regression is
`internal/transport/smux_halfclose_test.go`.
