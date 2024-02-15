package st

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"net"
	"os"
	"sync"
	"time"

	"github.com/edaniels/golog"
)

type CommPort interface {
	GetUri() string
	Send(ctx context.Context, command string) (string, error)
	Close() error
}

type comms struct {
	mu     sync.RWMutex
	logger golog.Logger
	Ctx    context.Context
	URI    string
	handle io.ReadWriteCloser
}

func newIpComm(ctx context.Context, uri string, timeout time.Duration, logger golog.Logger) (*comms, error) {
	logger.Debugf("Dialing %s", uri)
	d := net.Dialer{
		Timeout:   timeout,
		KeepAlive: 1 * time.Second,
		Deadline:  time.Now().Add(timeout),
	}
	socket, err := d.DialContext(ctx, "tcp", uri)
	if err != nil {
		return nil, err
	}
	return &comms{handle: socket, URI: uri, logger: logger, mu: sync.RWMutex{}}, nil
}

func newSerialComm(ctx context.Context, file string, logger golog.Logger) (*comms, error) {
	logger.Debugf("Opening %s", file)
	if fd, err := os.OpenFile(file, os.O_RDWR, fs.FileMode(os.O_RDWR)); err != nil {
		return nil, err
	} else {
		return &comms{handle: fd, URI: file, logger: logger, mu: sync.RWMutex{}}, nil
	}
}

func (s *comms) Send(ctx context.Context, command string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logger.Debugf("Sending command: %#v", command)
	// it is 3 + len(command) because we need the 07 to start and we need to append a carriage return (\r)
	sendBuffer := make([]byte, 3+len(command))
	sendBuffer[0] = 0
	sendBuffer[1] = 7
	for i, v := range command {
		sendBuffer[i+2] = byte(v)
	}
	sendBuffer[len(sendBuffer)-1] = '\r'
	s.logger.Debugf("Sending buffer: %#v", sendBuffer)
	nWritten, err := s.handle.Write(sendBuffer)
	if err != nil {
		return "", err
	}
	if nWritten != 3+len(command) {
		return "", errors.New("failed to write all bytes")
	}
	readBuffer := make([]byte, 1024)
	nRead, err := s.handle.Read(readBuffer)
	if err != nil {
		return "", err
	}
	// TODO: Check the return value to see if it resulted in an error (and wrap it) or was a success
	retString := string(readBuffer[:nRead])
	s.logger.Debugf("Response: %#v", retString)
	time.Sleep(1 * time.Millisecond)
	return retString, nil
}

func (s *comms) Close() error {
	s.logger.Debugf("Closing %s", s.URI)
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.handle.Close()
}

func (s *comms) GetUri() string {
	return s.URI
}
