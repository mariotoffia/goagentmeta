package lsp

import "sync"

// DocumentStore is a thread-safe in-memory store of open text documents.
type DocumentStore struct {
	mu   sync.RWMutex
	docs map[string]string // URI → content
}

// NewDocumentStore returns an empty DocumentStore.
func NewDocumentStore() *DocumentStore {
	return &DocumentStore{docs: make(map[string]string)}
}

// Open stores a newly opened document.
func (ds *DocumentStore) Open(uri, text string) {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	ds.docs[uri] = text
}

// Update replaces the content for an already-open document.
func (ds *DocumentStore) Update(uri, text string) {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	ds.docs[uri] = text
}

// Close removes a document from the store.
func (ds *DocumentStore) Close(uri string) {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	delete(ds.docs, uri)
}

// Get returns the content for a document and whether it exists.
func (ds *DocumentStore) Get(uri string) (string, bool) {
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	text, ok := ds.docs[uri]
	return text, ok
}
