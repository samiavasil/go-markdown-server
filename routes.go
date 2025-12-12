package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/beldmian/go-markdown-server/db"
	"github.com/beldmian/go-markdown-server/plantuml"
	"github.com/gorilla/mux"
	"github.com/russross/blackfriday"
)

func indexHandler(w http.ResponseWriter, r *http.Request) {
	// Root page should load the iframe layout (sidebar + iframe), not content
	// The iframe will load /content/Architecture (or first available collection)
	tmpl := template.Must(template.ParseFiles("md.html"))
	
	// Pass empty content - md.html will load collections via JavaScript
	output := template.HTML("")
	if err := tmpl.ExecuteTemplate(w, "md", output); err != nil {
		internalServerErrorPage(err, w)
	}
}

func mdNamedHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	post, err := db.GetPostByName(collection, vars["name"])
	if err != nil {
		errorNotFoundPage(w)
		return
	}
	
	// Process PlantUML blocks and convert cross-references
	processedBody := post.Body
	// For uploaded collections, use empty baseDir (inline blocks work, .puml files won't be found)
	// For content/ collections, use actual baseDir for .puml file resolution
	baseDir := ""
	if !strings.HasPrefix(post.Collection, "uploaded/") {
		baseDir = strings.TrimPrefix(post.Collection, "content/")
	}
	processedBody = plantuml.ProcessPlantUMLWithBase(processedBody, baseDir)
	processedBody = processCrossReferences(processedBody, post.Collection)
	
	// Use content.html (only content, no sidebar) for iframe display
	tmpl := template.Must(template.ParseFiles("content.html"))
	output := template.HTML(string(blackfriday.Run([]byte(processedBody))))
	tmpl.ExecuteTemplate(w, "content", output)
}

