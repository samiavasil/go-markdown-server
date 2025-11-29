package plantuml

import (
	"bytes"
	"compress/flate"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var plantUMLServerURL string
var plantUMLPublicURL string

func init() {
	plantUMLServerURL = os.Getenv("PLANTUML_SERVER")
	if plantUMLServerURL == "" {
		plantUMLServerURL = "http://plantuml:8080"
	}
	
	plantUMLPublicURL = os.Getenv("PLANTUML_PUBLIC_URL")
	if plantUMLPublicURL == "" {
		// Use relative URL through proxy instead of direct localhost:8081
		plantUMLPublicURL = "/plantuml"
	}
	
	log.Printf("PlantUML server URL (internal): %s", plantUMLServerURL)
	log.Printf("PlantUML public URL (browser): %s", plantUMLPublicURL)
}

// ProcessPlantUML finds PlantUML blocks in markdown and replaces them with rendered images
func ProcessPlantUML(markdown string) (string, error) {
	// Regex to find ```plantuml code blocks
	re := regexp.MustCompile("(?s)```plantuml\\s*\\n(.*?)\\n```")
	
	result := re.ReplaceAllStringFunc(markdown, func(match string) string {
		// Extract the PlantUML code
		codeMatch := re.FindStringSubmatch(match)
		if len(codeMatch) < 2 {
			return match
		}
		
		plantUMLCode := strings.TrimSpace(codeMatch[1])
		
		// Generate diagram using PlantUML server
		imageURL, err := generateDiagram(plantUMLCode)
		if err != nil {
			// If error, return original code block
			return match
		}
		
		// Replace with markdown image syntax
		return fmt.Sprintf("![PlantUML Diagram](%s)", imageURL)
	})
	
	return result, nil
}

// generateDiagram sends PlantUML code to server and returns image URL
func generateDiagram(plantUMLCode string) (string, error) {
	// Encode PlantUML code
	encoded := encodePlantUML(plantUMLCode)
	
	// Verify the diagram can be generated (use internal URL)
	validationURL := fmt.Sprintf("%s/png/%s", plantUMLServerURL, encoded)
	resp, err := http.Get(validationURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return "", fmt.Errorf("PlantUML server error: %s", string(body))
	}
	
	// Return public URL for browser
	diagramURL := fmt.Sprintf("%s/png/%s", plantUMLPublicURL, encoded)
	return diagramURL, nil
}

// encodePlantUML encodes PlantUML code using PlantUML text encoding
func encodePlantUML(code string) string {
	// Ensure code has @startuml and @enduml
	if !strings.Contains(code, "@startuml") {
		code = "@startuml\n" + code
	}
	if !strings.Contains(code, "@enduml") {
		code = code + "\n@enduml"
	}
	
	// Compress and encode
	compressed := deflate([]byte(code))
	encoded := encode64(compressed)
	
	return encoded
}

// deflate compresses data (simplified version)
func deflate(data []byte) []byte {
	// For simplicity, we'll use base64 encoding directly
	// In production, you'd want proper deflate compression
	return data
}

// encode64 uses PlantUML's custom base64-like encoding
func encode64(data []byte) string {
	// PlantUML uses a custom base64 alphabet
	alphabet := "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz-_"
	
	encoded := base64.StdEncoding.EncodeToString(data)
	
	// Convert standard base64 to PlantUML encoding
	var result strings.Builder
	for _, ch := range encoded {
		switch ch {
		case '+':
			result.WriteString("-")
		case '/':
			result.WriteString("_")
		case '=':
			// Skip padding
		default:
			// Map standard base64 to PlantUML alphabet
			if ch >= 'A' && ch <= 'Z' {
				result.WriteByte(alphabet[int(ch-'A')+10])
			} else if ch >= 'a' && ch <= 'z' {
				result.WriteByte(alphabet[int(ch-'a')+36])
			} else if ch >= '0' && ch <= '9' {
				result.WriteByte(alphabet[int(ch-'0')])
			}
		}
	}
	
	return result.String()
}

// Alternative simpler approach: use PlantUML proxy encoding
func GeneratePlantUMLImageURL(plantUMLCode string) string {
	// Ensure proper formatting
	if !strings.Contains(plantUMLCode, "@startuml") {
		plantUMLCode = "@startuml\n" + plantUMLCode + "\n@enduml"
	}
	
	// Use PlantUML's text/plain endpoint for simpler integration
	encoded := base64.StdEncoding.EncodeToString([]byte(plantUMLCode))
	return fmt.Sprintf("%s/png/%s", plantUMLPublicURL, encoded)
}

// RenderPlantUMLToImage fetches the actual image data
func RenderPlantUMLToImage(plantUMLCode string) ([]byte, error) {
	// Ensure @startuml/@enduml
	if !strings.Contains(plantUMLCode, "@startuml") {
		plantUMLCode = "@startuml\n" + plantUMLCode + "\n@enduml"
	}
	
	// Use PlantUML's text encoding
	encoded := base64.StdEncoding.EncodeToString([]byte(plantUMLCode))
	
	// Use internal URL for fetching
	url := fmt.Sprintf("%s/png/%s", plantUMLServerURL, encoded)
	
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to generate diagram: HTTP %d", resp.StatusCode)
	}
	
	return ioutil.ReadAll(resp.Body)
}

