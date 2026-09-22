package opensearch

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
)

type mockIndexEntry struct {
	mapping        map[string]interface{}
	docs           map[string]map[string]interface{}
	aliasToIndices map[string][]string
}

type MockOpenSearchServer struct {
	mu      sync.RWMutex
	server  *http.Server
	url     string
	indices map[string]*mockIndexEntry
}

func NewMockOpenSearchServer() *MockOpenSearchServer {
	m := &MockOpenSearchServer{
		indices: make(map[string]*mockIndexEntry),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", m.handler)

	m.server = &http.Server{
		Handler: mux,
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(fmt.Sprintf("failed to create listener for mock OpenSearch: %v", err))
	}

	m.url = "http://" + ln.Addr().String()

	go func() {
		_ = m.server.Serve(ln)
	}()

	return m
}

func (m *MockOpenSearchServer) URL() string {
	return m.url
}

func (m *MockOpenSearchServer) Close() {
	_ = m.server.Close()
}

func (m *MockOpenSearchServer) handler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	method := r.Method

	// Ping / health
	if method == http.MethodHead && (path == "/" || path == "/_cluster/health" || path == "/_cluster/health/") {
		w.WriteHeader(http.StatusOK)
		return
	}

	if method == http.MethodGet && (path == "/_cluster/health" || path == "/_cluster/health/") {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":          "GREEN",
			"number_of_nodes": 1,
			"active_shards":   0,
		})
		return
	}

	// PUT /_aliases — bulk alias operations
	if method == http.MethodPost && path == "/_aliases" {
		m.handleUpdateAliases(w, r)
		return
	}

	// PUT /{index} — create index
	if method == http.MethodPut && !strings.HasPrefix(path, "/_") &&
		!strings.Contains(path, "/_doc/") &&
		!strings.Contains(path, "/_search") &&
		!strings.Contains(path, "/_alias") {
		m.handleCreateIndex(w, r, path)
		return
	}

	// HEAD /{index} — index exists
	if method == http.MethodHead && !strings.HasPrefix(path, "/_") {
		m.handleIndexExists(w, r, path)
		return
	}

	// DELETE /{index} — delete index
	if method == http.MethodDelete && !strings.HasPrefix(path, "/_") &&
		!strings.Contains(path, "/_doc/") {
		m.handleDeleteIndex(w, r, path)
		return
	}

	// PUT /{index}/_aliases/{alias}
	if method == http.MethodPut && strings.Contains(path, "/_aliases/") {
		m.handlePutAlias(w, r, path)
		return
	}

	// GET /{index}/_alias/{alias} or GET /_alias/{alias}
	if method == http.MethodGet && strings.Contains(path, "/_alias/") {
		m.handleGetAlias(w, r, path)
		return
	}

	// PUT /{index}/_doc/{docID}
	if method == http.MethodPut && strings.Contains(path, "/_doc/") {
		m.handleIndexDocument(w, r, path)
		return
	}

	// DELETE /{index}/_doc/{docID}
	if method == http.MethodDelete && strings.Contains(path, "/_doc/") {
		m.handleDeleteDocument(w, r, path)
		return
	}

	// POST /{index}/_search
	if method == http.MethodPost && strings.HasSuffix(path, "/_search") {
		m.handleSearch(w, r, path)
		return
	}

	// POST /{index}/_bulk
	if method == http.MethodPost && strings.HasSuffix(path, "/_bulk") {
		m.handleBulk(w, r, path)
		return
	}

	http.NotFound(w, r)
}

func (m *MockOpenSearchServer) handleIndexExists(w http.ResponseWriter, r *http.Request, path string) {
	indexNames := strings.TrimPrefix(path, "/")
	for _, name := range strings.Split(indexNames, ",") {
		name = strings.TrimSpace(name)
		m.mu.RLock()
		_, exists := m.indices[name]
		m.mu.RUnlock()
		if !exists {
			w.WriteHeader(http.StatusNotFound)
			return
		}
	}
	w.WriteHeader(http.StatusOK)
}

