package browser

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/s3bw/vfs"
)

// getFileIcon returns the appropriate Nerd Font icon for a file
func getFileIcon(name string) string {
	if strings.HasSuffix(name, ".do") {
		return IconText
	}
	return IconFile
}

// getColorCode returns ANSI color code for a color name
func getColorCode(color string) string {
	switch color {
	case "red":
		return "\033[31m"
	case "blue":
		return "\033[34m"
	case "green":
		return "\033[32m"
	case "yellow":
		return "\033[33m"
	default:
		return ""
	}
}

// sortNodes sorts a slice of nodes based on sort criteria
func sortNodes(nodes []*vfs.Node, sortBy vfs.SortBy, ascending bool) []*vfs.Node {
	sorted := make([]*vfs.Node, len(nodes))
	copy(sorted, nodes)

	var lessFunc func(i, j int) bool

	switch sortBy {
	case vfs.SortByName:
		lessFunc = func(i, j int) bool {
			return strings.ToLower(sorted[i].Name) < strings.ToLower(sorted[j].Name)
		}
	case vfs.SortByCreated:
		lessFunc = func(i, j int) bool {
			return sorted[i].CreatedAt < sorted[j].CreatedAt
		}
	case vfs.SortByModified:
		lessFunc = func(i, j int) bool {
			return sorted[i].Modified < sorted[j].Modified
		}
	case vfs.SortBySize:
		lessFunc = func(i, j int) bool {
			return sorted[i].Size < sorted[j].Size
		}
	}

	// Bubble sort (simple for small lists)
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			// Always sort folders before files
			if sorted[i].IsDir != sorted[j].IsDir {
				if !sorted[i].IsDir && sorted[j].IsDir {
					sorted[i], sorted[j] = sorted[j], sorted[i]
				}
				continue
			}

			// Both are same type, apply sort
			shouldSwap := lessFunc(i, j)
			if !ascending {
				shouldSwap = !shouldSwap
			}
			if !shouldSwap {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	return sorted
}

// recursiveSearch searches through all subdirectories
func recursiveSearch(node *vfs.Node, query string, parentPath string) []*vfs.Node {
	if query == "" {
		return nil
	}

	results := []*vfs.Node{}
	query = strings.ToLower(query)

	// Check current node
	if strings.Contains(strings.ToLower(node.Name), query) {
		results = append(results, node)
	}

	// Search children if directory
	if node.IsDir && node.Children != nil {
		for _, child := range node.Children {
			childResults := recursiveSearch(child, query, "")
			results = append(results, childResults...)
		}
	}

	return results
}

// formatSize formats file size in human-readable format
func formatSize(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%dB", size)
	} else if size < 1024*1024 {
		return fmt.Sprintf("%.1fK", float64(size)/1024)
	} else {
		return fmt.Sprintf("%.1fM", float64(size)/(1024*1024))
	}
}

// formatDate formats unix timestamp to readable date
func formatDate(timestamp int64) string {
	if timestamp == 0 {
		return "                "
	}
	return time.Unix(timestamp, 0).Format("2006-01-02 15:04")
}

// openInEditor opens a file in the user's $EDITOR
func openInEditor(storage *vfs.GormStorage, node *vfs.Node) error {
	// Get file content
	content, err := storage.GetFileContent(node.ID)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	// Create temporary file
	tmpfile, err := os.CreateTemp("", fmt.Sprintf("vfs-*-%s", node.Name))
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	defer os.Remove(tmpfile.Name())

	// Write content to temp file
	if _, err := tmpfile.WriteString(content); err != nil {
		return fmt.Errorf("writing temp file: %w", err)
	}
	tmpfile.Close()

	// Get editor from environment or fallback to vim
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim"
	}

	// Restore terminal before opening editor
	restoreTerminal()

	// Open editor
	cmd := exec.Command(editor, tmpfile.Name())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		setupTerminal()
		return fmt.Errorf("running editor: %w", err)
	}

	// Re-setup terminal after editor closes
	setupTerminal()

	// Read edited content
	editedContent, err := os.ReadFile(tmpfile.Name())
	if err != nil {
		return fmt.Errorf("reading edited file: %w", err)
	}

	// Save back to storage if content changed
	if string(editedContent) != content {
		if err := storage.SetFileContent(node.ID, string(editedContent)); err != nil {
			return fmt.Errorf("saving file: %w", err)
		}
		// Update node metadata
		node.Modified = time.Now().Unix()
		node.Size = int64(len(editedContent))
	}

	return nil
}
