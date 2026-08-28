package ssh_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/castingcode/tuicast"
	tuissh "github.com/castingcode/tuicast/ssh"
	"github.com/castingcode/tuicast/vt220"
	. "github.com/smartystreets/goconvey/convey"
	xssh "golang.org/x/crypto/ssh"
)

type terminalRequest struct {
	Terminal string
	Width    uint32
	Height   uint32
}

func TestSSHConnector(t *testing.T) {
	Convey("SSH multiplexes interactive sessions over one authenticated connection", t, func() {
		peer, err := startSSHPeer()
		So(err, ShouldBeNil)
		defer peer.Close()

		connector, err := tuissh.NewConnector(tuissh.Config{
			Address: peer.Address(),
			ClientConfig: &xssh.ClientConfig{
				User:            "automation",
				HostKeyCallback: xssh.FixedHostKey(peer.PublicKey()),
			},
			TerminalModes: xssh.TerminalModes{xssh.ECHO: 0},
		})
		So(err, ShouldBeNil)
		logger := slog.New(slog.NewTextHandler(io.Discard, nil))
		server, err := tuicast.NewServer(logger)
		So(err, ShouldBeNil)
		connection, err := server.Connect(context.Background(), connector)
		So(err, ShouldBeNil)

		first := openSSHSession(connection)
		second := openSSHSession(connection)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_, err = first.WaitFor(ctx, tuicast.ScreenContains("SSH READY"))
		So(err, ShouldBeNil)
		_, err = second.WaitFor(ctx, tuicast.ScreenContains("SSH READY"))
		So(err, ShouldBeNil)

		So(first.Send([]byte(" FIRST")), ShouldBeNil)
		_, err = first.WaitFor(ctx, tuicast.ScreenContains("SSH READY FIRST"))
		So(err, ShouldBeNil)
		So(second.Send([]byte(" SECOND")), ShouldBeNil)
		_, err = second.WaitFor(ctx, tuicast.ScreenContains("SSH READY SECOND"))
		So(err, ShouldBeNil)

		for range 2 {
			request := <-peer.terminals
			So(request.Terminal, ShouldEqual, "vt220")
			So(request.Width, ShouldEqual, uint32(80))
			So(request.Height, ShouldEqual, uint32(24))
		}
		So(first.Resize(100, 40), ShouldBeNil)
		resize := <-peer.resizes
		So(resize.Width, ShouldEqual, uint32(100))
		So(resize.Height, ShouldEqual, uint32(40))
		So(server.Close(), ShouldBeNil)
	})

	Convey("SSH handshake honors context cancellation", t, func() {
		client, remote := net.Pipe()
		defer remote.Close()
		connector, err := tuissh.NewConnector(tuissh.Config{
			Address: "in-memory",
			ClientConfig: &xssh.ClientConfig{
				User:            "automation",
				HostKeyCallback: xssh.InsecureIgnoreHostKey(), // Test peer never completes a handshake.
			},
			DialContext: func(context.Context, string, string) (net.Conn, error) {
				return client, nil
			},
		})
		So(err, ShouldBeNil)
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()

		_, err = connector.Connect(ctx)

		So(err, ShouldNotBeNil)
	})
}

func openSSHSession(connection *tuicast.Connection) *tuicast.Session {
	terminal, err := vt220.New(80, 24)
	So(err, ShouldBeNil)
	session, err := connection.NewSession(context.Background(), terminal)
	So(err, ShouldBeNil)
	return session
}

type sshPeer struct {
	listener  net.Listener
	signer    xssh.Signer
	terminals chan terminalRequest
	resizes   chan terminalRequest
	done      chan struct{}
	closeOnce sync.Once
	mu        sync.Mutex
	conn      *xssh.ServerConn
}

func startSSHPeer() (*sshPeer, error) {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generating SSH host key: %w", err)
	}
	signer, err := xssh.NewSignerFromKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("creating SSH host signer: %w", err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("listening for SSH test peer: %w", err)
	}
	peer := &sshPeer{
		listener:  listener,
		signer:    signer,
		terminals: make(chan terminalRequest, 4),
		resizes:   make(chan terminalRequest, 4),
		done:      make(chan struct{}),
	}
	go peer.serve()
	return peer, nil
}

func (p *sshPeer) Address() string {
	return p.listener.Addr().String()
}

func (p *sshPeer) PublicKey() xssh.PublicKey {
	return p.signer.PublicKey()
}

func (p *sshPeer) Close() {
	p.closeOnce.Do(func() {
		_ = p.listener.Close()
		p.mu.Lock()
		if p.conn != nil {
			_ = p.conn.Close()
		}
		p.mu.Unlock()
		<-p.done
	})
}

func (p *sshPeer) serve() {
	defer close(p.done)
	raw, err := p.listener.Accept()
	if err != nil {
		return
	}
	config := &xssh.ServerConfig{NoClientAuth: true}
	config.AddHostKey(p.signer)
	connection, channels, requests, err := xssh.NewServerConn(raw, config)
	if err != nil {
		_ = raw.Close()
		return
	}
	p.mu.Lock()
	p.conn = connection
	p.mu.Unlock()
	go xssh.DiscardRequests(requests)
	for channel := range channels {
		if channel.ChannelType() != "session" {
			_ = channel.Reject(xssh.UnknownChannelType, "unsupported channel")
			continue
		}
		accepted, channelRequests, acceptErr := channel.Accept()
		if acceptErr != nil {
			continue
		}
		go p.serveSession(accepted, channelRequests)
	}
}

func (p *sshPeer) serveSession(channel xssh.Channel, requests <-chan *xssh.Request) {
	var echoOnce sync.Once
	for request := range requests {
		switch request.Type {
		case "pty-req":
			var payload struct {
				Terminal    string
				Width       uint32
				Height      uint32
				PixelWidth  uint32
				PixelHeight uint32
				Modes       string
			}
			if err := xssh.Unmarshal(request.Payload, &payload); err == nil {
				p.terminals <- terminalRequest{Terminal: payload.Terminal, Width: payload.Width, Height: payload.Height}
			}
			_ = request.Reply(true, nil)
		case "shell":
			_ = request.Reply(true, nil)
			echoOnce.Do(func() {
				_, _ = channel.Write([]byte("SSH READY"))
				go func() {
					_, _ = io.Copy(channel, channel)
				}()
			})
		case "window-change":
			var payload struct {
				Width       uint32
				Height      uint32
				PixelWidth  uint32
				PixelHeight uint32
			}
			if err := xssh.Unmarshal(request.Payload, &payload); err == nil {
				p.resizes <- terminalRequest{Width: payload.Width, Height: payload.Height}
			}
		default:
			_ = request.Reply(false, nil)
		}
	}
	_ = channel.Close()
}