// ProcessPlantUMLSimple processes both ```plantuml code blocks and ![](*.puml) file references
func ProcessPlantUMLSimple(markdown string) string {
	// First, process ```plantuml code blocks
	reCodeBlock := regexp.MustCompile("(?s)```plantuml\\s*\\n(.*?)\\n```")
	
	log.Printf("Processing markdown for PlantUML blocks (length: %d)", len(markdown))
	
	result := reCodeBlock.ReplaceAllStringFunc(markdown, func(match string) string {
		log.Printf("Found PlantUML block: %s", match[:50])
		codeMatch := reCodeBlock.FindStringSubmatch(match)
		if len(codeMatch) < 2 {
			return match
		}
		
		plantUMLCode := strings.TrimSpace(codeMatch[1])
		
		// Add @startuml/@enduml if missing
		var buf bytes.Buffer
		if !strings.Contains(plantUMLCode, "@startuml") {
			buf.WriteString("@startuml\n")
			// Add skinparam for smaller fonts
			buf.WriteString("skinparam defaultFontSize 11\n")
			buf.WriteString("skinparam defaultFontName Arial\n")
			buf.WriteString("skinparam ArrowFontSize 10\n")
			buf.WriteString("skinparam ClassFontSize 11\n")
			buf.WriteString("skinparam NoteFontSize 10\n")
		}
		buf.WriteString(plantUMLCode)
		if !strings.Contains(plantUMLCode, "@enduml") {
			buf.WriteString("\n@enduml")
		}
		
		// Use PlantUML text encoding (deflate + custom base64)
		encoded := encodePlantUMLText(buf.String())
		imageURL := fmt.Sprintf("%s/png/%s", plantUMLPublicURL, encoded)
		
		log.Printf("Generated PlantUML URL: %s", imageURL)
		return fmt.Sprintf("![PlantUML Diagram](%s)", imageURL)
	})
	
	// Second, process ![title](path/to/file.puml) image references
	rePumlFile := regexp.MustCompile(`!\[([^\]]*)\]\(([^\)]+\.puml)\)`)
	
	result = rePumlFile.ReplaceAllStringFunc(result, func(match string) string {
		fileMatch := rePumlFile.FindStringSubmatch(match)
		if len(fileMatch) < 3 {
			return match
		}
		
		title := fileMatch[1]
		pumlPath := fileMatch[2]
		
		// Try to read the .puml file (no baseDir in simple version)
		plantUMLCode, err := readPumlFile(pumlPath, "")
		if err != nil {
			// If can't read, try replacing .puml with .png
			pngPath := strings.Replace(pumlPath, ".puml", ".png", 1)
			return fmt.Sprintf("![%s](%s)", title, pngPath)
		}
		
		// Add skinparam settings if not already present
		if !strings.Contains(plantUMLCode, "skinparam defaultFontSize") {
			var buf bytes.Buffer
			if strings.Contains(plantUMLCode, "@startuml") {
				// Insert after @startuml
				lines := strings.Split(plantUMLCode, "\n")
				buf.WriteString(lines[0] + "\n")
				buf.WriteString("skinparam defaultFontSize 11\n")
				buf.WriteString("skinparam defaultFontName Arial\n")
				buf.WriteString("skinparam ArrowFontSize 10\n")
				buf.WriteString("skinparam ClassFontSize 11\n")
				buf.WriteString("skinparam NoteFontSize 10\n")
				buf.WriteString(strings.Join(lines[1:], "\n"))
				plantUMLCode = buf.String()
			}
		}
		
		// Encode and generate URL
		encoded := encodePlantUMLText(plantUMLCode)
		imageURL := fmt.Sprintf("%s/png/%s", plantUMLPublicURL, encoded)
		
		return fmt.Sprintf("![%s](%s)", title, imageURL)
	})
	
	return result
}

