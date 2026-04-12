package lsp

import (
	"bufio"
	"encoding/json"
	"io"
	"testing"
)

// --- Test helpers ---

func sendRequest(t *testing.T, w io.Writer, id int, method string, params any) {
	t.Helper()
	rawID := json.RawMessage(jsonMarshal(t, id))
	req := Request{
		JSONRPC: "2.0",
		ID:      &rawID,
		Method:  method,
	}
	if params != nil {
		req.Params = json.RawMessage(jsonMarshal(t, params))
	}
	body := jsonMarshal(t, req)
	if err := WriteMessage(w, body); err != nil {
		t.Fatalf("sendRequest(%s): %v", method, err)
	}
}

func sendNotification(t *testing.T, w io.Writer, method string, params any) {
	t.Helper()
	req := Request{
		JSONRPC: "2.0",
		Method:  method,
	}
	if params != nil {
		req.Params = json.RawMessage(jsonMarshal(t, params))
	}
	body := jsonMarshal(t, req)
	if err := WriteMessage(w, body); err != nil {
		t.Fatalf("sendNotification(%s): %v", method, err)
	}
}

func readResponse(t *testing.T, r *bufio.Reader) Response {
	t.Helper()
	body, err := ReadMessage(r)
	if err != nil {
		t.Fatalf("readResponse: %v", err)
	}
	var resp Response
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("readResponse unmarshal: %v", err)
	}
	return resp
}

func readNotification(t *testing.T, r *bufio.Reader) Request {
	t.Helper()
	body, err := ReadMessage(r)
	if err != nil {
		t.Fatalf("readNotification: %v", err)
	}
	var req Request
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatalf("readNotification unmarshal: %v", err)
	}
	return req
}

func jsonMarshal(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	return b
}

// startTestServer creates a Server wired to pipes and runs it in a goroutine.
// Returns (writer-to-server, reader-from-server, done-channel).
func startTestServer(t *testing.T) (io.Writer, *bufio.Reader, <-chan error) {
	t.Helper()

	clientToServer_r, clientToServer_w := io.Pipe()
	serverToClient_r, serverToClient_w := io.Pipe()

	srv := NewServer(clientToServer_r, serverToClient_w)

	done := make(chan error, 1)
	go func() {
		done <- srv.Run()
		serverToClient_w.Close()
	}()

	t.Cleanup(func() {
		clientToServer_w.Close()
	})

	return clientToServer_w, bufio.NewReader(serverToClient_r), done
}

// --- Tests ---

func TestServerLifecycle(t *testing.T) {
	w, r, done := startTestServer(t)

	// 1. Initialize
	sendRequest(t, w, 1, "initialize", InitializeParams{RootURI: "file:///test"})
	resp := readResponse(t, r)
	if resp.Error != nil {
		t.Fatalf("initialize error: %s", resp.Error.Message)
	}

	var result InitializeResult
	raw := jsonMarshal(t, resp.Result)
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("unmarshal InitializeResult: %v", err)
	}
	if result.ServerInfo.Name != "goagentmeta-lsp" {
		t.Errorf("server name = %q, want %q", result.ServerInfo.Name, "goagentmeta-lsp")
	}
	if result.Capabilities.TextDocumentSync == nil {
		t.Fatal("expected TextDocumentSync capability")
	}
	if result.Capabilities.TextDocumentSync.Change != TextDocumentSyncFull {
		t.Errorf("sync change = %d, want %d", result.Capabilities.TextDocumentSync.Change, TextDocumentSyncFull)
	}

	// 2. Initialized notification
	sendNotification(t, w, "initialized", nil)
	// No response expected for notifications.

	// 3. Shutdown
	sendRequest(t, w, 2, "shutdown", nil)
	resp = readResponse(t, r)
	if resp.Error != nil {
		t.Fatalf("shutdown error: %s", resp.Error.Message)
	}

	// 4. Exit
	sendNotification(t, w, "exit", nil)

	// Wait for server to stop.
	if err := <-done; err != nil {
		t.Fatalf("server exited with error: %v", err)
	}
}

func TestMethodNotFound(t *testing.T) {
	w, r, done := startTestServer(t)

	// Initialize first.
	sendRequest(t, w, 1, "initialize", InitializeParams{})
	readResponse(t, r)

	// Unknown method.
	sendRequest(t, w, 2, "textDocument/hover", nil)
	resp := readResponse(t, r)
	if resp.Error == nil {
		t.Fatal("expected error for unknown method")
	}
	if resp.Error.Code != CodeMethodNotFound {
		t.Errorf("error code = %d, want %d", resp.Error.Code, CodeMethodNotFound)
	}

	// Shutdown + exit.
	sendRequest(t, w, 3, "shutdown", nil)
	readResponse(t, r)
	sendNotification(t, w, "exit", nil)
	<-done
}

