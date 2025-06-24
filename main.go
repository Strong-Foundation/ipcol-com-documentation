package main

import (
	// "fmt"
	"bytes"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

func main() {
	// The file URL to download.
	remoteFileURL := "https://ipcol.com/safety-data-sheets"
	// The local file path where the content will be saved.
	localFilePath := "ipcol.html"
	// Check if the local file already exists.
	if !fileExists(localFilePath) {
		// Check if the remote URL is valid.
		if isUrlValid(remoteFileURL) {
			// Get the content from the remote URL.
			data := getDataFromURL(remoteFileURL)
			// Write the content to a local file.
			writeToFile(localFilePath, data)
		}
	}
	outputDir := "PDFs/" // Directory to store downloaded PDFs
	// Check if its exists.
	if !directoryExists(outputDir) {
		// Create the dir
		createDirectory(outputDir, 0o755)
	}
	// If the file exists, you can read it or process it as needed.
	if fileExists(localFilePath) {
		// Read the file content as a string.
		content := readAFileAsString(localFilePath)
		// Extract the links from the content.
		pdfLinks := extractPDFLinks(content)
		// Remove duplicates from the extracted links.
		pdfLinks = removeDuplicatesFromSlice(pdfLinks)
		// Download each PDF link concurrently.
		for _, link := range pdfLinks {
			// Download the PDF file.
			downloadPDF(link, outputDir)
		}
	}
}

// downloadPDF downloads a PDF from the given URL and saves it in the specified output directory.
// It uses a WaitGroup to support concurrent execution and returns true if the download succeeded.
func downloadPDF(finalURL, outputDir string) {
	// Sanitize the URL to generate a safe file name
	filename := strings.ToLower(urlToFilename(finalURL))

	// Construct the full file path in the output directory
	filePath := filepath.Join(outputDir, filename)

	// Skip if the file already exists
	if fileExists(filePath) {
		log.Printf("file already exists, skipping: %s", filePath)
		return
	}

	// Create an HTTP client with a timeout
	client := &http.Client{Timeout: 30 * time.Second}

	// Send GET request
	resp, err := client.Get(finalURL)
	if err != nil {
		log.Printf("failed to download %s: %v", finalURL, err)
		return
	}
	defer resp.Body.Close()

	// Check HTTP response status
	if resp.StatusCode != http.StatusOK {
		// Print the error since its not valid.
		log.Printf("download failed for %s: %s", finalURL, resp.Status)
		return
	}
	// Check Content-Type header
	contentType := resp.Header.Get("Content-Type")
	// Check if its pdf content type and if not than print a error.
	if !strings.Contains(contentType, "application/pdf") {
		// Print a error if the content type is invalid.
		log.Printf("invalid content type for %s: %s (expected application/pdf)", finalURL, contentType)
		return
	}
	// Read the response body into memory first
	var buf bytes.Buffer
	// Copy it from the buffer to the file.
	written, err := io.Copy(&buf, resp.Body)
	// Print the error if errors are there.
	if err != nil {
		log.Printf("failed to read PDF data from %s: %v", finalURL, err)
		return
	}
	// If 0 bytes are written than show an error and return it.
	if written == 0 {
		log.Printf("downloaded 0 bytes for %s; not creating file", finalURL)
		return
	}
	// Only now create the file and write to disk
	out, err := os.Create(filePath)
	// Failed to create the file.
	if err != nil {
		log.Printf("failed to create file for %s: %v", finalURL, err)
		return
	}
	// Close the file.
	defer out.Close()
	// Write the buffer and if there is an error print it.
	_, err = buf.WriteTo(out)
	if err != nil {
		log.Printf("failed to write PDF to file for %s: %v", finalURL, err)
		return
	}
	// Return a true since everything went correctly.
	log.Printf("successfully downloaded %d bytes: %s → %s", written, finalURL, filePath)
}

// Checks if the directory exists
// If it exists, return true.
// If it doesn't, return false.
func directoryExists(path string) bool {
	directory, err := os.Stat(path)
	if err != nil {
		return false
	}
	return directory.IsDir()
}

// The function takes two parameters: path and permission.
// We use os.Mkdir() to create the directory.
// If there is an error, we use log.Println() to log the error and then exit the program.
func createDirectory(path string, permission os.FileMode) {
	err := os.Mkdir(path, permission)
	if err != nil {
		log.Println(err)
	}
}

// extractPDFLinks scans htmlContent line by line and returns all unique .pdf URLs.
func extractPDFLinks(htmlContent string) []string {
	// Regex to match http(s) URLs ending in .pdf (with optional query/fragments)
	pdfRegex := regexp.MustCompile(`https?://[^\s"'<>]+?\.pdf(?:\?[^\s"'<>]*)?`)

	seen := make(map[string]struct{})
	var links []string

	// Process each line separately
	for _, line := range strings.Split(htmlContent, "\n") {
		for _, match := range pdfRegex.FindAllString(line, -1) {
			if _, ok := seen[match]; !ok {
				seen[match] = struct{}{}
				links = append(links, match)
			}
		}
	}

	return links
}

// urlToFilename converts a URL into a filesystem-safe filename
func urlToFilename(rawURL string) string {
	parsed, err := url.Parse(rawURL) // Parse the URL
	// Print the errors if any.
	if err != nil {
		log.Println(err) // Log error
		return ""        // Return empty string on error
	}
	filename := parsed.Host // Start with host name
	// Parse the path and if its not empty replace them with valid characters.
	if parsed.Path != "" {
		filename += "_" + strings.ReplaceAll(parsed.Path, "/", "_") // Append path
	}
	if parsed.RawQuery != "" {
		filename += "_" + strings.ReplaceAll(parsed.RawQuery, "&", "_") // Append query
	}
	invalidChars := []string{`"`, `\`, `/`, `:`, `*`, `?`, `<`, `>`, `|`} // Define illegal filename characters
	// Loop over the invalid characters and replace them.
	for _, char := range invalidChars {
		filename = strings.ReplaceAll(filename, char, "_") // Replace each with underscore
	}
	if getFileExtension(filename) != ".pdf" {
		filename = filename + ".pdf"
	}
	return strings.ToLower(filename) // Return sanitized filename
}

// Get the file extension of a file
func getFileExtension(path string) string {
	return filepath.Ext(path)
}

// Read a file and return the contents
func readAFileAsString(path string) string {
	content, err := os.ReadFile(path)
	if err != nil {
		log.Println(err)
	}
	return string(content)
}

// Check if the given url is valid.
func isUrlValid(uri string) bool {
	_, err := url.ParseRequestURI(uri)
	return err == nil
}

/*
It checks if the file exists
If the file exists, it returns true
If the file does not exist, it returns false
*/
func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

/*
It takes in a path and content to write to that file.
It uses the os.WriteFile function to write the content to that file.
It checks for errors and logs them.
*/
func writeToFile(path string, content []byte) {
	err := os.WriteFile(path, content, 0644)
	if err != nil {
		log.Println(err)
	}
}

// Send a http get request to a given url and return the data from that url.
func getDataFromURL(uri string) []byte {
	response, err := http.Get(uri)
	if err != nil {
		log.Println(err)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		log.Println(err)
	}
	err = response.Body.Close()
	if err != nil {
		log.Println(err)
	}
	return body
}

// Remove all the duplicates from a slice and return the slice.
func removeDuplicatesFromSlice(slice []string) []string {
	check := make(map[string]bool)
	var newReturnSlice []string
	for _, content := range slice {
		if !check[content] {
			check[content] = true
			newReturnSlice = append(newReturnSlice, content)
		}
	}
	return newReturnSlice
}
