package filesync

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/beldmian/go-markdown-server/db"
	"github.com/beldmian/go-markdown-server/plantuml"
	"go.mongodb.org/mongo-driver/mongo"
)

// SyncConfig holds configuration for file sync
type SyncConfig struct {
	RootDir    string
	Collection *mongo.Collection
}

// SyncAllFiles recursively scans directory and imports all .md files
func SyncAllFiles(config SyncConfig) error {
	fmt.Printf("Starting sync from directory: %s\n", config.RootDir)
	
	// Check if directory exists
	if _, err := os.Stat(config.RootDir); os.IsNotExist(err) {
		return fmt.Errorf("directory does not exist: %s", config.RootDir)
	}
	
	fileCount := 0
	
	err := filepath.Walk(config.RootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		// Skip directories
		if info.IsDir() {
			return nil
		}
		
		// Only process .md files
		if !strings.HasSuffix(strings.ToLower(info.Name()), ".md") {
			return nil
		}
		
		// Import the file
		if err := importMarkdownFile(path, config.RootDir, config.Collection); err != nil {
			fmt.Printf("Error importing %s: %v\n", path, err)
			return nil // Continue with other files
		}
		
		fileCount++
		fmt.Printf("✓ Imported: %s\n", path)
		
		return nil
	})
	
	if err != nil {
		return err
	}
	
	fmt.Printf("Sync complete! Imported %d files.\n", fileCount)
	return nil
}

// importMarkdownFile reads a markdown file and creates a post in the database
func importMarkdownFile(filePath string, rootDir string, collection *mongo.Collection) error {
	// Read file content
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		return err
	}
	
	// Get relative path from root
	relPath, err := filepath.Rel(rootDir, filePath)
	if err != nil {
		relPath = filepath.Base(filePath)
	}
	
	// Determine collection name from directory structure
	collectionName := extractCollectionName(relPath, rootDir)
	
	// Parse file for title and body
	title, body, isIndex := parseMarkdownFile(string(content), filepath.Base(filePath))
	
	// Process PlantUML diagrams with collection-aware base directory
	body = plantuml.ProcessPlantUMLWithBase(body, collectionName)
	
	// Create URL-friendly slug
	url := createSlug(title, relPath)
	
	// Create post
	post := db.Post{
		Title:      title,
		URL:        url,
		Body:       body,
		Collection: collectionName,
		IsIndex:    isIndex,
	}
	
	// Upsert (update or insert) into database
	err = db.UpsertPost(collection, post, "124252")
	return err
}

// extractCollectionName determines collection name from file path
func extractCollectionName(relPath string, rootDir string) string {
	// Get first directory in relative path
	parts := strings.Split(filepath.ToSlash(relPath), "/")
	
	if len(parts) > 1 {
		// Use first subdirectory as collection name
		return parts[0]
	}
	
	// If file is in root, use root directory name
	return filepath.Base(rootDir)
}

// parseMarkdownFile extracts title and body from markdown content
func parseMarkdownFile(content string, filename string) (title string, body string, isIndex bool) {
	body = content
	title = strings.TrimSuffix(filename, ".md")
	isIndex = false
	
	// Check if this is an index file
	lowerFilename := strings.ToLower(filename)
	if lowerFilename == "index.md" || lowerFilename == "readme.md" {
		isIndex = true
	}
	
	// Parse frontmatter if exists
	if strings.HasPrefix(content, "---") {
		parts := strings.SplitN(content, "---", 3)
		if len(parts) >= 3 {
			frontmatter := parts[1]
			body = strings.TrimSpace(parts[2])
			
			// Extract title from frontmatter
			for _, line := range strings.Split(frontmatter, "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "title:") {
					title = strings.TrimSpace(strings.TrimPrefix(line, "title:"))
					title = strings.Trim(title, "\"'")
					break
				}
			}
		}
	}
	
	// If no title from frontmatter, try to extract from first H1
	if title == strings.TrimSuffix(filename, ".md") {
		lines := strings.Split(body, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "# ") {
				title = strings.TrimPrefix(line, "# ")
				break
			}
		}
	}
	
	return title, body, isIndex
}

// createSlug creates a URL-friendly slug from title and path
func createSlug(title string, relPath string) string {
	// Use relative path without extension for more stable URLs
	slug := strings.TrimSuffix(relPath, ".md")
	slug = filepath.ToSlash(slug)
	
	// Replace spaces and special characters
	slug = strings.ToLower(slug)
	slug = strings.ReplaceAll(slug, " ", "-")
	slug = strings.ReplaceAll(slug, "/", "-")
	
	// Remove multiple consecutive dashes
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}
	
	return slug
}

// ClearCollection removes all posts from database before sync
func ClearCollection(collection *mongo.Collection, collectionName string) error {
	if collectionName == "" {
		// Clear all collections
		return db.DeleteCollection(collection, collectionName)
	}
	return db.DeleteCollection(collection, collectionName)
}
