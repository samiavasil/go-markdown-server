package main

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/mongo"

	"github.com/beldmian/go-markdown-server/db"
	"github.com/beldmian/go-markdown-server/plantuml"
	"github.com/gorilla/mux"
	"github.com/russross/blackfriday"
)

// Post ...
type Post struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	URL   string `json:"url"`
}

var (
	port       string
	collection *mongo.Collection
	syncDir    string
	autoSync   bool
	clients    = make(map[chan string]bool)
	clientsMux sync.Mutex
)

func init() {
	if os.Getenv("PORT") != "" {
		port = ":" + os.Getenv("PORT")
	} else {
		port = ":8080"
	}
	
	// Sync directory for markdown files
	syncDir = os.Getenv("SYNC_DIR")
	if syncDir == "" {
		syncDir = "./content"
	}

	// Auto sync flag
	autoSyncEnv := os.Getenv("AUTO_SYNC")
	autoSync = strings.ToLower(autoSyncEnv) == "true"
	log.Printf("DEBUG: AUTO_SYNC env='%s', parsed=%v", autoSyncEnv, autoSync)
}

func main() {
	collectionResp, err := db.ConnectToDB()
	if err != nil {
		log.Fatal(err)
	}
	collection = collectionResp
	
	// Check for sync command
	if len(os.Args) > 1 && os.Args[1] == "sync" {
		runSync()
		return
	}
	
	if autoSync {
		log.Printf("AUTO_SYNC enabled. Scanning '%s' for markdown collections...", syncDir)
		if err := autoSyncFromContent(); err != nil {
			log.Printf("Auto sync warning: %v", err)
		}
	} else {
		log.Printf("AUTO_SYNC disabled. Skipping initial content import.")
	}

	r := mux.NewRouter()
	configureRouter(r)

	log.Printf("Server starting on %s", port)
	log.Printf("Sync directory: %s", syncDir)
	
	// Start file watcher if AUTO_SYNC enabled
	if autoSync {
		go watchContentDirectory()
	}
	
	if err := http.ListenAndServe(port, r); err != nil {
		log.Fatal(err)
	}
}

func runSync() {
	log.Printf("Starting file sync from: %s", syncDir)
	// Import filesync package and run sync
	log.Fatal("Sync functionality - use /api/sync endpoint")
}

// autoSyncFromContent walks SYNC_DIR and imports markdown files into Mongo.
// Uses UPSERT to re-import deleted files (from content/ directory only).
// Directory naming convention: each first-level subdirectory under SYNC_DIR becomes a collection.
// Files directly under SYNC_DIR (without subdirectory) go to collection "root".
func autoSyncFromContent() error {
	// Ensure base exists
	if _, err := os.Stat(syncDir); err != nil {
		return err
	}

	// Walk syncDir and UPSERT all files (no duplicate check - content/ is source of truth)
	return filepath.Walk(syncDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		// Only .md files
		if !strings.HasSuffix(strings.ToLower(info.Name()), ".md") {
			return nil
		}

		rel, _ := filepath.Rel(syncDir, path)
		parts := strings.Split(rel, string(filepath.Separator))
		var collectionName string
		var filename string
		if len(parts) == 1 {
			collectionName = "content/root"
			filename = parts[0]
		} else {
			// Prefix with "content/" to distinguish from uploaded collections
			collectionName = "content/" + parts[0]
			filename = strings.Join(parts[1:], "/")
		}

		// Derive title/url
		title := strings.TrimSuffix(filepath.Base(filename), ".md")
		url := strings.ToLower(strings.ReplaceAll(title, " ", "-"))
		url = strings.ReplaceAll(url, "_", "-")
		isIndex := false
		lowerName := strings.ToLower(filepath.Base(filename))
		if lowerName == "index.md" || lowerName == "readme.md" {
			isIndex = true
			// Replace slashes with dashes for URL-friendly index slug
			url = strings.ReplaceAll(collectionName, "/", "-") + "-index"
		}

		contentBytes, readErr := ioutil.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		body := string(contentBytes)
		
		// Process PlantUML diagrams with base-aware resolver (use original folder name without prefix)
		baseDir := strings.TrimPrefix(collectionName, "content/")
		body = plantuml.ProcessPlantUMLWithBase(body, baseDir)

		post := db.Post{
			Title:      title,
			URL:        url,
			Body:       body,
			Collection: collectionName,
			IsIndex:    isIndex,
		}
		
		// Use UPSERT to re-import files deleted from GUI
		if insErr := db.UpsertPost(collection, post, "124252"); insErr != nil {
			log.Printf("Failed to upsert '%s' (%s): %v", title, path, insErr)
		} else {
			log.Printf("Synced '%s' -> collection '%s' (index=%v)", title, collectionName, isIndex)
		}
		return nil
	})
}

