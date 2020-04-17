// PROGRAM: goPing
// AUTHOR: Alvi Habib
// DESCRIPTION: A simple CLI ping application to send echo requests and receive
// echo replies in an infinite loop, unless specified.
//
// ATTRIBUTES:
// 1) Written in the Go programming language
// 2) CLI interface with positional argument for hostname/IP address
// 3) Sends ICMP "echo requests" in an infinite loop
// 4) Reports loss and RTT times for each message
// 5) Handles both IPv4 and IPv6 (with flag)
// 6) Supports setting TTL with time exceeded messages (flag)
// 7) Supports finite number of pings (with flag)
// 8) Supports calculating jitter
// 9) Supports displaying summary of statistics upon termination

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

const (
	listenNetwork4  string = "ip4:icmp"      // Listen network for IPv4
	listenNetwork6  string = "ip6:ipv6-icmp" // Listen network for IPv6
	listenAddress4  string = "0.0.0.0"       // Default listen address for IPv4
	listenAddress6  string = "::"            // Default listen address for IPv6
	resolveNetwork4 string = "ip4"           // Resolve network for IPv4
	resolveNetwork6 string = "ip6"           // Resolve network for IPv6
	protocolICMP4   int    = 1               // ICMP protocol for IPv4 for ParseMessage
	protocolICMP6   int    = 58              // ICMP protocol for IPv6 for ParseMessage
)

var (
	wantIPv6 bool // Is IPv6 desired?
	ttl      int  // Time-To-Live (-ttl) flag
)

type statistic struct {
	count               int             // Number of packets sent
	lost                int             // Number of packets lost
	rtt                 time.Duration   // Round trip time for each packet
	loss                float64         // Percent loss at iteration
	rttAll              []time.Duration // All RTTs in a slice for jitter calculation
	totalDifferencesRTT time.Duration   // Differences between subsequent RTTs for jitter calculation
	jitter              time.Duration   // Jitter
}

func main() {
	// Remove timestamp from log
	log.SetFlags(log.Flags() &^ (log.Ldate | log.Ltime))

	// Create statistics client
	stats := new(statistic)

	// Listen for ctrl-c termination
	stats.closeHandler()

	// Parse flags to variables
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

	// Error check pingCount (-c) input
	if *pingCount < -1 {
		log.Printf("Times to ping must be positive int, or -1 for infinite. Defaulting to infinite...")
		*pingCount = -1
	}

	// Error check timeToLive (-ttl) input
	if *timeToLive < 0 {
		log.Printf("Invalid TTL. Defaulting to 64...")
		*timeToLive = 64
	}
	ttl = *timeToLive

	// Establish IP Version
	if wantIPv6 = *ipVersion == 6; wantIPv6 {
		log.Printf("Using IPv6...\n")
	} else {
		log.Printf("Using IPv4...\n")
	}

	// Establish hostname/IP address
	var address string // Store hostname or IP address
	if flag.NArg() == 0 {
		log.Printf("No IP/hostname specified. Defaulting to cloudflare.com...\n")
		address = "cloudflare.com"
	} else if flag.NArg() > 1 {
		log.Printf("Please enter only one IP/hostname as a positional argument\n")
		os.Exit(1)
	} else {
		address = flag.Arg(0)
	}

	// Main ping loop
	// Can be infinite or finite
	for i := 0; i != *pingCount; i++ {
		logIPAddress, logErr := stats.ping(address)
		// Update statistics
		if logErr != nil {
			stats.count++
			stats.lost++
			log.Printf("ERROR: %s\n", logErr)
		} else {
			stats.count++
			stats.rttAll = append(stats.rttAll, stats.rtt)
		}
		stats.loss = (float64(stats.lost) / float64(stats.count)) * 100.0
		// Pring statistics every message
		log.Printf(
			"Seq: %d\t\tPinging: %s\t\tRTT: %s\t\tLoss: %.2f%%\n",
			stats.count,
			logIPAddress,
			stats.rtt,
			stats.loss)
		time.Sleep(time.Second) // Sleep for 1 second
	}
	// Show summary if finite pings reached
	stats.showStatistics()
}

// Ping the address, receiving a pointer to the statistics client
func (stats *statistic) ping(address string) (*net.IPAddr, error) {
	stats.rtt = 0 // Reset rtt in case error causes return before update

	var (
		listenNetwork  string    // listenNetwork for ListenPacket
		listenAddress  string    // listenAddress for ListenPacket
		resolveNetwork string    // resolveNetwork for ResolveIPAddr
		messageType    icmp.Type // messageType for icmp.Message
		protocolICMP   int       // protocolICMP for ParseMessage
	)

	// Set parameters according to IPv4 or IPv6
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

	// Listen for reply packets
	listenPacket, err := icmp.ListenPacket(listenNetwork, listenAddress)
	if err != nil {
		return nil, err
	}
	defer listenPacket.Close()

	// Set TTL deadlines
	if wantIPv6 {
		listenPacket.IPv6PacketConn().SetHopLimit(ttl)
	} else {
		listenPacket.IPv4PacketConn().SetTTL(ttl)
	}

	// Resolve hostname to IP address
	ipAddress, err := net.ResolveIPAddr(resolveNetwork, address)
	if err != nil {
		return nil, err
	}

	// Create ICMP echo request packet
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

	// Send packet
	timeSent := time.Now()
	if _, err := listenPacket.WriteTo(requestEncoded, ipAddress); err != nil {
		return ipAddress, err
	}
	replyEncoded := make([]byte, 1000)
	// Set timeout to read reply
	err = listenPacket.SetReadDeadline(time.Now().Add(10 * time.Second))
	if err != nil {
		return ipAddress, err
	}

	// Read echo reply
	replyRead, _, err := listenPacket.ReadFrom(replyEncoded)
	stats.rtt = time.Since(timeSent).Round(10 * time.Microsecond)

	// Parse echo reply
	reply, err := icmp.ParseMessage(protocolICMP, replyEncoded[:replyRead])
	if err != nil {
		return ipAddress, err
	}
	// Determine return based on reply type
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

// Listen for ctrl-c type signal interrupt and exit after displaying summary
func (stats *statistic) closeHandler() {
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func(stats *statistic) {
		<-c
		fmt.Println(": Signal Interrupt received... ")
		// Print statistics now
		stats.showStatistics()
		os.Exit(0)
	}(stats)
}

// Print statistics at program termination
func (stats *statistic) showStatistics() {
	fmt.Println("\n----------------------------| Statistics |----------------------------")
	// Calculate jitter only 2 or more pings stored
	if len(stats.rttAll) > 1 {
		// Formula derived from https://www.pingman.com/kb/article/what-is-jitter-57.html
		for val := range stats.rttAll[:len(stats.rttAll)-1] {
			diff := stats.rttAll[val] - stats.rttAll[val+1]
			if diff < 0 {
				diff = -diff
			}
			stats.totalDifferencesRTT += diff
		}
		// https://stackoverflow.com/questions/54777109/dividing-a-time-duration-in-golang
		stats.jitter = time.Duration(int64(stats.totalDifferencesRTT) / int64(len(stats.rttAll)-1))
	}
	fmt.Printf(
		"Packets sent: %d\t\tPackets lost: %d\t\tLoss: %.2f%%\t\tJitter: %s\n",
		stats.count,
		stats.lost,
		stats.loss,
		stats.jitter)
}
