# goPing -- A Simple CLI Ping Application
#### Author: Alvi Habib

## Features:

- Written in the Go programming language

- CLI interface with positional argument for hostname/IP address

- Sends ICMP "echo requests" in an infinite loop

- Reports loss and RTT times for each message

- Handles both IPv4 and IPv6 (with flag)

- Supports setting TTL with time exceeded messages (flag)

- Supports finite number of pings (with flag)

- Supports calculating jitter

- Supports displaying summary of statistics upon termination

## Usage:
#### To run the application:

Navigate to the correct folder:

    cd goPing
Build the project:

    go build
Run the executable as superuser:

    sudo ./goPing [-c int] [-ipv int] [-ttl int] address
where: 
`-c` is finite number of times to ping, -1 being infinite (default -1)
`-ipv` is 4 or 6, corresponding to which IP version to use (default 4)
`-ttl` is time-to-live before package expires (default 64)

#### Example:
`sudo ./goPing -c 3 -ttl 64 -ipv 6 www.cloudflare.com`

Output:

    Using IPv6...
    Seq: 1          Pinging: 2606:4700::6811:d109           RTT: 340µs              Loss: 0.00%
    Seq: 2          Pinging: 2606:4700::6811:d209           RTT: 21.09ms            Loss: 0.00%
    Seq: 3          Pinging: 2606:4700::6811:d209           RTT: 228.42ms           Loss: 0.00%
    
    ----------------------------| Statistics |----------------------------
    Packets sent: 3         Packets lost: 0         Loss: 0.00%             Jitter: 114.04ms
