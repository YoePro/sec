package protocol

import (
	"bufio"
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

func TestReadMessageParsesContentLengthFraming(t *testing.T) {
	body := []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)
	input := "Content-Type: application/vscode-jsonrpc; charset=utf-8\r\nContent-Length: " +
		strconv.Itoa(len(body)) + "\r\n\r\n" + string(body)

	message, err := ReadMessage(bufio.NewReader(strings.NewReader(input)))
	if err != nil {
		t.Fatalf("ReadMessage returned error: %v", err)
	}
	if message.JSONRPC != "2.0" || message.Method != "initialize" || string(message.ID) != "1" {
		t.Fatalf("wrong message: %#v", message)
	}
}

func TestWriteMessagePreservesExplicitNullResult(t *testing.T) {
	var out bytes.Buffer
	if err := WriteMessage(&out, ResponseMessage{JSONRPC: "2.0", ID: json.RawMessage("1"), Result: nil}); err != nil {
		t.Fatalf("WriteMessage returned error: %v", err)
	}
	if !strings.Contains(out.String(), "Content-Length:") || !strings.Contains(out.String(), `"result":null`) {
		t.Fatalf("wrong framed response: %q", out.String())
	}
}
