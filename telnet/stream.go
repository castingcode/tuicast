package telnet

import (
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
)

const (
	commandSE   byte = 240
	commandSB   byte = 250
	commandWILL byte = 251
	commandWONT byte = 252
	commandDO   byte = 253
	commandDONT byte = 254
	commandIAC  byte = 255

	optionBinary          byte = 0
	optionEcho            byte = 1
	optionSuppressGoAhead byte = 3
	optionTerminalType    byte = 24
	optionNegotiateWindow byte = 31
	terminalTypeIs        byte = 0
	terminalTypeSend      byte = 1
)

type parseState uint8

const (
	stateData parseState = iota
	stateIAC
	stateNegotiation
	stateSubOption
	stateSubData
	stateSubIAC
)

type stream struct {
	conn net.Conn

	readMu  sync.Mutex
	writeMu sync.Mutex
	stateMu sync.Mutex

	state       parseState
	negotiation byte
	subOption   byte
	subData     []byte
	pending     []byte
	pendingErr  error
	terminal    string
	width       int
	height      int
	local       map[byte]bool
	remote      map[byte]bool
	closeOnce   sync.Once
	closeErr    error
}

func newStream(conn net.Conn, terminal string, width, height int) *stream {
	return &stream{
		conn:     conn,
		terminal: terminal,
		width:    width,
		height:   height,
		local:    make(map[byte]bool),
		remote:   make(map[byte]bool),
	}
}

func (s *stream) Read(p []byte) (int, error) {
	s.readMu.Lock()
	defer s.readMu.Unlock()

	if len(p) == 0 {
		return 0, nil
	}
	buffer := make([]byte, 4096)
	for {
		if len(s.pending) > 0 {
			n := copy(p, s.pending)
			s.pending = s.pending[n:]
			return n, nil
		}
		if s.pendingErr != nil {
			err := s.pendingErr
			s.pendingErr = nil
			return 0, err
		}

		n, readErr := s.conn.Read(buffer)
		if n > 0 {
			if err := s.process(buffer[:n]); err != nil {
				s.pendingErr = fmt.Errorf("processing Telnet protocol: %w", err)
			}
		}
		if readErr != nil && s.pendingErr == nil {
			if errors.Is(readErr, io.EOF) {
				s.pendingErr = io.EOF
			} else {
				s.pendingErr = fmt.Errorf("reading Telnet connection: %w", readErr)
			}
		}
	}
}

func (s *stream) Write(data []byte) (int, error) {
	encoded := make([]byte, 0, len(data))
	for _, b := range data {
		encoded = append(encoded, b)
		if b == commandIAC {
			encoded = append(encoded, commandIAC)
		}
	}
	if err := s.writeBytes(encoded); err != nil {
		return 0, fmt.Errorf("writing Telnet data: %w", err)
	}
	return len(data), nil
}

func (s *stream) Resize(width, height int) error {
	if width <= 0 || height <= 0 {
		return fmt.Errorf("resizing Telnet session: dimensions must be positive")
	}
	if width > 65535 || height > 65535 {
		return fmt.Errorf("resizing Telnet session: dimensions exceed NAWS limits")
	}
	s.stateMu.Lock()
	s.width = width
	s.height = height
	enabled := s.local[optionNegotiateWindow]
	s.stateMu.Unlock()
	if enabled {
		if err := s.sendWindowSize(); err != nil {
			return fmt.Errorf("sending Telnet window size: %w", err)
		}
	}
	return nil
}

func (s *stream) Close() error {
	s.closeOnce.Do(func() {
		if err := s.conn.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			s.closeErr = fmt.Errorf("closing Telnet stream: %w", err)
		}
	})
	return s.closeErr
}

