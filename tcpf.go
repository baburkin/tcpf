package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
)

const (
	// Default socket read buffer size
	readBufSize = 1024
)

var (
	// Generator of IDs for tunnels
	id = 0
)

// TunnelRealm describes the common properties of TCP tunnels, such as:
// * local bind interface and port
// * destination IP/hostname and port
// * map of currently registered TCPTunnel's in the system
type TunnelRealm struct {
	bindIF   string
	bindPort string
	dstHost  string
	dstPort  string
	tunnels  map[string]*TCPTunnel
	joining  chan net.Conn
}

// NewTunnelRealm creates a new TunnelRealm with given bind IP:port and destination IP:port
func NewTunnelRealm(bindIF string, bindPort string, dstHost string, dstPort string) *TunnelRealm {
	realm := &TunnelRealm{
		tunnels:  make(map[string]*TCPTunnel),
		joining:  make(chan net.Conn),
		bindIF:   bindIF,
		bindPort: bindPort,
		dstHost:  dstHost,
		dstPort:  dstPort,
	}
	realm.listen()
	return realm
}

func (realm *TunnelRealm) listen() {
	go func() {
		for {
			select {
			case conn := <-realm.joining:
				realm.join(conn)
			}
		}
	}()
}

func (realm *TunnelRealm) listTunnels() {
	log.Printf("The realm has the following tunnels: [%v]", realm.tunnels)
}

func (realm *TunnelRealm) join(conn net.Conn) {
	tunnel := newTCPTunnel(conn, realm)
	id := tunnel.id
	realm.tunnels[id] = tunnel
	log.Printf("Added tunnel: %v:[%v]", id, tunnel)
	realm.listTunnels()
}

func (realm *TunnelRealm) leave(tunnel *TCPTunnel) {
	log.Printf("Tunnel leaving realm and being closed: [%v]", tunnel)
	delete(realm.tunnels, tunnel.id)
	(*tunnel.inbound).Close()
	(*tunnel.outbound).Close()
	realm.listTunnels()
}

// TCPTunnel contains connection properties of a TCP tunnel:
// * inbound and outbound socket connections
// * channels for exchanging streams of bytes between inbound/outbound sockets
// * pointer to the realm (TunnelRealm)
type TCPTunnel struct {
	id       string
	local    chan []byte
	remote   chan []byte
	inbound  *net.Conn
	outbound *net.Conn
	realm    *TunnelRealm
}

func generateID() string {
	id++
	return strconv.Itoa(id)
}

func newTCPTunnel(conn net.Conn, realm *TunnelRealm) *TCPTunnel {
	address := realm.dstHost + ":" + realm.dstPort
	outbound, err := net.Dial("tcp", address)
	if err != nil {
		log.Printf("Destination address is not available: %v. Error: %v", address, err)
		os.Exit(1)
	}
	tunnel := &TCPTunnel{
		id:       generateID(),
		local:    make(chan []byte),
		remote:   make(chan []byte),
		inbound:  &conn,
		outbound: &outbound,
		realm:    realm,
	}
	tunnel.listen()
	return tunnel
}

func (tunnel *TCPTunnel) String() string {
	local := (*tunnel.inbound).RemoteAddr()
	remote := (*tunnel.outbound).RemoteAddr()
	return fmt.Sprintf("%v -> %v", local, remote)
}

func (tunnel *TCPTunnel) listen() {
	go tunnel.readFromInbound()
	go tunnel.writeToOutbound()
	go tunnel.readFromOutbound()
	go tunnel.writeToInbound()
}

func readFromConn(conn *net.Conn, channel *chan []byte) error {
	for {
		bytes := make([]byte, readBufSize)
		n, err := (*conn).Read(bytes)
		if err != nil {
			return err
		}
		*channel <- bytes[:n]
	}
}

func writeToConn(conn *net.Conn, channel *chan []byte) error {
	for {
		bytes := <-*channel
		_, err := (*conn).Write(bytes)
		if err != nil {
			return err
		}
	}
}

func (tunnel *TCPTunnel) closeTunnel() {
	log.Printf("Connection closed for %v", tunnel)
	tunnel.realm.leave(tunnel)
}

func (tunnel *TCPTunnel) readFromInbound() {
	err := readFromConn(tunnel.inbound, &tunnel.remote)
	if err != io.EOF {
		log.Printf("Error occured: %v", err)
	}
	tunnel.closeTunnel()
}

func (tunnel *TCPTunnel) readFromOutbound() {
	err := readFromConn(tunnel.outbound, &tunnel.local)
	if err != io.EOF {
		log.Printf("Error occured: %v", err)
	}
	tunnel.closeTunnel()
}

func (tunnel *TCPTunnel) writeToInbound() {
	err := writeToConn(tunnel.inbound, &tunnel.local)
	if err != io.EOF {
		log.Printf("Error occured: %v", err)
	}
	tunnel.closeTunnel()
}

func (tunnel *TCPTunnel) writeToOutbound() {
	err := writeToConn(tunnel.outbound, &tunnel.remote)
	if err != io.EOF {
		log.Printf("Error occured: %v", err)
	}
	tunnel.closeTunnel()
}

func main() {
	flag.Parse()
	bindPort := flag.Arg(0)
	remoteHost := flag.Arg(1)
	remotePort := flag.Arg(2)
	log.Printf("Starting TCPF on port %v => %v:%v...", bindPort, remoteHost, remotePort)
	realm := NewTunnelRealm("", bindPort, remoteHost, remotePort)
	serverSock, _ := net.Listen("tcp", "127.0.0.1:"+bindPort)
	for {
		conn, err := serverSock.Accept()
		if err != nil {
			log.Printf("Error occured: %v", err)
		}
		realm.joining <- conn
	}
}