// readPumlFile reads a PlantUML file from disk, searching relative to baseDir
func readPumlFile(path string, baseDir string) (string, error) {
	log.Printf("DEBUG: readPumlFile called with path: %s, baseDir: %s", path, baseDir)
	
	// Build candidate paths based on baseDir
	candidates := []string{}
	
	// If baseDir provided, prioritize paths relative to it
	if baseDir != "" {
		// Direct: baseDir/path
		candidates = append(candidates, filepath.Join("content", baseDir, path))
		// With diagrams: baseDir/diagrams/basename
		candidates = append(candidates, filepath.Join("content", baseDir, "diagrams", filepath.Base(path)))
	}
	
	// Fallback: direct path from content root
	candidates = append(candidates, filepath.Join("content", path))
	
	// Last resort: path as-is (for absolute or current-dir references)
	candidates = append(candidates, path)
	
	for _, fullPath := range candidates {
		log.Printf("DEBUG: Trying to read: %s", fullPath)
		content, err := ioutil.ReadFile(fullPath)
		if err == nil {
			log.Printf("DEBUG: Successfully read .puml file from: %s", fullPath)
			contentStr := string(content)
			
			// If file contains ```plantuml fence, extract content
			if strings.HasPrefix(contentStr, "```plantuml") {
				lines := strings.Split(contentStr, "\n")
				if len(lines) > 2 {
					// Remove first line (```plantuml) and last line (```)
					contentStr = strings.Join(lines[1:len(lines)-1], "\n")
				}
			}
			
			return contentStr, nil
		}
	}
	
	// If not found in any candidate path, return error
	log.Printf("DEBUG: .puml file not found: %s (tried %d paths)", path, len(candidates))
	return "", fmt.Errorf("puml file not found: %s", path)
}