// deletePostFromDB removes a post from database based on file path
func deletePostFromDB(filePath string) error {
	// Extract collection name from path: /app/content/{CollectionName}/...
	relPath := strings.TrimPrefix(filePath, syncDir+"/")
	parts := strings.Split(relPath, "/")
	if len(parts) < 1 {
		return fmt.Errorf("invalid file path: %s", filePath)
	}
	
	// Prefix with "content/" to match auto-sync collection naming
	collectionName := "content/" + parts[0]
	fileName := filepath.Base(filePath)
	
	// Check if this is an index file (README.md or index.md)
	lowerFilename := strings.ToLower(fileName)
	isIndex := (lowerFilename == "index.md" || lowerFilename == "readme.md")
	
	var url string
	if isIndex {
		// For index files, URL is {collection}-index
		url = collectionName + "-index"
	} else {
		// Create URL using same logic as filesync - convert path to slug
		url = strings.TrimSuffix(relPath, ".md")
		url = filepath.ToSlash(url)
		url = strings.ToLower(url)
		url = strings.ReplaceAll(url, " ", "-")
		url = strings.ReplaceAll(url, "/", "-")
		// Remove multiple consecutive dashes
		for strings.Contains(url, "--") {
			url = strings.ReplaceAll(url, "--", "-")
		}
	}
	
	log.Printf("DEBUG: Deleting post - collection=%s, url=%s, isIndex=%v, path=%s", collectionName, url, isIndex, filePath)
	
	coll, err := db.ConnectToDB()
	if err != nil {
		return fmt.Errorf("DB connection failed: %v", err)
	}
	
	err = db.DeletePostByPath(coll, collectionName, url)
	if err != nil {
		log.Printf("DEBUG: Delete failed: %v", err)
		return err
	}
	
	log.Printf("Successfully deleted post: %s/%s", collectionName, url)
	return nil
}

// broadcastChange sends reload notification to all connected SSE clients
func broadcastChange(message string) {
	clientsMux.Lock()
	defer clientsMux.Unlock()
	
	log.Printf("DEBUG: Broadcasting '%s' to %d connected clients", message, len(clients))
	
	for client := range clients {
		select {
		case client <- message:
			log.Printf("DEBUG: Sent '%s' to client", message)
		default:
			log.Printf("DEBUG: Client blocked, skipping")
		}
	}
}

// watchContentDirectory monitors content directory for changes and auto-syncs
func watchContentDirectory() {
	log.Printf("File watcher started for: %s", syncDir)
	
	// Track file hashes to detect actual content changes (not just timestamps)
	fileHashes := make(map[string]string)
	// Track existing collections (directories)
	collectionDirs := make(map[string]bool)
	
	// Initial scan
	filepath.Walk(syncDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		
		// Track directories (collections)
		if info.IsDir() && path != syncDir {
			relPath, _ := filepath.Rel(syncDir, path)
			parts := strings.Split(relPath, string(filepath.Separator))
			if len(parts) >= 1 {
				collectionDirs[parts[0]] = true
			}
		}
		
		// Track .md files
		if !info.IsDir() && strings.HasSuffix(strings.ToLower(path), ".md") {
			if hash, err := hashFile(path); err == nil {
				fileHashes[path] = hash
			}
		}
		return nil
	})
	
	log.Printf("DEBUG: Initial collections tracked: %v", collectionDirs)
	
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	
	for range ticker.C {
		changed := false
		currentCollections := make(map[string]bool)
		
		// Scan directory for changes
		filepath.Walk(syncDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			
			// Track current collections
			if info.IsDir() && path != syncDir {
				relPath, _ := filepath.Rel(syncDir, path)
				parts := strings.Split(relPath, string(filepath.Separator))
				if len(parts) >= 1 {
					currentCollections[parts[0]] = true
				}
			}
			
			// Track .md files
			if info.IsDir() {
				return nil
			}
			
			if !strings.HasSuffix(strings.ToLower(path), ".md") {
				return nil
			}
			
			currentHash, hashErr := hashFile(path)
			if hashErr != nil {
				return nil
			}
			
			previousHash, exists := fileHashes[path]
			if !exists || previousHash != currentHash {
				changed = true
				fileHashes[path] = currentHash
				log.Printf("Detected change in: %s", path)
			}
			
			return nil
		})
		
		// Check for deleted collections (directories)
		for collectionName := range collectionDirs {
			if !currentCollections[collectionName] {
				// Prefix with "content/" to match collection naming in MongoDB
				fullCollectionName := "content/" + collectionName
				log.Printf("Detected deleted collection: %s", fullCollectionName)
				
				// Delete entire collection from MongoDB
				coll, err := db.ConnectToDB()
				if err != nil {
					log.Printf("Failed to connect to DB: %v", err)
				} else {
					if err := db.DeleteCollection(coll, fullCollectionName); err != nil {
						log.Printf("Failed to delete collection '%s': %v", fullCollectionName, err)
					} else {
						log.Printf("Successfully deleted collection: %s", fullCollectionName)
						changed = true
					}
				}
			}
		}
		
		// Update tracked collections
		collectionDirs = currentCollections
		
		// Remove deleted files from tracking and delete from DB
		for path := range fileHashes {
			if _, err := os.Stat(path); os.IsNotExist(err) {
				delete(fileHashes, path)
				log.Printf("Detected deletion: %s", path)
				
				// Delete from database
				if err := deletePostFromDB(path); err != nil {
					log.Printf("Failed to delete post from DB: %v", err)
				}
				
				changed = true
			}
		}
		
		if changed {
			log.Printf("Changes detected, triggering auto-sync...")
			if err := autoSyncFromContent(); err != nil {
				log.Printf("Auto-sync error: %v", err)
			} else {
				log.Printf("Auto-sync completed successfully")
				broadcastChange("reload") // Notify browser to reload
			}
		}
	}
}