func TestDocumentSyncOpenClose(t *testing.T) {
	w, r, done := startTestServer(t)

	// Initialize.
	sendRequest(t, w, 1, "initialize", InitializeParams{})
	readResponse(t, r)
	sendNotification(t, w, "initialized", nil)

	// Open a valid YAML document.
	yamlContent := "kind: agent\nid: my-agent\ndescription: test\n"
	sendNotification(t, w, "textDocument/didOpen", DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{
			URI:        "file:///test/agent.yaml",
			LanguageID: "yaml",
			Version:    1,
			Text:       yamlContent,
		},
	})

	// Should get diagnostics notification (empty for valid YAML).
	notif := readNotification(t, r)
	if notif.Method != "textDocument/publishDiagnostics" {
		t.Fatalf("expected publishDiagnostics, got %s", notif.Method)
	}

	var diagParams PublishDiagnosticsParams
	if err := json.Unmarshal(notif.Params, &diagParams); err != nil {
		t.Fatalf("unmarshal diag params: %v", err)
	}
	if diagParams.URI != "file:///test/agent.yaml" {
		t.Errorf("diag URI = %q, want %q", diagParams.URI, "file:///test/agent.yaml")
	}
	if len(diagParams.Diagnostics) != 0 {
		t.Errorf("expected 0 diagnostics, got %d: %+v", len(diagParams.Diagnostics), diagParams.Diagnostics)
	}

	// Close the document — should clear diagnostics.
	sendNotification(t, w, "textDocument/didClose", DidCloseTextDocumentParams{
		TextDocument: TextDocumentIdentifier{URI: "file:///test/agent.yaml"},
	})
	notif = readNotification(t, r)
	if notif.Method != "textDocument/publishDiagnostics" {
		t.Fatalf("expected publishDiagnostics on close, got %s", notif.Method)
	}
	if err := json.Unmarshal(notif.Params, &diagParams); err != nil {
		t.Fatalf("unmarshal close diag params: %v", err)
	}
	if len(diagParams.Diagnostics) != 0 {
		t.Errorf("expected 0 diagnostics on close, got %d", len(diagParams.Diagnostics))
	}

	// Shutdown + exit.
	sendRequest(t, w, 2, "shutdown", nil)
	readResponse(t, r)
	sendNotification(t, w, "exit", nil)
	<-done
}

func TestDocumentChange(t *testing.T) {
	w, r, done := startTestServer(t)

	// Initialize.
	sendRequest(t, w, 1, "initialize", InitializeParams{})
	readResponse(t, r)
	sendNotification(t, w, "initialized", nil)

	// Open a YAML with an error (missing id).
	sendNotification(t, w, "textDocument/didOpen", DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{
			URI:  "file:///test/skill.yaml",
			Text: "kind: skill\ndescription: test\n",
		},
	})
	notif := readNotification(t, r)
	var diagParams PublishDiagnosticsParams
	if err := json.Unmarshal(notif.Params, &diagParams); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(diagParams.Diagnostics) == 0 {
		t.Fatal("expected diagnostics for missing id")
	}

	// Fix the document — add "id" field.
	sendNotification(t, w, "textDocument/didChange", DidChangeTextDocumentParams{
		TextDocument: VersionedTextDocumentIdentifier{
			URI:     "file:///test/skill.yaml",
			Version: 2,
		},
		ContentChanges: []TextDocumentContentChangeEvent{
			{Text: "kind: skill\nid: my-skill\ndescription: test\n"},
		},
	})
	notif = readNotification(t, r)
	if err := json.Unmarshal(notif.Params, &diagParams); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(diagParams.Diagnostics) != 0 {
		t.Errorf("expected 0 diagnostics after fix, got %d: %+v", len(diagParams.Diagnostics), diagParams.Diagnostics)
	}

	// Shutdown + exit.
	sendRequest(t, w, 2, "shutdown", nil)
	readResponse(t, r)
	sendNotification(t, w, "exit", nil)
	<-done
}

func TestDocumentSave(t *testing.T) {
	w, r, done := startTestServer(t)

	// Initialize.
	sendRequest(t, w, 1, "initialize", InitializeParams{})
	readResponse(t, r)

	// Open.
	text := "kind: agent\nid: a1\n"
	sendNotification(t, w, "textDocument/didOpen", DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{URI: "file:///x.yaml", Text: text},
	})
	readNotification(t, r) // consume open diagnostics

	// Save with new text.
	newText := "kind: agent\nid: a1\ndescription: saved\n"
	sendNotification(t, w, "textDocument/didSave", DidSaveTextDocumentParams{
		TextDocument: TextDocumentIdentifier{URI: "file:///x.yaml"},
		Text:         &newText,
	})
	notif := readNotification(t, r)
	if notif.Method != "textDocument/publishDiagnostics" {
		t.Fatalf("expected publishDiagnostics, got %s", notif.Method)
	}

	// Shutdown + exit.
	sendRequest(t, w, 2, "shutdown", nil)
	readResponse(t, r)
	sendNotification(t, w, "exit", nil)
	<-done
}
