// Package telnet serves the reference TUI over Telnet.
package telnet

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"

	"github.com/castingcode/tuicast/reference"
)

const (
	commandSE   byte = 240
	commandSB   byte = 250
	commandWILL byte = 251
	commandWONT byte = 252
	commandDO   byte = 253
	commandDONT byte = 254
	commandIAC  byte = 255

	optionEcho            byte = 1
	optionSuppressGoAhead byte = 3
	optionTerminalType    byte = 24
	optionWindowSize      byte = 31
	terminalTypeSend      byte = 1
)

// Serve accepts Telnet connections until the context is canceled or the
// listener fails. Each connection gets an isolated reference application.
func Serve(ctx context.Context, listener net.Listener, logger *slog.Logger) error {
	if ctx == nil {
		return fmt.Errorf("serving reference TUI over Telnet: context is required")
	}
	if listener == nil {
		return fmt.Errorf("serving reference TUI over Telnet: listener is required")
	}
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()
	for {
		connection, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return nil
			}
			return fmt.Errorf("accepting Telnet connection: %w", err)
		}
		go serveConnection(connection, logger)
	}
}

func serveConnection(connection net.Conn, logger *slog.Logger) {
	defer connection.Close()
	resizes := make(chan reference.Size, 1)
	stream := &protocolStream{connection: connection, resizes: resizes}
	if err := stream.negotiate(); err != nil {
		logger.Warn("Telnet negotiation failed", "remote", connection.RemoteAddr(), "error", err)
		close(resizes)
		return
	}
	application, err := reference.New(80, 24)
	if err == nil {
		err = application.RunWithResizes(stream, stream, resizes)
	}
	close(resizes)
	if err != nil && !errors.Is(err, io.EOF) {
		logger.Warn("reference Telnet session stopped", "remote", connection.RemoteAddr(), "error", err)
	}
}

type parseState uint8

const (
	stateData parseState = iota
	stateIAC
	stateNegotiation
	stateSubOption
	stateSubData
	stateSubIAC
)

type protocolStream struct {
	connection net.Conn
	resizes    chan reference.Size
	writeMu    sync.Mutex
	state      parseState
	command    byte
	subOption  byte
	subData    []byte
	pending    []byte
	pendingErr error
}

func (s *protocolStream) negotiate() error {
	return s.writeProtocol([]byte{
		commandIAC, commandWILL, optionEcho,
		commandIAC, commandWILL, optionSuppressGoAhead,
		commandIAC, commandDO, optionTerminalType,
		commandIAC, commandDO, optionWindowSize,
	})
}

func (s *protocolStream) Read(data []byte) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}
	buffer := make([]byte, 4096)
	for len(s.pending) == 0 {
		if s.pendingErr != nil {
			err := s.pendingErr
			s.pendingErr = nil
			return 0, err
		}
		n, err := s.connection.Read(buffer)
		if n > 0 {
			if processErr := s.process(buffer[:n]); processErr != nil {
				return 0, fmt.Errorf("processing Telnet input: %w", processErr)
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				s.pendingErr = io.EOF
			} else {
				s.pendingErr = fmt.Errorf("reading Telnet connection: %w", err)
			}
		}
	}
	n := copy(data, s.pending)
	s.pending = s.pending[n:]
	return n, nil
}

func (s *protocolStream) Write(data []byte) (int, error) {
	encoded := make([]byte, 0, len(data))
	for _, value := range data {
		encoded = append(encoded, value)
		if value == commandIAC {
			encoded = append(encoded, commandIAC)
		}
	}
	if err := s.writeProtocol(encoded); err != nil {
		return 0, fmt.Errorf("writing Telnet output: %w", err)
	}
	return len(data), nil
}

func (s *protocolStream) process(data []byte) error {
	for _, value := range data {
		switch s.state {
		case stateData:
			if value == commandIAC {
				s.state = stateIAC
			} else {
				s.pending = append(s.pending, value)
			}
		case stateIAC:
			switch value {
			case commandIAC:
				s.pending = append(s.pending, value)
				s.state = stateData
			case commandWILL, commandWONT, commandDO, commandDONT:
				s.command = value
				s.state = stateNegotiation
			case commandSB:
				s.state = stateSubOption
			default:
				s.state = stateData
			}
		case stateNegotiation:
			s.state = stateData
			if s.command == commandWILL && value == optionTerminalType {
				if err := s.writeProtocol([]byte{commandIAC, commandSB, optionTerminalType, terminalTypeSend, commandIAC, commandSE}); err != nil {
					return err
				}
			}
		case stateSubOption:
			s.subOption = value
			s.subData = s.subData[:0]
			s.state = stateSubData
		case stateSubData:
			if value == commandIAC {
				s.state = stateSubIAC
			} else {
				s.subData = append(s.subData, value)
			}
		case stateSubIAC:
			if value == commandIAC {
				s.subData = append(s.subData, value)
				s.state = stateSubData
			} else {
				s.state = stateData
				if value == commandSE {
					s.applySubnegotiation()
				}
			}
		}
	}
	return nil
}

func (s *protocolStream) applySubnegotiation() {
	if s.subOption != optionWindowSize || len(s.subData) != 4 {
		return
	}
	width := int(s.subData[0])<<8 | int(s.subData[1])
	height := int(s.subData[2])<<8 | int(s.subData[3])
	if width == 0 || height == 0 {
		return
	}
	size := reference.Size{Width: width, Height: height}
	select {
	case s.resizes <- size:
	default:
		<-s.resizes
		s.resizes <- size
	}
}

func (s *protocolStream) writeProtocol(data []byte) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	for len(data) > 0 {
		n, err := s.connection.Write(data)
		data = data[n:]
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
	}
	return nil
}