// hashFile computes MD5 hash of file content
func hashFile(path string) (string, error) {
	content, err := ioutil.ReadFile(path)
	if err != nil {
		return "", err
	}
	hash := md5.Sum(content)
	return hex.EncodeToString(hash[:]), nil
}

// sseHandler handles Server-Sent Events for auto-refresh
func sseHandler(w http.ResponseWriter, r *http.Request) {
	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	
	// Create channel for this client
	messageChan := make(chan string, 10)
	
	// Register client
	clientsMux.Lock()
	clients[messageChan] = true
	clientsMux.Unlock()
	
	// Unregister on disconnect
	defer func() {
		clientsMux.Lock()
		delete(clients, messageChan)
		close(messageChan)
		clientsMux.Unlock()
	}()
	
	// Send initial connection message
	fmt.Fprintf(w, "data: connected\n\n")
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	
	// Listen for messages or client disconnect
	for {
		select {
		case msg := <-messageChan:
			log.Printf("DEBUG: Sending SSE message to client: %s", msg)
			fmt.Fprintf(w, "data: %s\n\n", msg)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
				log.Printf("DEBUG: SSE message flushed to client")
			}
		case <-r.Context().Done():
			log.Printf("DEBUG: SSE client disconnected")
			return
		}
	}
}

// securityHeadersMiddleware adds security headers to all responses
func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip adding security headers for SSE endpoint (has its own headers)
		if r.URL.Path != "/api/events" {
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "SAMEORIGIN") // Allow iframe from same origin
			w.Header().Set("X-XSS-Protection", "1; mode=block")
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		}
		next.ServeHTTP(w, r)
	})
}

// plantUMLProxyHandler proxies requests to PlantUML server to avoid CORS/CSRF issues
func plantUMLProxyHandler(w http.ResponseWriter, r *http.Request) {
	// Get the diagram code from URL path: /plantuml/png/{encoded}
	parts := strings.Split(r.URL.Path, "/plantuml/")
	if len(parts) < 2 {
		http.Error(w, "Invalid PlantUML path", http.StatusBadRequest)
		return
	}
	
	// Forward to internal PlantUML server
	plantUMLServer := os.Getenv("PLANTUML_SERVER")
	if plantUMLServer == "" {
		plantUMLServer = "http://plantuml:8080"
	}
	
	targetURL := plantUMLServer + "/" + parts[1]
	resp, err := http.Get(targetURL)
	if err != nil {
		http.Error(w, "Failed to fetch diagram", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	
	// Copy headers
	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.Header().Set("Cache-Control", "public, max-age=86400") // Cache diagrams for 1 day
	
	// Copy body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "Failed to read diagram", http.StatusInternalServerError)
		return
	}
	
	w.Write(body)
}

func configureRouter(r *mux.Router) {
	r.HandleFunc("/", indexHandler)
	r.HandleFunc("/post/{name}", mdNamedHandler)
	r.HandleFunc("/add", addHandler)
	r.HandleFunc("/collections", collectionsHandler)
	// REMOVED: /collection/{collection} - replaced by /content/{collection...}
	r.HandleFunc("/content/{collection:.*}", collectionContentHandler) // Match everything after /content/
	
	// Collection management endpoints
	r.HandleFunc("/api/collection/create", createCollectionHandler).Methods("POST")
	r.HandleFunc("/api/collection/{name:.*}/delete", deleteCollectionHandler).Methods("DELETE")
	r.HandleFunc("/api/collection/{name:.*}/upload", uploadFilesHandler).Methods("POST")
	r.HandleFunc("/api/collection/{name:.*}", getCollectionPostsHandler).Methods("GET")
	
	// File sync endpoint
	r.HandleFunc("/api/sync", syncDirectoryHandler).Methods("POST", "GET")
	
	// SSE endpoint for auto-refresh
	r.HandleFunc("/api/events", sseHandler)
	
	// PlantUML proxy to avoid CSRF warnings
	r.PathPrefix("/plantuml/").HandlerFunc(plantUMLProxyHandler)
	
	// Add security headers middleware
	r.Use(securityHeadersMiddleware)
}

func errorNotFoundPage(w http.ResponseWriter) {
	text := `# Error 404
This page not found`
	tmpl := template.Must(template.ParseFiles("content.html"))
	output := template.HTML(string(blackfriday.Run([]byte(text))))
	tmpl.ExecuteTemplate(w, "content", output)
}

func internalServerErrorPage(err error, w http.ResponseWriter) {
	log.Panic(err)
	text := `# Error 500
Internal server error`
	tmpl := template.Must(template.ParseFiles("content.html"))
	output := template.HTML(string(blackfriday.Run([]byte(text))))
	tmpl.ExecuteTemplate(w, "content", output)
}
