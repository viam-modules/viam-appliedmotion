package st

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"go.uber.org/multierr"
)

type commPort = *comms

type comms struct {
	mu     sync.RWMutex
	logger golog.Logger
	ctx    context.Context
	uri    string
	handle io.ReadWriteCloser

	// Sometimes the IP-based motor controllers lose their connection, and we need to make it again.
	reconnect func() (io.ReadWriteCloser, error)
}

func newIpComm(ctx context.Context, uri string, timeout time.Duration, logger golog.Logger) (commPort, error) {
	reconnect := func() (io.ReadWriteCloser, error) {
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
		return socket, nil
	}

	if socket, err := reconnect(); err != nil {
		return nil, err
	} else {
		return &comms{handle: socket, URI: uri, logger: logger, reconnect: reconnect}, nil
	}
}

func newSerialComm(ctx context.Context, file string, logger golog.Logger) (commPort, error) {
	reconnect := func() (io.ReadWriteCloser, error) {
		logger.Debugf("Opening %s", file)
		return os.OpenFile(file, os.O_RDWR, fs.FileMode(os.O_RDWR))
	}

	if fd, err := reconnect(); err != nil {
		return nil, err
	} else {
		return &comms{handle: fd, URI: file, logger: logger, reconnect: reconnect}, nil
	}
}

func (s *comms) send(ctx context.Context, command string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logger.Debugf("Sending command: %#v", command)

	// As described on page 336 of
	// https://appliedmotion.s3.amazonaws.com/Host-Command-Reference_920-0002W_0.pdf, all packets
	// sent either from us or to us should start with the two bytes 0x00 0x07, and end with the
	// byte 0x0D (carriage return). The main command we send is sandwiched between them, so the
	// buffer of data we send needs to be 3 bytes longer than the command.
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
		// After a long period of inactivity, our connection might have gotten stale. Try
		// reconnecting and sending the buffer one more time, but give up if that doesn't help.
		err = multierr.Combine(err, s.handle.Close())
		handle, reconnectErr := s.reconnect()
		if reconnectErr != nil {
			return "", multierr.Combine(err, reconnectErr)
		}
		s.handle = handle

		// Update the previous nWritten when we retry writing.
		var secondErr error
		nWritten, secondErr = s.handle.Write(sendBuffer)
		if err != nil {
			return "", multierr.Combine(err, secondErr)
		}
	}
	if nWritten != 3+len(command) {
		return "", errors.New("failed to write all bytes")
	}
	readBuffer := make([]byte, 1024)
	nRead, err := s.handle.Read(readBuffer)
	if err != nil {
		return "", err
	}

	// Like the packet we sent, the one we receive should start with 0x00 0x07 and end with 0x0D.
	// We care about the part in between these.
	if readBuffer[0] != 0x00 || readBuffer[1] != 0x07 || readBuffer[nRead-1] != 0x0D {
		return "", fmt.Errorf("unexpected response from motor controller: %#v", readBuffer)
	}

	retString := string(readBuffer[2:nRead-1])
	s.logger.Debugf("Response: %#v", retString)

	return retString, nil
}

func (s *comms) store(ctx context.Context, command string, value float64) error {
	// Many commands can only handle 3 digits of precision, but some can handle 4 and the
	// controller will round to the nearest value it can handle anyway.
	result, err := s.send(ctx, fmt.Sprintf("%s%.4f", command, value))
	if err != nil {
		return err
	}
	// Executed commands use "%" for their ACK, and buffered commands use "*" for it.
	if result != "%" && result != "*" {
		return fmt.Errorf("got non-ack response when trying to set %s to %f: %s",
		                  command, value, result)
	}
	return nil
}

func (s *comms) Close() error {
	s.logger.Debugf("Closing %s", s.uri)
	return s.handle.Close()
}
