// Package driver exposes TUICast through a language-neutral JSON-RPC protocol.
package driver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

const protocolVersion = "1"

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *responseError  `json:"error,omitempty"`
}

type notification struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params"`
}

type responseError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *responseError) Error() string {
	return e.Message
}

func invalidParams(format string, arguments ...any) *responseError {
	return &responseError{Code: -32602, Message: fmt.Sprintf(format, arguments...)}
}

func applicationError(err error) *responseError {
	return &responseError{Code: -32000, Message: err.Error()}
}

func decodeParams(data json.RawMessage, destination any) *responseError {
	if len(data) == 0 {
		data = []byte("{}")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return invalidParams("invalid parameters: %v", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return invalidParams("invalid parameters: expected one JSON value")
	}
	return nil
}

// ConnectionOptions describes a transport connection requested through the
// driver. SSH host-key verification must use either KnownHostsFile,
// HostKeyFingerprint, or the explicit insecure option.
type ConnectionOptions struct {
	Protocol                   string `json:"protocol"`
	Address                    string `json:"address"`
	Username                   string `json:"username,omitempty"`
	Password                   string `json:"password,omitempty"`
	PrivateKey                 string `json:"privateKey,omitempty"`
	PrivateKeyPassphrase       string `json:"privateKeyPassphrase,omitempty"`
	KnownHostsFile             string `json:"knownHostsFile,omitempty"`
	HostKeyFingerprint         string `json:"hostKeyFingerprint,omitempty"`
	InsecureSkipHostKeyCheck   bool   `json:"insecureSkipHostKeyCheck,omitempty"`
	ConnectTimeoutMilliseconds int64  `json:"connectTimeoutMilliseconds,omitempty"`
}

// Matcher describes a serializable screen expectation. Exactly one field must
// be set.
type Matcher struct {
	Contains *string        `json:"contains,omitempty"`
	Line     *LineMatcher   `json:"line,omitempty"`
	Cursor   *CursorMatcher `json:"cursor,omitempty"`
	All      []Matcher      `json:"all,omitempty"`
	Any      []Matcher      `json:"any,omitempty"`
	Not      *Matcher       `json:"not,omitempty"`
}

type LineMatcher struct {
	Row  int    `json:"row"`
	Text string `json:"text"`
}

type CursorMatcher struct {
	Column int `json:"column"`
	Row    int `json:"row"`
}

type screenResult struct {
	Width    int          `json:"width"`
	Height   int          `json:"height"`
	Cells    []cellResult `json:"cells"`
	Cursor   cursorResult `json:"cursor"`
	Revision uint64       `json:"revision"`
	Text     string       `json:"text"`
}

type cellResult struct {
	Text       string `json:"text"`
	Width      int    `json:"width"`
	Foreground int16  `json:"foreground"`
	Background int16  `json:"background"`
	Attributes uint16 `json:"attributes"`
}

type cursorResult struct {
	Column  int  `json:"column"`
	Row     int  `json:"row"`
	Visible bool `json:"visible"`
}

type terminalEventResult struct {
	Sequence uint64 `json:"sequence"`
	Type     string `json:"type"`
	Data     string `json:"data,omitempty"`
}
