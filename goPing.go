// Program: goPing
//
// Description: A simple CLI ping application to send echo requests and receive
// echo replies in an infinite loop.
//
// Attributes:
// 1) Written in the Go programming language
// 2) CLI interface with positional argument for hostname/IP address
// 3) Sends ICMP "echo requests" in an infinite loop
// 4) Reports loss and RTT times for each message
// 5) Handles both IPv4 and IPv6 (with flag)

package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

func main() {
	// Remove timestamp from log
	log.SetFlags(log.Flags() &^ (log.Ldate | log.Ltime))

	ipVersion := flag.Int(
		"ipv",
		4,
		"4 or 6, corresponding to which IP version to use")
	flag.Parse()

	wantIPv6 = *ipVersion == 6

	if wantIPv6 {
		log.Printf("Using IPv6\n")
	} else {
		log.Printf("Using IPv4\n")
	}

	var address string

	if flag.NArg() == 0 {
		log.Printf("No IP/hostname specified. Defaulting to cloudflare.com\n")
		address = "cloudflare.com"
	} else if flag.NArg() > 1 {
		log.Printf("Please enter only one IP/hostname as a positional argument\n")
		os.Exit(1)
	} else {
		address = flag.Arg(0)
	}

	stats := new(statistic)
	for {
		logIPAddress, logErr := stats.ping(address)
		if logErr != nil {
			stats.count++
			stats.lost++
			log.Printf("ERROR: %s\n", logErr)
		} else {
			stats.count++
		}
		stats.loss = (float64(stats.lost) / float64(stats.count)) * 100.0
		log.Printf(
			"Seq: %d\t\tPinging: %s\t\tRTT: %s\t\tLoss: %.2f%%\n",
			stats.count,
			logIPAddress,
			stats.rtt,
			stats.loss)
		time.Sleep(time.Second)
	}
}

const (
	listenNetwork4  string = "ip4:icmp"
	listenNetwork6  string = "ip6:ipv6-icmp"
	listenAddress4  string = "0.0.0.0"
	listenAddress6  string = "::"
	resolveNetwork4 string = "ip4"
	resolveNetwork6 string = "ip6"
	protocolICMP4   int    = 1
	protocolICMP6   int    = 58
)

var (
	wantIPv6 bool
)

type statistic struct {
	count    int
	sent     int
	received int
	lost     int
	rtt      time.Duration
	loss     float64
}

func (stats *statistic) ping(address string) (*net.IPAddr, error) {
	stats.rtt = 0 // Reset rtt in case error causes return before update

	var (
		listenNetwork  string
		listenAddress  string
		resolveNetwork string
		messageType    icmp.Type
		//replyType      icmp.Type
		protocolICMP int
	)

	if wantIPv6 {
		listenNetwork = listenNetwork6
		listenAddress = listenAddress6
		resolveNetwork = resolveNetwork6
		messageType = ipv6.ICMPTypeEchoRequest
		protocolICMP = protocolICMP6

	} else {
		listenNetwork = listenNetwork4
		listenAddress = listenAddress4
		resolveNetwork = resolveNetwork4
		messageType = ipv4.ICMPTypeEcho
		protocolICMP = protocolICMP4
	}

	listenPacket, err := icmp.ListenPacket(listenNetwork, listenAddress)
	if err != nil {
		return nil, err
	}
	defer listenPacket.Close()

	ipAddress, err := net.ResolveIPAddr(resolveNetwork, address)
	if err != nil {
		return nil, err
	}

	request := icmp.Message{
		Type: messageType,
		Code: 0,
		Body: &icmp.Echo{
			ID:   os.Getpid() & 0xffff,
			Seq:  stats.count,
			Data: []byte("PLS-GIB-INTERNSHIP"),
		},
	}
	requestEncoded, err := request.Marshal(nil)
	if err != nil {
		return ipAddress, err
	}

	timeSent := time.Now()
	if _, err := listenPacket.WriteTo(requestEncoded, ipAddress); err != nil {
		return ipAddress, err
	}
	replyEncoded := make([]byte, 1000)
	err = listenPacket.SetReadDeadline(time.Now().Add(3 * time.Second))
	if err != nil {
		return ipAddress, err
	}

	replyRead, _, err := listenPacket.ReadFrom(replyEncoded)
	stats.rtt = time.Since(timeSent).Round(10 * time.Microsecond)

	reply, err := icmp.ParseMessage(protocolICMP, replyEncoded[:replyRead])
	if err != nil {
		return ipAddress, err
	}
	switch reply.Type {
	case ipv4.ICMPTypeEchoReply, ipv6.ICMPTypeEchoReply:
		stats.received++
		return ipAddress, nil
	default:
		return ipAddress, fmt.Errorf("reply type is %s", reply.Type)

	}
}
