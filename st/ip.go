package st

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/edaniels/golog"
)

type CommPort interface {
	GetUri() string
	Send(ctx context.Context, command string) (string, error)
	Close() error
}

type ST_IP struct {
	logger golog.Logger
	Ctx    context.Context
	URI    string
	socket net.Conn
}

func newIpComm(ctx context.Context, uri string, timeout time.Duration, logger golog.Logger) (*ST_IP, error) {
	d := net.Dialer{
		Timeout: timeout,
	}
	socket, err := d.DialContext(ctx, "tcp", uri)
	return &ST_IP{socket: socket, URI: uri, logger: logger}, err
}

func (s *ST_IP) Send(ctx context.Context, command string) (string, error) {
	// it is 3 + len(command) because we need the 07 to start and we need to append a carriage return (\r)
	sendBuffer := make([]byte, 3+len(command))
	sendBuffer[0] = 0
	sendBuffer[1] = 7
	for i, v := range command {
		sendBuffer[i+2] = byte(v)
	}
	sendBuffer[len(sendBuffer)-1] = '\r'
	s.logger.Debugf("Sending command: %#v", string(sendBuffer))
	nWritten, err := s.socket.Write(sendBuffer)
	if err != nil {
		return "", err
	}
	if nWritten != 3+len(command) {
		return "", errors.New("failed to write all bytes")
	}
	readBuffer := make([]byte, 1024)
	nRead, err := s.socket.Read(readBuffer)
	if err != nil {
		return "", err
	}
	retString := string(readBuffer[:nRead])
	s.logger.Debugf("Response: %#v", retString)
	return retString, nil
}

func (s *ST_IP) Close() error {
	return s.socket.Close()
}

func (s *ST_IP) GetUri() string {
	return s.URI
}

// TODO: Need to implement this, I think it's going to depend on how the RS485 interface is exposed to the OS/software
type ST_RS485 struct {
}

// TODO: Need to implement this, I think it's going to depend on how the RS485 interface is exposed to the OS/software
type ST_RS232 struct {
}
