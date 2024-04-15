package memcached

import (
	"bufio"
	"fmt"
	"net"
	"regexp"
	"strings"
)

type Transport interface {
	connect() error
	Close()
	Write(string) error
	Read([]byte) (string, error)
}

type Cache interface {
	Set(key key, value string, ttl ttl) error
	Get(key key) (string, error)
	Delete(key key) error
}

type TransportSocket struct {
	network string
	address string
	conn    net.Conn
	reader  *bufio.Reader
}

func (t *TransportSocket) connect() error {
	if t.conn != nil {
		return nil
	}
	var dialErr error
	t.conn, dialErr = net.Dial(t.network, t.address)
	if dialErr != nil {
		return fmt.Errorf("cannot connect: %q\n", dialErr)
	}
	t.reader = bufio.NewReader(t.conn)
	return nil
}

func (t *TransportSocket) Close() {
	if t.conn == nil {
		return
	}
	err := t.conn.Close()
	if err != nil {
		fmt.Println("cannot close connection: ", err)
	}
}

func (t *TransportSocket) Write(data string) error {
	_, err := t.conn.Write([]byte(data))
	return err
}

func (t *TransportSocket) Read(bytes []byte) (string, error) {
	var line string
	var err error
	if len(bytes) == 0 {
		line, err = t.reader.ReadString('\n')
		if err != nil {
			return "", err
		}
		return line, nil
	}
	_, err = t.reader.Read(bytes)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func NewTransportSocket(network string, address string) *TransportSocket {
	return &TransportSocket{network: network, address: address}
}

type ttl int32

func (t ttl) isValid() error {
	return nil
}

type key string

func (k *key) isValid() error {
	if len(*k) == 0 {
		return fmt.Errorf("empty key\n")
	}
	if len(*k) > 250 {
		return fmt.Errorf("key too long\n")
	}
	var re = regexp.MustCompile(`[\x00-\x1F\x7F\s]`)
	if re.MatchString(string(*k)) {
		return fmt.Errorf("invalid key\n")
	}
	return nil
}

type Memcached struct {
	transport Transport
}

func NewMemcached(network string, address string) (*Memcached, error) {
	return &Memcached{transport: NewTransportSocket(network, address)}, nil
}

func (m *Memcached) Set(key key, value string, ttl ttl) error {
	validKeyErr := key.isValid()
	if validKeyErr != nil {
		return validKeyErr
	}
	validTtlErr := ttl.isValid()
	if validTtlErr != nil {
		return validTtlErr
	}

	cmd := fmt.Sprintf("set %s 0 %d %d\r\n%s", key, ttl, len(value), value)
	resp, err := m.command(cmd)
	if err != nil {
		return err
	}
	if resp != "STORED\r\n" {
		return fmt.Errorf("value is not stored: %q\n", resp)
	}
	return err
}

func (m *Memcached) Get(key key) (string, error) {
	validKeyErr := key.isValid()
	if validKeyErr != nil {
		return "", validKeyErr
	}

	eof := "END\r\n"

	cmd := fmt.Sprintf("get %s", key)
	header, err := m.command(cmd)
	if err != nil {
		return "", err
	}
	if header == eof {
		return "", nil
	}

	flags, bytes := 0, 0
	n, err := fmt.Sscanf(header, "VALUE %s %d %d\r\n", &key, &flags, &bytes)
	if n != 3 {
		return "", fmt.Errorf("cannot parse header: %q\n", header)
	}
	dataBuf := make([]byte, bytes)
	_, err = m.transport.Read(dataBuf)
	if err != nil {
		return "", err
	}

	rnEof := "\r\n" + eof
	rnEofLen := len(rnEof)
	rnEofBuf := make([]byte, rnEofLen)
	_, err = m.transport.Read(rnEofBuf)
	if err != nil {
		return "", err
	}
	line := string(rnEofBuf)
	if line != rnEof {
		return "", fmt.Errorf("unexpected end: %q\n", line)
	}

	return string(dataBuf), nil
}

func (m *Memcached) Delete(key key) error {
	validKeyErr := key.isValid()
	if validKeyErr != nil {
		return validKeyErr
	}

	cmd := fmt.Sprintf("delete %s", key)
	resp, err := m.command(cmd)
	if err != nil {
		return err
	}
	if resp != "DELETED\r\n" {
		return fmt.Errorf("delete failed: %q\n", resp)
	}
	return nil
}

func (m *Memcached) Close() {
	m.transport.Close()
}

func (m *Memcached) command(cmd string) (string, error) {
	connectErr := m.transport.connect()
	if connectErr != nil {
		return "", connectErr
	}

	writeErr := m.transport.Write(cmd + "\r\n")
	if writeErr != nil {
		m.Close()
		return "", fmt.Errorf("write error: %q\n", writeErr)
	}

	line, readErr := m.transport.Read([]byte{})
	if readErr != nil {
		m.Close()
		return "", fmt.Errorf("read error: %q\n", readErr)
	}
	if line == "ERROR\r\n" {
		return "", fmt.Errorf("nonexistent command: %q\n", cmd)
	}
	if strings.HasPrefix(line, "CLIENT_ERROR ") || strings.HasPrefix(line, "SERVER_ERROR ") {
		return "", fmt.Errorf("error: %q\n", line)
	}
	return line, nil
}