func (s *stream) process(data []byte) error {
	for _, b := range data {
		switch s.state {
		case stateData:
			if b == commandIAC {
				s.state = stateIAC
			} else {
				s.pending = append(s.pending, b)
			}
		case stateIAC:
			switch b {
			case commandIAC:
				s.pending = append(s.pending, commandIAC)
				s.state = stateData
			case commandDO, commandDONT, commandWILL, commandWONT:
				s.negotiation = b
				s.state = stateNegotiation
			case commandSB:
				s.state = stateSubOption
			default:
				s.state = stateData
			}
		case stateNegotiation:
			s.state = stateData
			if err := s.negotiate(s.negotiation, b); err != nil {
				return err
			}
		case stateSubOption:
			s.subOption = b
			s.subData = s.subData[:0]
			s.state = stateSubData
		case stateSubData:
			if b == commandIAC {
				s.state = stateSubIAC
			} else {
				s.subData = append(s.subData, b)
			}
		case stateSubIAC:
			switch b {
			case commandIAC:
				s.subData = append(s.subData, commandIAC)
				s.state = stateSubData
			case commandSE:
				s.state = stateData
				if err := s.subnegotiate(); err != nil {
					return err
				}
			default:
				s.state = stateData
			}
		}
	}
	return nil
}

func (s *stream) negotiate(command, option byte) error {
	switch command {
	case commandDO:
		if !supportsLocal(option) {
			return s.sendCommand(commandWONT, option)
		}
		s.stateMu.Lock()
		alreadyEnabled := s.local[option]
		s.local[option] = true
		s.stateMu.Unlock()
		if !alreadyEnabled {
			if err := s.sendCommand(commandWILL, option); err != nil {
				return err
			}
			if option == optionNegotiateWindow {
				return s.sendWindowSize()
			}
		}
	case commandDONT:
		s.stateMu.Lock()
		enabled := s.local[option]
		delete(s.local, option)
		s.stateMu.Unlock()
		if enabled {
			return s.sendCommand(commandWONT, option)
		}
	case commandWILL:
		if !supportsRemote(option) {
			return s.sendCommand(commandDONT, option)
		}
		s.stateMu.Lock()
		alreadyEnabled := s.remote[option]
		s.remote[option] = true
		s.stateMu.Unlock()
		if !alreadyEnabled {
			return s.sendCommand(commandDO, option)
		}
	case commandWONT:
		s.stateMu.Lock()
		enabled := s.remote[option]
		delete(s.remote, option)
		s.stateMu.Unlock()
		if enabled {
			return s.sendCommand(commandDONT, option)
		}
	}
	return nil
}

func supportsLocal(option byte) bool {
	switch option {
	case optionBinary, optionSuppressGoAhead, optionTerminalType, optionNegotiateWindow:
		return true
	default:
		return false
	}
}

func supportsRemote(option byte) bool {
	switch option {
	case optionBinary, optionEcho, optionSuppressGoAhead:
		return true
	default:
		return false
	}
}

func (s *stream) subnegotiate() error {
	if s.subOption != optionTerminalType || len(s.subData) == 0 || s.subData[0] != terminalTypeSend {
		return nil
	}
	payload := appendEscaped([]byte{commandIAC, commandSB, optionTerminalType, terminalTypeIs}, []byte(s.terminal))
	payload = append(payload, commandIAC, commandSE)
	return s.writeBytes(payload)
}

func (s *stream) sendWindowSize() error {
	s.stateMu.Lock()
	width := s.width
	height := s.height
	s.stateMu.Unlock()
	payload := appendEscaped([]byte{commandIAC, commandSB, optionNegotiateWindow}, []byte{
		byte(width >> 8), byte(width), byte(height >> 8), byte(height),
	})
	payload = append(payload, commandIAC, commandSE)
	return s.writeBytes(payload)
}

func (s *stream) sendCommand(command, option byte) error {
	return s.writeBytes([]byte{commandIAC, command, option})
}

func (s *stream) writeBytes(data []byte) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	for len(data) > 0 {
		n, err := s.conn.Write(data)
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

func appendEscaped(destination, data []byte) []byte {
	for _, b := range data {
		destination = append(destination, b)
		if b == commandIAC {
			destination = append(destination, commandIAC)
		}
	}
	return destination
}