func (m *MockOpenSearchServer) handleCreateIndex(w http.ResponseWriter, r *http.Request, path string) {
	indexName := strings.TrimPrefix(path, "/")

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.indices[indexName]; exists {
		w.WriteHeader(http.StatusOK)
		return
	}

	var mapping map[string]interface{}
	body, _ := io.ReadAll(r.Body)
	if len(body) > 0 {
		_ = json.Unmarshal(body, &mapping)
	}

	m.indices[indexName] = &mockIndexEntry{
		mapping:        mapping,
		docs:           make(map[string]map[string]interface{}),
		aliasToIndices: make(map[string][]string),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"acknowledged":        true,
		"shards_acknowledged": true,
		"index":               indexName,
	})
}

func (m *MockOpenSearchServer) handleDeleteIndex(w http.ResponseWriter, r *http.Request, path string) {
	indexName := strings.TrimPrefix(path, "/")

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.indices[indexName]; !exists {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	delete(m.indices, indexName)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"acknowledged": true})
}

func (m *MockOpenSearchServer) handlePutAlias(w http.ResponseWriter, r *http.Request, path string) {
	parts := strings.SplitN(path, "/_aliases/", 2)
	if len(parts) != 2 {
		http.NotFound(w, r)
		return
	}
	indexName := strings.TrimPrefix(parts[0], "/")
	aliasName := parts[1]

	m.mu.Lock()
	defer m.mu.Unlock()

	idx, ok := m.indices[indexName]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	found := false
	for _, a := range idx.aliasToIndices[aliasName] {
		if a == indexName {
			found = true
			break
		}
	}
	if !found {
		idx.aliasToIndices[aliasName] = append(idx.aliasToIndices[aliasName], indexName)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"acknowledged": true})
}

