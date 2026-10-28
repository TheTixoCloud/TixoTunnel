// Linux core entry point by ir_spoof. https://t.me/ir_spoof
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"backhaul.local/ir_spoof/internal/config"
	"backhaul.local/ir_spoof/internal/core"
	"backhaul.local/ir_spoof/internal/ipx"
)

var (
	buildVersion = "dev"
	commit       = "unknown"
	buildDate    = "unknown"
)

func main() {
	args := flag.NewFlagSet("backhaul", flag.ContinueOnError)
	args.SetOutput(os.Stderr)
	var file string
	var check, version, short bool
	args.StringVar(&file, "c", "", "local TOML/JSON configuration file")
	args.BoolVar(&check, "check", false, "validate local configuration without starting network or changing interfaces")
	args.BoolVar(&version, "version", false, "print independent build identity")
	args.BoolVar(&short, "v", false, "print independent build identity")
	if err := args.Parse(os.Args[1:]); err != nil {
		if err == flag.ErrHelp {
			return
		}
		os.Exit(2)
	}
	if version || short {
		fmt.Printf("TixoTunnel %s (commit %s, built %s) %s/%s; independent reconstruction\n",
			buildVersion, commit, buildDate, runtime.GOOS, runtime.GOARCH)
		return
	}
	if args.NArg() != 0 || file == "" {
		fmt.Fprintln(os.Stderr, "usage: backhaul -c CONFIG [-check] | -version")
		os.Exit(2)
	}
	c, err := config.Load(file)
	if err == nil {
		_, err = core.ParseMappings(c.Ports.Mapping)
	}
	if err == nil && c.IsIPX() {
		err = ipx.Validate(ipx.Config{Profile: c.IPX.Profile, Role: c.IPX.Mode, Interface: c.IPX.Interface, LocalIP: c.IPX.ListenIP, PeerIP: c.IPX.DstIP, SpoofSrcIP: c.IPX.SpoofSrcIP, SpoofDstIP: c.IPX.SpoofDstIP, LocalPort: c.IPX.LocalPort, PeerPort: c.IPX.PeerPort, Protocol: c.IPX.Protocol, TUNName: c.TUN.Name, LocalTUN: c.TUN.LocalAddr, PeerTUN: c.TUN.RemoteAddr, Routes: c.TUN.Routes, MTU: c.TUN.MTU, OuterMTU: c.IPX.OuterMTU, Queues: c.IPX.Queues, Key: c.Security.PSK, Algorithm: c.Security.Algorithm, KDFIterations: c.Security.KDFIterations, AllowInsecurePlaintext: c.Security.AllowInsecurePlaintext && !c.Encrypted()})
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "backhaul: configuration invalid:", err)
		os.Exit(2)
	}
	if check {
		fmt.Println("configuration valid; no network activity")
		return
	}
	if runtime.GOOS != "linux" {
		fmt.Fprintln(os.Stderr, "backhaul: run this core on Linux")
		os.Exit(2)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	fmt.Println("Backhaul ir_spoof ready; local configuration only")
	if err = core.Run(ctx, c); err != nil && ctx.Err() == nil {
		fmt.Fprintln(os.Stderr, "backhaul:", err)
		os.Exit(1)
	}
}