// ProcessPlantUMLWithBase allows resolving .puml relative to a base directory (collection)
func ProcessPlantUMLWithBase(markdown string, baseDir string) string {
	log.Printf("DEBUG: ProcessPlantUMLWithBase called (length: %d, baseDir: '%s')", len(markdown), baseDir)
	
	// First, process ```plantuml code blocks (same as ProcessPlantUMLSimple)
	reCodeBlock := regexp.MustCompile("(?s)```plantuml\\s*\\n(.*?)\\n```")
	result := reCodeBlock.ReplaceAllStringFunc(markdown, func(match string) string {
		codeMatch := reCodeBlock.FindStringSubmatch(match)
		if len(codeMatch) < 2 {
			return match
		}
		plantUMLCode := strings.TrimSpace(codeMatch[1])
		var buf bytes.Buffer
		if !strings.Contains(plantUMLCode, "@startuml") {
			buf.WriteString("@startuml\n")
			buf.WriteString("skinparam defaultFontSize 11\n")
			buf.WriteString("skinparam defaultFontName Arial\n")
			buf.WriteString("skinparam ArrowFontSize 10\n")
			buf.WriteString("skinparam ClassFontSize 11\n")
			buf.WriteString("skinparam NoteFontSize 10\n")
		}
		buf.WriteString(plantUMLCode)
		if !strings.Contains(plantUMLCode, "@enduml") {
			buf.WriteString("\n@enduml")
		}
		encoded := encodePlantUMLText(buf.String())
		imageURL := fmt.Sprintf("%s/png/%s", plantUMLPublicURL, encoded)
		return fmt.Sprintf("![PlantUML Diagram](%s)", imageURL)
	})

	// Then, resolve puml file references relative to baseDir first
	rePumlFile := regexp.MustCompile(`!\[([^\]]*)\]\(([^\)]+\.puml)\)`)
	
	log.Printf("DEBUG: Searching for .puml references in markdown (length: %d, baseDir: %s)", len(result), baseDir)
	
	result = rePumlFile.ReplaceAllStringFunc(result, func(match string) string {
		log.Printf("DEBUG: Found .puml reference: %s", match)
		fileMatch := rePumlFile.FindStringSubmatch(match)
		if len(fileMatch) < 3 {
			return match
		}
		title := fileMatch[1]
		pumlPath := fileMatch[2]

		// Use readPumlFile with baseDir - it handles all path resolution
		plantUMLCode, err := readPumlFile(pumlPath, baseDir)
		if err != nil {
			// Fallback: try .png replacement
			pngPath := strings.Replace(pumlPath, ".puml", ".png", 1)
			return fmt.Sprintf("![%s](%s)", title, pngPath)
		}

		if !strings.Contains(plantUMLCode, "skinparam defaultFontSize") {
			if strings.Contains(plantUMLCode, "@startuml") {
				lines := strings.Split(plantUMLCode, "\n")
				var buf2 bytes.Buffer
				buf2.WriteString(lines[0] + "\n")
				buf2.WriteString("skinparam defaultFontSize 11\n")
				buf2.WriteString("skinparam defaultFontName Arial\n")
				buf2.WriteString("skinparam ArrowFontSize 10\n")
				buf2.WriteString("skinparam ClassFontSize 11\n")
				buf2.WriteString("skinparam NoteFontSize 10\n")
				buf2.WriteString(strings.Join(lines[1:], "\n"))
				plantUMLCode = buf2.String()
			}
		}

		encoded := encodePlantUMLText(plantUMLCode)
		imageURL := fmt.Sprintf("%s/png/%s", plantUMLPublicURL, encoded)
		return fmt.Sprintf("![%s](%s)", title, imageURL)
	})

	return result
}

// encodePlantUMLText encodes text using PlantUML's deflate + base64 encoding
func encodePlantUMLText(text string) string {
	// Deflate compression
	var compressed bytes.Buffer
	w, _ := flate.NewWriter(&compressed, flate.DefaultCompression)
	w.Write([]byte(text))
	w.Close()
	
	// PlantUML uses custom base64 alphabet
	return encode64PlantUML(compressed.Bytes())
}

// encode64PlantUML encodes bytes using PlantUML's custom base64 alphabet
func encode64PlantUML(data []byte) string {
	alphabet := "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz-_"
	
	var result strings.Builder
	var bits uint32
	var bitsLen uint
	
	for _, b := range data {
		bits = (bits << 8) | uint32(b)
		bitsLen += 8
		
		for bitsLen >= 6 {
			bitsLen -= 6
			idx := (bits >> bitsLen) & 0x3F
			result.WriteByte(alphabet[idx])
		}
	}
	
	// Handle remaining bits
	if bitsLen > 0 {
		bits <<= (6 - bitsLen)
		idx := bits & 0x3F
		result.WriteByte(alphabet[idx])
	}
	
	return result.String()
}