func (m *MockOpenSearchServer) handleGetAlias(w http.ResponseWriter, r *http.Request, path string) {
	var indexName, aliasName string

	if strings.Contains(path, "/_alias/") {
		parts := strings.SplitN(path, "/_alias/", 2)
		aliasName = parts[len(parts)-1]
		if len(parts) > 1 && strings.TrimPrefix(parts[0], "/") != "" {
			indexName = strings.TrimPrefix(parts[0], "/")
		}
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]interface{})

	if indexName != "" {
		idx, ok := m.indices[indexName]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if aliasName != "" {
			indices := idx.aliasToIndices[aliasName]
			if len(indices) == 0 {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			result[indexName] = map[string]interface{}{
				"aliases": map[string]interface{}{
					aliasName: map[string]interface{}{},
				},
			}
		}
	} else if aliasName != "" {
		for idxName, idx := range m.indices {
			for _, a := range idx.aliasToIndices[aliasName] {
				if a == idxName {
					result[idxName] = map[string]interface{}{
						"aliases": map[string]interface{}{
							aliasName: map[string]interface{}{},
						},
					}
				}
			}
		}
		if len(result) == 0 {
			w.WriteHeader(http.StatusNotFound)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(result)
}

func (m *MockOpenSearchServer) handleUpdateAliases(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var updateBody struct {
		Actions []map[string]interface{} `json:"actions"`
	}
	if err := json.Unmarshal(body, &updateBody); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, action := range updateBody.Actions {
		if add, ok := action["add"].(map[string]interface{}); ok {
			indexName, _ := add["index"].(string)
			aliasName, _ := add["alias"].(string)
			if idx, ok := m.indices[indexName]; ok {
				found := false
				for _, a := range idx.aliasToIndices[aliasName] {
					if a == indexName {
						found = true
						break
					}
				}
				if !found {
					idx.aliasToIndices[aliasName] = append(idx.aliasToIndices[aliasName], indexName)
				}
			}
		}
		if remove, ok := action["remove"].(map[string]interface{}); ok {
			indexName, _ := remove["index"].(string)
			aliasName, _ := remove["alias"].(string)
			if idx, ok := m.indices[indexName]; ok {
				newList := make([]string, 0, len(idx.aliasToIndices[aliasName]))
				for _, a := range idx.aliasToIndices[aliasName] {
					if a != indexName {
						newList = append(newList, a)
					}
				}
				idx.aliasToIndices[aliasName] = newList
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"acknowledged": true})
}

func (m *MockOpenSearchServer) handleIndexDocument(w http.ResponseWriter, r *http.Request, path string) {
	parts := strings.SplitN(path, "/_doc/", 2)
	if len(parts) != 2 {
		http.NotFound(w, r)
		return
	}
	indexName := strings.TrimPrefix(parts[0], "/")
	docID := parts[1]

	m.mu.Lock()
	defer m.mu.Unlock()

	idx, ok := m.indices[indexName]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	var doc map[string]interface{}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	idx.docs[docID] = doc

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"_index": indexName,
		"_id":    docID,
		"result": "created",
	})
}

func (m *MockOpenSearchServer) handleDeleteDocument(w http.ResponseWriter, r *http.Request, path string) {
	parts := strings.SplitN(path, "/_doc/", 2)
	if len(parts) != 2 {
		http.NotFound(w, r)
		return
	}
	indexName := strings.TrimPrefix(parts[0], "/")
	docID := parts[1]

	m.mu.Lock()
	defer m.mu.Unlock()

	idx, ok := m.indices[indexName]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if _, exists := idx.docs[docID]; !exists {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	delete(idx.docs, docID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"_index": indexName,
		"_id":    docID,
		"result": "deleted",
	})
}

func (m *MockOpenSearchServer) handleSearch(w http.ResponseWriter, r *http.Request, path string) {
	// Path: /{index_or_alias}/_search
	identifier := strings.TrimPrefix(path, "/")
	identifier = strings.TrimSuffix(identifier, "/_search")

	m.mu.RLock()
	defer m.mu.RUnlock()

	var docs []map[string]interface{}

	for idxName, idx := range m.indices {
		if idxName == identifier || indexHasAlias(idx, identifier) {
			for _, doc := range idx.docs {
				docs = append(docs, doc)
			}
		}
	}

	// Deduplicate by listing_id
	seen := make(map[string]bool)
	uniqueDocs := make([]map[string]interface{}, 0, len(docs))
	for _, doc := range docs {
		id, _ := doc["listing_id"].(string)
		if id == "" {
			id = fmt.Sprintf("%p", doc)
		}
		if !seen[id] {
			seen[id] = true
			uniqueDocs = append(uniqueDocs, doc)
		}
	}

	hits := make([]map[string]interface{}, 0, len(uniqueDocs))
	for _, doc := range uniqueDocs {
		hits = append(hits, map[string]interface{}{
			"_index":  identifier,
			"_id":     getDocID(doc),
			"_score":  1.0,
			"_source": doc,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"took": 1,
		"hits": map[string]interface{}{
			"total": map[string]interface{}{
				"value":    len(uniqueDocs),
				"relation": "eq",
			},
			"max_score": 1.0,
			"hits":      hits,
		},
	})
}

func (m *MockOpenSearchServer) handleBulk(w http.ResponseWriter, r *http.Request, path string) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	indexName := strings.TrimPrefix(path, "/")
	indexName = strings.TrimSuffix(indexName, "/_bulk")

	m.mu.Lock()
	defer m.mu.Unlock()

	idx, ok := m.indices[indexName]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	for i := 0; i+1 < len(lines); i += 2 {
		var meta map[string]interface{}
		if err := json.Unmarshal([]byte(lines[i]), &meta); err != nil {
			continue
		}
		var doc map[string]interface{}
		if err := json.Unmarshal([]byte(lines[i+1]), &doc); err != nil {
			continue
		}

		indexMeta, ok := meta["index"].(map[string]interface{})
		if !ok {
			continue
		}
		docID, _ := indexMeta["_id"].(string)
		if docID == "" {
			docID = fmt.Sprintf("doc-%d", i)
		}
		idx.docs[docID] = doc
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"took":   1,
		"errors": false,
		"items":  []interface{}{},
	})
}

func indexHasAlias(idx *mockIndexEntry, alias string) bool {
	return len(idx.aliasToIndices[alias]) > 0
}

func getDocID(doc map[string]interface{}) string {
	if id, ok := doc["listing_id"].(string); ok && id != "" {
		return id
	}
	return "unknown"
}
