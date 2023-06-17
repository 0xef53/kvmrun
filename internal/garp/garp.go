package garp

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"syscall"
	"time"
)

const (
	// ARP protocol opcodes.
	ARPOP_REQUEST uint16 = 1 // ARP request
	ARPOP_REPLY   uint16 = 2 // ARP reply

	// Ethernet Protocol identifiers.
	ETH_TYPE_IPV4 uint16 = 0x0800 // Internet Protocol v4
)

var (
	ethernetBroadcast = net.HardwareAddr{0xff, 0xff, 0xff, 0xff, 0xff, 0xff}
)

// arpHeader specifies the header for an ARP message.
type arpHeader struct {
	// HardwareType specifies an IANA-assigned hardware type,
	// as described in RFC 826.
	HardwareType uint16

	// ProtocolType specifies the internet protocol for which
	// the ARP request is intended. Typically, this is the IPv4 EtherType.
	ProtocolType uint16

	// HardwareAddrLength specifies the length of the sender and target
	// hardware addresses included in a Packet.
	HardwareAddrLength uint8

	// ProtocolAddrLength specifies the length of the sender and target
	// IPv4 addresses included in a Packet.
	ProtocolAddrLength uint8

	// Operation specifies the ARP operation being performed, such as request
	// or reply.
	Operation uint16
}

// arpMessage represents an ARP message.
type arpMessage struct {
	arpHeader

	// SenderHardwareAddr specifies the hardware address of the sender of this Packet.
	SenderHardwareAddr []byte

	// SenderProtocolAddr specifies the IPv4 address of the sender of this Packet.
	SenderProtocolAddr []byte

	// TargetHardwareAddr specifies the hardware address of the target of this Packet.
	TargetHardwareAddr []byte

	// TargetProtocolAddr specifies the IPv4 address of the target of this Packet.
	TargetProtocolAddr []byte
}

// newMessage returns a new ARP message that contains a Gratuitous ARP
// reply from the specified sender.
//
// Only Ethernet MAC-address and IPv4 address are supported.
func newMessage(ip net.IP, hwaddr net.HardwareAddr) (*arpMessage, error) {
	if ip.To4() == nil {
		return nil, fmt.Errorf("not an IPv4 address: %q", ip)
	}
	if len(hwaddr) != 6 {
		return nil, fmt.Errorf("not an Ethernet MAC-address: %q", hwaddr)
	}

	msg := &arpMessage{
		arpHeader{
			syscall.ARPHRD_ETHER,
			ETH_TYPE_IPV4,
			uint8(len(hwaddr)),
			uint8(net.IPv4len),
			ARPOP_REPLY,
		},
		hwaddr,
		ip.To4(),
		ethernetBroadcast,
		net.IPv4bcast,
	}

	return msg, nil
}

// Bytes returns the wire representation of the ARP message.
func (m *arpMessage) Bytes() ([]byte, error) {
	// 2 bytes: hardware type
	// 2 bytes: protocol type
	// 1 byte : hardware address length
	// 1 byte : protocol length
	// 2 bytes: operation
	// N bytes: source hardware address
	// N bytes: source protocol address
	// N bytes: target hardware address
	// N bytes: target protocol address

	buf := new(bytes.Buffer)

	if err := binary.Write(buf, binary.BigEndian, m.arpHeader); err != nil {
		return nil, fmt.Errorf("binary write failed: %v", err)
	}
	buf.Write(m.SenderHardwareAddr)
	buf.Write(m.SenderProtocolAddr)
	buf.Write(m.TargetHardwareAddr)
	buf.Write(m.TargetProtocolAddr)

	return buf.Bytes(), nil
}

// Send sends a gratuitous ARP message through the specified interface
// COUNT times (or until the context is closed) with a given INTERVAL.
func Send(ctx context.Context, ifname, ipstr string, count, interval uint) error {
	iface, err := net.InterfaceByName(ifname)
	if err != nil {
		return err
	}

	ip := net.ParseIP(ipstr)
	if ip == nil {
		return fmt.Errorf("invalid IP address: %s", ipstr)
	}

	msg, err := newMessage(ip, iface.HardwareAddr)
	if err != nil {
		return err
	}

	return send(ctx, iface, msg, count, interval)
}

func send(ctx context.Context, iface *net.Interface, msg *arpMessage, count, interval uint) error {
	fd, err := syscall.Socket(syscall.AF_PACKET, syscall.SOCK_DGRAM, int(htons(syscall.ETH_P_ARP)))
	if err != nil {
		return fmt.Errorf("failed to create raw socket: %s", err)
	}
	defer syscall.Close(fd)

	if err := syscall.BindToDevice(fd, iface.Name); err != nil {
		return fmt.Errorf("failed to bind to device: %s", err)
	}

	ll := syscall.SockaddrLinklayer{
		Protocol: htons(syscall.ETH_P_ARP),
		Ifindex:  iface.Index,
		Pkttype:  syscall.PACKET_HOST,
		Hatype:   msg.HardwareType,
		Halen:    msg.HardwareAddrLength,
	}

	target := msg.TargetHardwareAddr

	for i := 0; i < len(target); i++ {
		ll.Addr[i] = target[i]
	}

	b, err := msg.Bytes()
	if err != nil {
		return fmt.Errorf("failed to convert ARP message: %s", err)
	}

	if err := syscall.Bind(fd, &ll); err != nil {
		return fmt.Errorf("failed to bind: %s", err)
	}

	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	for c := 0; c < int(count); c++ {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}

		if err := syscall.Sendto(fd, b, 0, &ll); err != nil {
			return fmt.Errorf("failed to send: %v", err)
		}
	}

	return nil
}

// htons (HostToNetShort) converts a 16-bit integer from host to network byte order.
func htons(i uint16) uint16 {
	b := make([]byte, 2)
	binary.LittleEndian.PutUint16(b, i)

	return binary.BigEndian.Uint16(b)
}
