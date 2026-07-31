// Package protocol implements the JSON-RPC transport used by the Sec language
// server. It deliberately contains no Sec language semantics or LSP feature
// routing, so the server can use it as a protocol boundary.
package protocol

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Message is a JSON-RPC request, notification, or error response envelope.
// Params remains raw because each routed LSP method owns its parameter type.
type Message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

// Error is a JSON-RPC error payload.
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ResponseMessage is a success response envelope. A distinct type preserves an
// explicit JSON null result, which is required for requests such as shutdown.
type ResponseMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result"`
}

// ReadMessage reads one LSP Content-Length-framed JSON-RPC message.
func ReadMessage(reader *bufio.Reader) (Message, error) {
	contentLength := -1
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return Message{}, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(key), "Content-Length") {
			length, err := strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				return Message{}, err
			}
			contentLength = length
		}
	}
	if contentLength < 0 {
		return Message{}, fmt.Errorf("missing Content-Length header")
	}

	body := make([]byte, contentLength)
	if _, err := io.ReadFull(reader, body); err != nil {
		return Message{}, err
	}

	var message Message
	if err := json.Unmarshal(body, &message); err != nil {
		return Message{}, err
	}
	return message, nil
}

// WriteMessage writes one JSON-RPC message using LSP Content-Length framing.
// Callers with concurrent writers must serialize calls around this function.
func WriteMessage(writer io.Writer, message any) error {
	body, err := json.Marshal(message)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "Content-Length: %d\r\n\r\n", len(body)); err != nil {
		return err
	}
	_, err = writer.Write(body)
	return err
}
