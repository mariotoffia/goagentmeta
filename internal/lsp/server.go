package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"sync"
)

// Server implements a minimal LSP server communicating over JSON-RPC 2.0 on stdio.
type Server struct {
	reader      *bufio.Reader
	writer      io.Writer
	documents   *DocumentStore
	rootURI     string
	initialized bool
	shutdown    bool
	mu          sync.Mutex
	logger      *log.Logger
}

// NewServer creates a Server that reads from in and writes to out.
func NewServer(in io.Reader, out io.Writer) *Server {
	return &Server{
		reader:    bufio.NewReader(in),
		writer:    out,
		documents: NewDocumentStore(),
		logger:    log.New(io.Discard, "[lsp] ", log.LstdFlags),
	}
}

// SetLogger sets a logger for diagnostics output (defaults to discard).
func (s *Server) SetLogger(l *log.Logger) {
	s.logger = l
}

// Run reads messages from the input, dispatches them, and writes responses
// until exit or an unrecoverable error.
func (s *Server) Run() error {
	for {
		body, err := ReadMessage(s.reader)
		if err != nil {
			// EOF is the normal shutdown path when the client closes the pipe.
			if err == io.EOF || isEOF(err) {
				return nil
			}
			return fmt.Errorf("read message: %w", err)
		}

		var req Request
		if err := json.Unmarshal(body, &req); err != nil {
			s.logger.Printf("invalid JSON-RPC message: %v", err)
			continue
		}

		exit, err := s.handleMessage(&req)
		if err != nil {
			s.logger.Printf("handle %s: %v", req.Method, err)
		}
		if exit {
			return nil
		}
	}
}

// handleMessage dispatches a single JSON-RPC request or notification.
// Returns (true, nil) when the server should exit.
func (s *Server) handleMessage(req *Request) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch req.Method {
	case "initialize":
		return false, s.handleInitialize(req)
	case "initialized":
		return false, s.handleInitialized(req)
	case "shutdown":
		return false, s.handleShutdown(req)
	case "exit":
		return true, nil
	case "textDocument/didOpen":
		return false, s.handleDidOpen(req)
	case "textDocument/didChange":
		return false, s.handleDidChange(req)
	case "textDocument/didSave":
		return false, s.handleDidSave(req)
	case "textDocument/didClose":
		return false, s.handleDidClose(req)
	default:
		if !req.IsNotification() {
			return false, WriteErrorResponse(s.writer, req.ID, CodeMethodNotFound,
				fmt.Sprintf("method not found: %s", req.Method))
		}
		return false, nil
	}
}

func (s *Server) handleInitialize(req *Request) error {
	var params InitializeParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return WriteErrorResponse(s.writer, req.ID, CodeInvalidParams, err.Error())
		}
	}

	s.rootURI = params.RootURI
	s.initialized = true

	result := InitializeResult{
		Capabilities: ServerCapabilities{
			TextDocumentSync: &TextDocumentSyncOptions{
				OpenClose: true,
				Change:    TextDocumentSyncFull,
				Save:      &SaveOptions{IncludeText: true},
			},
			DiagnosticProvider: &DiagnosticOptions{
				InterFileDependencies: true,
				WorkspaceDiagnostics:  false,
			},
		},
		ServerInfo: ServerInfo{
			Name:    "goagentmeta-lsp",
			Version: "0.1.0",
		},
	}

	s.logger.Printf("initialized with rootURI=%s", s.rootURI)
	return WriteResponse(s.writer, req.ID, result)
}

func (s *Server) handleInitialized(_ *Request) error {
	s.logger.Printf("client initialization complete")
	return nil
}

func (s *Server) handleShutdown(req *Request) error {
	s.shutdown = true
	s.logger.Printf("shutdown requested")
	return WriteResponse(s.writer, req.ID, nil)
}

func (s *Server) handleDidOpen(req *Request) error {
	var params DidOpenTextDocumentParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return fmt.Errorf("didOpen params: %w", err)
	}

	uri := params.TextDocument.URI
	text := params.TextDocument.Text
	s.documents.Open(uri, text)
	s.logger.Printf("opened %s", uri)

	return s.publishDiagnostics(uri, text)
}

func (s *Server) handleDidChange(req *Request) error {
	var params DidChangeTextDocumentParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return fmt.Errorf("didChange params: %w", err)
	}

	uri := params.TextDocument.URI
	if len(params.ContentChanges) == 0 {
		return nil
	}

	// Full sync — last change event has the full text.
	text := params.ContentChanges[len(params.ContentChanges)-1].Text
	s.documents.Update(uri, text)
	s.logger.Printf("changed %s", uri)

	return s.publishDiagnostics(uri, text)
}

func (s *Server) handleDidSave(req *Request) error {
	var params DidSaveTextDocumentParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return fmt.Errorf("didSave params: %w", err)
	}

	uri := params.TextDocument.URI
	var text string
	if params.Text != nil {
		text = *params.Text
		s.documents.Update(uri, text)
	} else {
		var ok bool
		text, ok = s.documents.Get(uri)
		if !ok {
			return nil
		}
	}
	s.logger.Printf("saved %s", uri)

	return s.publishDiagnostics(uri, text)
}

func (s *Server) handleDidClose(req *Request) error {
	var params DidCloseTextDocumentParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return fmt.Errorf("didClose params: %w", err)
	}

	uri := params.TextDocument.URI
	s.documents.Close(uri)
	s.logger.Printf("closed %s", uri)

	// Clear diagnostics for the closed document.
	return WriteNotification(s.writer, "textDocument/publishDiagnostics",
		PublishDiagnosticsParams{URI: uri, Diagnostics: []Diagnostic{}})
}

// publishDiagnostics runs validation on content and sends results to the client.
func (s *Server) publishDiagnostics(uri, content string) error {
	diags := DiagnoseYAML(uri, content)
	if diags == nil {
		diags = []Diagnostic{}
	}
	return WriteNotification(s.writer, "textDocument/publishDiagnostics",
		PublishDiagnosticsParams{URI: uri, Diagnostics: diags})
}

// ExitCode returns 0 if shutdown was requested before exit, 1 otherwise.
func (s *Server) ExitCode() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.shutdown {
		return 0
	}
	return 1
}

// isEOF checks whether an error wraps io.EOF.
func isEOF(err error) bool {
	if err == nil {
		return false
	}
	return err == io.EOF || err == io.ErrUnexpectedEOF ||
		fmt.Sprintf("%v", err) == "reading header: EOF"
}
