// Program: goPing
//
// Description: A simple CLI ping application to send echo requests and receive
// echo replies in an infinite loop, unless specified.
//
// Attributes:
// 1) Written in the Go programming language
// 2) CLI interface with positional argument for hostname/IP address
// 3) Sends ICMP "echo requests" in an infinite loop
// 4) Reports loss and RTT times for each message
// 5) Handles both IPv4 and IPv6 (with flag)
// 6) Supports setting TTL with time exceeded messages (flag)
// 7) Supports finite number of pings (with flag)
// 8) Supports calculating jitter
// 9) Supports displaying summary or statistics upon termination

package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

func main() {
	// Remove timestamp from log
	log.SetFlags(log.Flags() &^ (log.Ldate | log.Ltime))

	stats := new(statistic)
	stats.closeHandler()

	ipVersion := flag.Int(
		"ipv",
		4,
		"4 or 6, corresponding to which IP version to use")
	pingCount := flag.Int(
		"c",
		-1,
		"Finite number of times to ping, -1 being infinite")
	timeToLive := flag.Int(
		"ttl",
		64,
		"Time-to-live before package expires")
	flag.Parse()

	if *pingCount < -1 {
		log.Printf("Times to ping must be positive int, or -1 for infinite. Defaulting to infinite...")
		*pingCount = -1
	}
	if *timeToLive < 0 {
		log.Printf("Invalid TTL. Defaulting to 64...")
		*timeToLive = 64
	}
	ttl = *timeToLive

	if wantIPv6 = *ipVersion == 6; wantIPv6 {
		log.Printf("Using IPv6...\n")
	} else {
		log.Printf("Using IPv4...\n")
	}

	var address string

	if flag.NArg() == 0 {
		log.Printf("No IP/hostname specified. Defaulting to cloudflare.com...\n")
		address = "cloudflare.com"
	} else if flag.NArg() > 1 {
		log.Printf("Please enter only one IP/hostname as a positional argument\n")
		os.Exit(1)
	} else {
		address = flag.Arg(0)
	}

	for i := 0; i != *pingCount; i++ {
		logIPAddress, logErr := stats.ping(address)
		if logErr != nil {
			stats.count++
			stats.lost++
			log.Printf("ERROR: %s\n", logErr)
		} else {
			stats.count++
			stats.rttAll = append(stats.rttAll, stats.rtt)
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
	stats.showStatistics()
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
	ttl      int
)

type statistic struct {
	count               int
	lost                int
	rtt                 time.Duration
	loss                float64
	rttAll              []time.Duration
	totalDifferencesRTT time.Duration
	jitter              time.Duration
}

func (stats *statistic) ping(address string) (*net.IPAddr, error) {
	stats.rtt = 0 // Reset rtt in case error causes return before update

	var (
		listenNetwork  string
		listenAddress  string
		resolveNetwork string
		messageType    icmp.Type
		protocolICMP   int
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

	if wantIPv6 {
		listenPacket.IPv6PacketConn().SetHopLimit(ttl)
	} else {
		listenPacket.IPv4PacketConn().SetTTL(ttl)
	}

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
	err = listenPacket.SetReadDeadline(time.Now().Add(10 * time.Second))
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
	// Let IPv6 discovery not count as error (https://www.sharetechnote.com/html/IP_Network_IPv6.html)
	case ipv6.ICMPTypeNeighborSolicitation, ipv6.ICMPTypeNeighborAdvertisement:
		fallthrough
	case ipv4.ICMPTypeEchoReply, ipv6.ICMPTypeEchoReply:
		return ipAddress, nil
	default:
		return ipAddress, fmt.Errorf("Received %s instead of echo reply", reply.Type)

	}
}

func (stats *statistic) closeHandler() {
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func(stats *statistic) {
		<-c
		fmt.Println(": Signal Interrupt received... ")
		stats.showStatistics()
		os.Exit(0)
	}(stats)
}

func (stats *statistic) showStatistics() {
	fmt.Println("\n--------------| Statistics |--------------")
	if len(stats.rttAll) > 1 {
		for val := range stats.rttAll[:len(stats.rttAll)-1] {
			diff := stats.rttAll[val] - stats.rttAll[val+1]
			if diff < 0 {
				diff = -diff
			}
			stats.totalDifferencesRTT += diff
		}
		stats.jitter = time.Duration(int64(stats.totalDifferencesRTT) / int64(len(stats.rttAll)-1))
	}
	fmt.Printf("Packets sent: %d\t\tPackets lost: %d\t\tLoss: %.2f%%\t\tJitter: %s\n", stats.count, stats.lost, stats.loss, stats.jitter)
}
