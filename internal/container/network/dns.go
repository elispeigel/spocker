package network

import (
	"bufio"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"
)

// GetDefaultDNS returns the default DNS IP address.
func GetDefaultDNS() (net.IP, error) {
	// Open the resolv.conf file
	file, err := os.Open("/etc/resolv.conf")
	if err != nil {
		log.Printf("Error opening resolv.conf: %v", err)
		return nil, err
	}
	defer file.Close()

	// Read the file line by line
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)

		// Look for the nameserver directive
		if len(fields) >= 2 && fields[0] == "nameserver" {
			ip := net.ParseIP(fields[1])
			if ip != nil {
				return ip, nil
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Error reading resolv.conf: %v", err)
		return nil, err
	}

	return nil, nil
}

func configureDNS(containerID, dns string) error {
	udpAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", dns, 53))
	if err != nil {
		return fmt.Errorf("failed to resolve DNS address: %w", err)
	}

	udpConn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		return fmt.Errorf("failed to create UDP connection to DNS server: %w", err)
	}
	defer udpConn.Close()

	// For example, querying "example.com" with a type A (IPv4) record
	query, err := createDNSQuery("example.com", 1)
	if err != nil {
		return fmt.Errorf("failed to create DNS query: %w", err)
	}
	if _, err := udpConn.Write(query); err != nil {
		return fmt.Errorf("failed to send DNS query: %w", err)
	}

	// Set a read timeout for the response
	udpConn.SetReadDeadline(time.Now().Add(5 * time.Second))

	// Read the DNS response
	response := make([]byte, 512)
	n, _, err := udpConn.ReadFrom(response)
	if err != nil {
		return fmt.Errorf("failed to read DNS response: %w", err)
	}

	// Parse the DNS response
	answers, err := parseDNSResponse(response[:n])
	if err != nil {
		return fmt.Errorf("failed to parse DNS response: %w", err)
	}

	// Process the DNS response
	for _, answer := range answers {
		if answer.Type == 1 {
			fmt.Printf("IPv4 address for %s: %s\n", answer.Name, answer.Data)
		}
	}

	return nil
}

func parseDNSResponse(response []byte) ([]Answer, error) {
	header, err := parseHeader(response)
	if err != nil {
		return nil, err
	}

	if header.qr == 0 {
		return nil, errors.New("not a response")
	}

	if header.rcode != 0 {
		return nil, fmt.Errorf("error in RCODE %d", header.rcode)
	}

	offset := 12
	// Skip over the question section
	for i := 0; i < int(header.qdcount); i++ {
		_, end, err := readDomainName(response, offset)
		if err != nil {
			return nil, fmt.Errorf("failed to read domain name: %w", err)
		}
		offset = end + 4 // 4 bytes for QTYPE and QCLASS
	}

	answers := make([]Answer, header.ancount)
	for i := 0; i < int(header.ancount); i++ {
		var err error
		answers[i], offset, err = readAnswer(response, offset)
		if err != nil {
			return nil, fmt.Errorf("failed to read answer: %w", err)
		}
	}

	return answers, nil
}

func readDomainName(data []byte, offset int) (string, int, error) {
	var name []string
	for {
		length := data[offset]
		if length == 0 {
			break
		}
		if length&0xC0 == 0xC0 {
			compressedOffset := int(binary.BigEndian.Uint16(data[offset:offset+2])) & 0x3FFF
			compressedName, _, err := readDomainName(data, compressedOffset)
			if err != nil {
				return "", offset, err
			}
			name = append(name, compressedName)
			offset += 2
			break
		}

		offset++
		name = append(name, string(data[offset:offset+int(length)]))
		offset += int(length)
	}

	return strings.Join(name, "."), offset + 1, nil
}

func readAnswer(data []byte, offset int) (Answer, int, error) {
	name, end, err := readDomainName(data, offset)
	if err != nil {
		return Answer{}, offset, err
	}
	rtype := binary.BigEndian.Uint16(data[end : end+2])
	rdlength := binary.BigEndian.Uint16(data[end+8 : end+10])
	rdata := data[end+10 : end+10+int(rdlength)]

	var addr string
	switch rtype {
	case 1: // A
		addr = net.IP(rdata).String()
	case 28: // AAAA
		addr = net.IP(rdata).String()
	default:
		return Answer{}, end + 10 + int(rdlength), fmt.Errorf("unsupported record type: %d", rtype)
	}

	return Answer{
		Name: name,
		Type: rtype,
		TTL:  binary.BigEndian.Uint32(data[end+4 : end+8]),
		Data: addr,
	}, end + 10 + int(rdlength), nil
}

func createDNSQuery(domain string, qtype uint16) ([]byte, error) {
	var idBytes [2]byte
	if _, err := rand.Read(idBytes[:]); err != nil {
		return nil, fmt.Errorf("failed to generate random ID: %w", err)
	}

	id := binary.BigEndian.Uint16(idBytes[:])

	header := make([]byte, 12)
	binary.BigEndian.PutUint16(header[0:], id)
	const recursionDesiredFlag = 1 // Recursion desired
	header[2] = header[2] | recursionDesiredFlag
	binary.BigEndian.PutUint16(header[4:], 1) // One question

	question := make([]byte, 0, 32)
	labels := strings.Split(domain, ".")
	for _, label := range labels {
		question = append(question, byte(len(label)))
		question = append(question, []byte(label)...)
	}
	question = append(question, 0) // Zero-length label (root)

	binary.BigEndian.PutUint16(question, uint16(len(question)-2))
	binary.BigEndian.PutUint16(question, qtype)
	binary.BigEndian.PutUint16(question, 1) // Class IN

	return append(header, question...), nil
}

func parseHeader(response []byte) (dnsHeader, error) {
	if len(response) < 12 {
		return dnsHeader{}, errors.New("response too short")
	}

	header := response[:12]
	return dnsHeader{
		id:      binary.BigEndian.Uint16(header[0:2]),
		qr:      header[2] >> 7,
		opcode:  (header[2] >> 3) & 0xF,
		aa:      (header[2] >> 2) & 0x1,
		tc:      (header[2] >> 1) & 0x1,
		rd:      header[2] & 0x1,
		ra:      header[3] >> 7,
		z:       (header[3] >> 4) & 0x7,
		rcode:   header[3] & 0xF,
		qdcount: binary.BigEndian.Uint16(header[4:6]),
		ancount: binary.BigEndian.Uint16(header[6:8]),
		nscount: binary.BigEndian.Uint16(header[8:10]),
		arcount: binary.BigEndian.Uint16(header[10:12]),
	}, nil
}