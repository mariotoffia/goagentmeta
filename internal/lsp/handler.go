package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// ReadMessage reads one LSP message from r (Content-Length header + JSON body).
// Returns the raw JSON body or an error.
func ReadMessage(r *bufio.Reader) ([]byte, error) {
	contentLength := -1

	// Read headers until the blank line.
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("reading header: %w", err)
		}

		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}

		parts := strings.SplitN(line, ": ", 2)
		if len(parts) != 2 {
			continue
		}

		if strings.EqualFold(parts[0], "Content-Length") {
			n, err := strconv.Atoi(parts[1])
			if err != nil {
				return nil, fmt.Errorf("invalid Content-Length %q: %w", parts[1], err)
			}
			contentLength = n
		}
	}

	if contentLength < 0 {
		return nil, fmt.Errorf("missing Content-Length header")
	}

	body := make([]byte, contentLength)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, fmt.Errorf("reading body: %w", err)
	}

	return body, nil
}

// WriteMessage writes one LSP message to w with Content-Length header.
func WriteMessage(w io.Writer, body []byte) error {
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))
	if _, err := io.WriteString(w, header); err != nil {
		return err
	}
	_, err := w.Write(body)
	return err
}

// WriteResponse writes a JSON-RPC response for the given request ID.
func WriteResponse(w io.Writer, id *json.RawMessage, result any) error {
	resp := Response{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	body, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	return WriteMessage(w, body)
}

// WriteErrorResponse writes a JSON-RPC error response.
func WriteErrorResponse(w io.Writer, id *json.RawMessage, code int, message string) error {
	resp := Response{
		JSONRPC: "2.0",
		ID:      id,
		Error: &ResponseError{
			Code:    code,
			Message: message,
		},
	}
	body, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	return WriteMessage(w, body)
}

// WriteNotification writes a JSON-RPC notification (no ID).
func WriteNotification(w io.Writer, method string, params any) error {
	notif := Notification{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}
	body, err := json.Marshal(notif)
	if err != nil {
		return err
	}
	return WriteMessage(w, body)
}