// collectionsHandler returns JSON list of all collections with autoSync flag
func collectionsHandler(w http.ResponseWriter, r *http.Request) {
	collections, err := db.GetCollections(collection)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	
	// Check which collections exist in content/ directory (auto-sync sources)
	type CollectionInfo struct {
		Name     string `json:"name"`
		AutoSync bool   `json:"autoSync"`
	}
	
	result := make([]CollectionInfo, 0, len(collections))
	for _, name := range collections {
		// Check if collection name starts with "content/" prefix (auto-sync source)
		autoSync := strings.HasPrefix(name, "content/")
		
		result = append(result, CollectionInfo{
			Name:     name,
			AutoSync: autoSync,
		})
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// collectionContentHandler renders ONLY the content (for iframe)
func collectionContentHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	collectionName := vars["collection"]
	
	log.Printf("DEBUG: collectionContentHandler called with: '%s'", collectionName)
	
	// Try to find index.md for this collection
	indexPost, err := db.GetIndexPost(collection, collectionName)
	
	var out string
	if err == nil && indexPost.IsIndex {
		// Found index.md - show it
		out = indexPost.Body
	} else {
		// No index.md - show list of all posts in collection
		posts, err := db.GetPostsByCollection(collection, collectionName)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		
		out = "# " + collectionName + "\n---\n"
		for _, post := range posts {
			out += "- [" + post.Title + "](/post/" + post.URL + ")\n"
		}
	}
	
	// Process PlantUML and cross-references (base = collection name without prefix)
	baseDir := ""
	if !strings.HasPrefix(collectionName, "uploaded/") {
		baseDir = strings.TrimPrefix(collectionName, "content/")
	}
	out = plantuml.ProcessPlantUMLWithBase(out, baseDir)
	out = processCrossReferences(out, collectionName)
	
	tmpl := template.Must(template.ParseFiles("content.html"))
	output := template.HTML(string(blackfriday.Run([]byte(out))))
	if err := tmpl.ExecuteTemplate(w, "content", output); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// processCrossReferences converts relative MD links to absolute URLs
// Converts: [text](./file.md) -> [text](/post/file)
// Converts: [text](file.md) -> [text](/post/file)
func processCrossReferences(body string, currentCollection string) string {
	// Simple regex replacement for markdown links
	// Pattern: [text](./something.md) or [text](something.md)
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		// Replace relative links
		line = strings.ReplaceAll(line, "](./", "](/post/")
		line = strings.ReplaceAll(line, "](.md)", "]")
		
		// Replace .md extension in links
		if strings.Contains(line, "](/post/") {
			line = strings.ReplaceAll(line, ".md)", ")")
		}
		lines[i] = line
	}
	return strings.Join(lines, "\n")
}

// getCollectionPostsHandler returns all posts for a collection as JSON
func getCollectionPostsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	collectionName := vars["name"]
	
	posts, err := db.GetPostsByCollection(collection, collectionName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(posts)
}

func addHandler(w http.ResponseWriter, r *http.Request) {
	v := r.URL.Query()
	
	collectionName := ""
	if c, ok := v["collection"]; ok && len(c) > 0 {
		collectionName = c[0]
	}
	
	isIndex := false
	if idx, ok := v["isIndex"]; ok && len(idx) > 0 && idx[0] == "true" {
		isIndex = true
	}
	
	post := db.Post{
		Title:      v["title"][0],
		URL:        v["url"][0],
		Body:       v["body"][0],
		Collection: collectionName,
		IsIndex:    isIndex,
	}
	_, err := db.InsertPost(collection, post, v["key"][0])
	if err != nil {
		w.Write([]byte(err.Error()))
		return
	}
	w.Write([]byte("success"))
	
	// Notify connected clients to reload
	broadcastChange("reload")
}

// createCollectionHandler creates a new collection and optionally uploads files
func createCollectionHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("=== createCollectionHandler called ===")
	
	if err := r.ParseMultipartForm(100 << 20); err != nil { // 100MB max
		log.Printf("Error parsing form: %v", err)
		http.Error(w, "Failed to parse form: "+err.Error(), http.StatusBadRequest)
		return
	}
	
	collectionName := r.FormValue("name")
	files := r.MultipartForm.File["files"]
	
	log.Printf("Collection name from form: '%s'", collectionName)
	log.Printf("Number of files: %d", len(files))
	
	// If no name provided, try to extract from first file's path
	if collectionName == "" && len(files) > 0 {
		firstFile := files[0]
		log.Printf("First file filename: '%s'", firstFile.Filename)
		// Filename contains the full path when uploaded via directory selector
		if strings.Contains(firstFile.Filename, "/") {
			// Extract folder name from path
			parts := strings.Split(firstFile.Filename, "/")
			if len(parts) > 1 {
				collectionName = parts[0]
				log.Printf("Extracted collection name from path: '%s'", collectionName)
			}
		}
	}
	
	if collectionName == "" {
		log.Println("Error: Collection name is empty")
		http.Error(w, "Collection name is required", http.StatusBadRequest)
		return
	}
	
	log.Printf("Final collection name: '%s'", collectionName)
	
	// Prefix uploaded collections with "uploaded/" to distinguish from content/ sources
	collectionName = "uploaded/" + collectionName
	log.Printf("Prefixed collection name: '%s'", collectionName)
	
	// Process uploaded files
	if len(files) == 0 {
		// Create empty collection with a placeholder index
		log.Println("No files uploaded, creating empty collection")
		post := db.Post{
			Title:      collectionName,
			URL:        collectionName + "-index",
			Body:       fmt.Sprintf("# %s\n\nNew collection created.", collectionName),
			Collection: collectionName,
			IsIndex:    true,
		}
		if _, err := db.InsertPost(collection, post, "124252"); err != nil {
			log.Printf("Error inserting empty collection: %v", err)
			http.Error(w, "Failed to create collection: "+err.Error(), http.StatusInternalServerError)
			return
		}
		log.Println("Empty collection created successfully")
	} else {
		// Upload all markdown files - they all go to the same collection
		log.Printf("Processing %d files...", len(files))
		for i, fileHeader := range files {
			log.Printf("Processing file %d/%d: %s", i+1, len(files), fileHeader.Filename)
			if err := processUploadedFileToCollection(fileHeader, collectionName); err != nil {
				log.Printf("Error processing file '%s': %v", fileHeader.Filename, err)
				http.Error(w, "Failed to process file "+fileHeader.Filename+": "+err.Error(), http.StatusInternalServerError)
				return
			}
		}
		log.Printf("All %d files processed successfully", len(files))
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success", "collection": collectionName})
	log.Printf("=== Collection '%s' created successfully ===", collectionName)
	
	// Notify connected clients to reload
	broadcastChange("reload")
}

// deleteCollectionHandler deletes a collection and all its posts
func deleteCollectionHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	collectionName := vars["name"]
	
	if collectionName == "" {
		http.Error(w, "Collection name is required", http.StatusBadRequest)
		return
	}
	
	if err := db.DeleteCollection(collection, collectionName); err != nil {
		http.Error(w, "Failed to delete collection: "+err.Error(), http.StatusInternalServerError)
		return
	}
	
	// Trigger auto-sync to re-import from content/ directory (read-only source)
	go autoSyncFromContent()
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
	
	// Notify connected clients to reload
	broadcastChange("reload")
}

// uploadFilesHandler uploads markdown files to an existing collection
func uploadFilesHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	collectionName := vars["name"]
	
	if err := r.ParseMultipartForm(100 << 20); err != nil { // 100MB max
		http.Error(w, "Failed to parse form: "+err.Error(), http.StatusBadRequest)
		return
	}
	
	replaceExisting := r.FormValue("replace") == "true"
	
	// If replacing, delete existing posts
	if replaceExisting {
		if err := db.DeleteCollection(collection, collectionName); err != nil {
			http.Error(w, "Failed to clear collection: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}
	
	// Process uploaded files
	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		http.Error(w, "No files uploaded", http.StatusBadRequest)
		return
	}
	
	uploadedCount := 0
	for _, fileHeader := range files {
		if err := processUploadedFileToCollection(fileHeader, collectionName); err != nil {
			http.Error(w, "Failed to process file: "+err.Error(), http.StatusInternalServerError)
			return
		}
		uploadedCount++
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"count":  uploadedCount,
	})
	
	// Notify connected clients to reload
	broadcastChange("reload")
}

// processUploadedFileToCollection processes file and puts it in specified collection
func processUploadedFileToCollection(fileHeader *multipart.FileHeader, collectionName string) error {
	log.Printf("  Processing file: %s (size: %d bytes)", fileHeader.Filename, fileHeader.Size)
	
	file, err := fileHeader.Open()
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()
	
	// Only process .md files
	filename := fileHeader.Filename
	
	// Extract filename from full path if present (webkitRelativePath)
	if strings.Contains(filename, "/") {
		parts := strings.Split(filename, "/")
		filename = parts[len(parts)-1]
		log.Printf("  Extracted filename from path: %s", filename)
	}
	
	if !strings.HasSuffix(strings.ToLower(filename), ".md") {
		log.Printf("  Skipping non-markdown file: %s", filename)
		return nil // Skip non-markdown files
	}
	
	// Read file content
	content, err := ioutil.ReadAll(file)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}
	
	log.Printf("  Read %d bytes from file", len(content))
	
	// Parse frontmatter or use filename as title
	title := strings.TrimSuffix(filename, ".md")
	url := strings.ToLower(strings.ReplaceAll(title, " ", "-"))
	url = strings.ReplaceAll(url, "_", "-")
	body := string(content)
	isIndex := false
	
	// Check if this is an index file
	lowerFilename := strings.ToLower(filename)
	if lowerFilename == "index.md" || lowerFilename == "readme.md" {
		isIndex = true
		url = collectionName + "-index"
		log.Printf("  Detected index file")
	}
	
	// Parse simple frontmatter if exists
	if strings.HasPrefix(body, "---") {
		parts := strings.SplitN(body, "---", 3)
		if len(parts) >= 3 {
			frontmatter := parts[1]
			body = strings.TrimSpace(parts[2])
			
			// Extract title from frontmatter
			for _, line := range strings.Split(frontmatter, "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "title:") {
					title = strings.TrimSpace(strings.TrimPrefix(line, "title:"))
					title = strings.Trim(title, "\"")
					log.Printf("  Extracted title from frontmatter: %s", title)
					break
				}
			}
		}
	}
	
	// Process PlantUML diagrams and cross-references (use base-aware processor)
	body = plantuml.ProcessPlantUMLWithBase(body, collectionName)
	body = processCrossReferences(body, collectionName)
	
	// Create post
	post := db.Post{
		Title:      title,
		URL:        url,
		Body:       body,
		Collection: collectionName,
		IsIndex:    isIndex,
	}
	
	log.Printf("  Creating post: title='%s', url='%s', collection='%s', isIndex=%v", title, url, collectionName, isIndex)
	
	_, err = db.InsertPost(collection, post, "124252")
	if err != nil {
		return fmt.Errorf("failed to insert post: %w", err)
	}
	
	log.Printf("  ✓ Successfully inserted post '%s'", title)
	return nil
}

// syncDirectoryHandler syncs markdown files from content/ directory
func syncDirectoryHandler(w http.ResponseWriter, r *http.Request) {
	// Trigger auto-sync from content/ directory
	if err := autoSyncFromContent(); err != nil {
		http.Error(w, "Sync failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "Files synced successfully from content/",
	})
	
	// Notify connected clients to reload
	broadcastChange("reload")
}

