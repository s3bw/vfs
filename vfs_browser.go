package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// ANSI color codes
const (
	Reset      = "\033[0m"
	Bold       = "\033[1m"
	Purple     = "\033[35m"
	Green      = "\033[32m"
	Gray       = "\033[90m"
	BrightBlue = "\033[94m"
	Cyan       = "\033[36m"
	Yellow     = "\033[33m"
)

// Nerd Font icons
const (
	IconFolder     = "\uf114" //
	IconFolderOpen = "\uf115" //
	IconMarkdown   = "\uf48a" //
	IconGo         = "\ue627" //
	IconHTML       = "\ue736" //
	IconCSS        = "\ue749" //
	IconJS         = "\ue74e" //
	IconText       = "\uf15c" //
	IconFile       = "\uf15b" //
	IconCode       = "\ue796" //
)

// Node represents a file or directory
type Node struct {
	ID        string
	Name      string
	IsDir     bool
	Children  []*Node
	Color     string // Color tag: red, blue, green, yellow
	CreatedAt int64  // Unix timestamp
	Modified  int64  // Unix timestamp
	Size      int64  // File size in bytes
}

// SortBy represents the sorting method
type SortBy int

const (
	SortByName SortBy = iota
	SortByCreated
	SortByModified
	SortBySize
)

// VFS represents the virtual file system
type VFS struct {
	Root    *Node
	Current *Node
	Stack   []*Node
	// Sort state per directory
	DirSortBy  map[string]SortBy
	DirSortAsc map[string]bool
	// Cursor position per directory
	DirCursor map[string]int
}

// Clipboard for cut/paste
type Clipboard struct {
	Node *Node
	Path string
}

// Model represents the application state
type Model struct {
	vfs           *VFS
	cursor        int
	showingFile   bool
	selectedFile  *Node
	quitting      bool
	searchMode    bool
	searchQuery   string
	sortBy        SortBy
	sortAsc       bool
	clipboard     *Clipboard
	statusMessage string
	colorMode     bool // When true, waiting for color selection
	sortMode      bool // When true, waiting for sort selection
	newItemMode   bool // When true, waiting for new item name
	newItemName   string
	newItemIsDir  bool
	lastKey       string // Track last key for double-press detection
	renameMode    bool   // When true, waiting for new name
	renameText    string
	db            *DB
}

// NewVFS creates a new virtual file system with the example structure
func NewVFS() *VFS {
	root := &Node{Name: "project", IsDir: true, CreatedAt: 1700000000, Modified: 1700000000}

	reviewsBlogs := &Node{Name: "reviews/blogs", IsDir: true, CreatedAt: 1700000100, Modified: 1700000100}
	reviewsBlogs.Children = []*Node{
		{Name: "post1.md", IsDir: false, CreatedAt: 1700000200, Modified: 1700000500, Size: 2048},
		{Name: "post2.md", IsDir: false, CreatedAt: 1700000300, Modified: 1700000600, Size: 3072},
		{Name: "draft.md", IsDir: false, CreatedAt: 1700000400, Modified: 1700000700, Size: 1024},
	}

	oneToOne := &Node{Name: "1to1", IsDir: true, CreatedAt: 1700000150, Modified: 1700000150}
	oneToOne.Children = []*Node{
		{Name: "alice.txt", IsDir: false, CreatedAt: 1700000250, Modified: 1700000550, Size: 512},
		{Name: "bob.txt", IsDir: false, CreatedAt: 1700000350, Modified: 1700000650, Size: 768},
	}

	tests := &Node{Name: "tests", IsDir: true, CreatedAt: 1700000180, Modified: 1700000180}
	tests.Children = []*Node{
		{Name: "test_main.go", IsDir: false, CreatedAt: 1700000280, Modified: 1700000580, Size: 4096},
		{Name: "test_utils.go", IsDir: false, CreatedAt: 1700000380, Modified: 1700000680, Size: 2560},
	}

	public := &Node{Name: "public", IsDir: true, CreatedAt: 1700000200, Modified: 1700000200}
	public.Children = []*Node{
		{Name: "index.html", IsDir: false, CreatedAt: 1700000300, Modified: 1700000600, Size: 5120},
		{Name: "styles.css", IsDir: false, CreatedAt: 1700000400, Modified: 1700000700, Size: 1536},
		{Name: "script.js", IsDir: false, CreatedAt: 1700000500, Modified: 1700000800, Size: 3584},
	}

	root.Children = []*Node{
		reviewsBlogs,
		{Name: "recent1to1.do", IsDir: false, CreatedAt: 1700000120, Modified: 1700000520, Size: 896},
		oneToOne,
		tests,
		public,
	}

	return &VFS{
		Root:       root,
		Current:    root,
		Stack:      []*Node{root},
		DirSortBy:  make(map[string]SortBy),
		DirSortAsc: make(map[string]bool),
		DirCursor:  make(map[string]int),
	}
}

func (vfs *VFS) GetCurrentItems() []*Node {
	if vfs.Current == nil {
		return nil
	}
	return vfs.Current.Children
}

func (vfs *VFS) Enter(node *Node) bool {
	if !node.IsDir {
		return false
	}
	vfs.Stack = append(vfs.Stack, node)
	vfs.Current = node
	return true
}

func (vfs *VFS) GoBack() bool {
	if len(vfs.Stack) <= 1 {
		return false
	}
	vfs.Stack = vfs.Stack[:len(vfs.Stack)-1]
	vfs.Current = vfs.Stack[len(vfs.Stack)-1]
	return true
}

func (vfs *VFS) GetPath() string {
	parts := []string{}
	for _, node := range vfs.Stack {
		parts = append(parts, node.Name)
	}
	return strings.Join(parts, "/")
}

// SaveSortState saves sort preferences for current directory
func (vfs *VFS) SaveSortState(sortBy SortBy, sortAsc bool) {
	path := vfs.GetPath()
	vfs.DirSortBy[path] = sortBy
	vfs.DirSortAsc[path] = sortAsc
}

// LoadSortState loads sort preferences for current directory
func (vfs *VFS) LoadSortState() (SortBy, bool, bool) {
	path := vfs.GetPath()
	sortBy, hasSortBy := vfs.DirSortBy[path]
	sortAsc, hasSortAsc := vfs.DirSortAsc[path]

	if hasSortBy && hasSortAsc {
		return sortBy, sortAsc, true
	}
	return SortByName, true, false
}

// SaveCursorPosition saves cursor position for current directory
func (vfs *VFS) SaveCursorPosition(cursor int) {
	path := vfs.GetPath()
	vfs.DirCursor[path] = cursor
}

// LoadCursorPosition loads cursor position for current directory
func (vfs *VFS) LoadCursorPosition() (int, bool) {
	path := vfs.GetPath()
	cursor, found := vfs.DirCursor[path]
	return cursor, found
}

// getFileIcon returns the appropriate Nerd Font icon for a file
func getFileIcon(name string) string {
	if strings.HasSuffix(name, ".do") {
		return IconText
	} else {
		return IconFile
	}
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
func sortNodes(nodes []*Node, sortBy SortBy, ascending bool) []*Node {
	sorted := make([]*Node, len(nodes))
	copy(sorted, nodes)

	var lessFunc func(i, j int) bool

	switch sortBy {
	case SortByName:
		lessFunc = func(i, j int) bool {
			return strings.ToLower(sorted[i].Name) < strings.ToLower(sorted[j].Name)
		}
	case SortByCreated:
		lessFunc = func(i, j int) bool {
			return sorted[i].CreatedAt < sorted[j].CreatedAt
		}
	case SortByModified:
		lessFunc = func(i, j int) bool {
			return sorted[i].Modified < sorted[j].Modified
		}
	case SortBySize:
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
					// i is file, j is folder - swap
					sorted[i], sorted[j] = sorted[j], sorted[i]
				}
				continue
			}

			// Both are same type (both folders or both files), apply sort
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

// filterNodes filters nodes by search query
func filterNodes(nodes []*Node, query string) []*Node {
	if query == "" {
		return nodes
	}

	filtered := []*Node{}
	query = strings.ToLower(query)

	for _, node := range nodes {
		if strings.Contains(strings.ToLower(node.Name), query) {
			filtered = append(filtered, node)
		}
	}

	return filtered
}

// recursiveSearch searches through all subdirectories
func recursiveSearch(node *Node, query string, parentPath string) []*Node {
	if query == "" {
		return nil
	}

	results := []*Node{}
	query = strings.ToLower(query)

	// Check current node
	if strings.Contains(strings.ToLower(node.Name), query) {
		// Return the node as-is (just the name, not full path)
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
	// For demo purposes with fake timestamps, calculate a demo date
	// In real use: time.Unix(timestamp, 0).Format("2006-01-02 15:04")

	// Convert timestamp offset to a date (starting from 2026-03-23)
	baseDate := int64(1700000000) // Our base timestamp
	daysSince := (timestamp - baseDate) / 86400
	hoursSince := ((timestamp - baseDate) % 86400) / 3600
	minutesSince := ((timestamp - baseDate) % 3600) / 60

	// Base date: March 23, 2026
	day := 23 + int(daysSince)
	month := 3
	year := 2026

	// Simple month overflow handling
	if day > 31 {
		day = day % 31
		month++
	}
	if month > 12 {
		month = month % 12
		year++
	}

	hour := int(hoursSince) % 24
	minute := int(minutesSince) % 60

	return fmt.Sprintf("%04d-%02d-%02d %02d:%02d", year, month, day, hour, minute)
}

// getCurrentDate returns the current date in YYYY-MM-DD format
func getCurrentDate() string {
	// For the virtual system, return a fixed current date
	// In real use: time.Now().Format("2006-01-02")
	return "2026-01-15"
}

// processDateShortcut replaces /d with current date
func processDateShortcut(text string) string {
	return strings.ReplaceAll(text, "/d", getCurrentDate())
}

func InitialModel() Model {
	db, err := InitDB("file.db", "files")
	if err != nil {
		panic(err)
	}

	vfs, err := db.LoadVFSFromDB()
	if err != nil {
		panic(err)
	}

	return Model{
		vfs:       vfs,
		cursor:    0,
		sortBy:    SortByName,
		sortAsc:   true,
		clipboard: nil,
		db:        db,
	}
}

func checkName(name string, children []*Node) string {
	for _, child := range children {
		if child.Name == name {
			baseName := name
			// Check if it's a file, else directory
			if strings.HasSuffix(baseName, ".do") {
				baseName = baseName[:len(baseName)-3]
				name = baseName + "_copy.do"
			} else {
				name = name + "_copy"
			}
			name = checkName(name, children)
			break
		}
	}
	return name
}

func (m Model) Init() {}

func (m Model) Update(key string) Model {
	// Handle rename mode
	if m.renameMode {
		if key == "enter" {
			if m.renameText != "" {
				items := m.getDisplayItems()
				if m.cursor >= 0 && m.cursor < len(items) {
					selected := items[m.cursor]
					oldName := selected.Name
					newName := processDateShortcut(m.renameText)

					if err := m.db.RenameFile(selected.ID, newName); err != nil {
						m.statusMessage = fmt.Sprintf("Error: %v", err)
					} else {
						m.statusMessage = fmt.Sprintf("Renamed '%s' to '%s'", oldName, newName)
					}
				}
			}
			m.renameMode = false
			m.renameText = ""
			return m
		} else if key == "backspace" {
			if len(m.renameText) > 0 {
				m.renameText = m.renameText[:len(m.renameText)-1]
			}
			return m
		} else if key == "space" {
			m.renameText += " "
			return m
		} else if len(key) == 1 && key[0] >= 32 && key[0] <= 126 {
			m.renameText += key
			return m
		}
		return m
	}

	// Handle new item mode
	if m.newItemMode {
		if key == "enter" {
			if m.newItemName != "" {
				// Process date shortcuts before creating
				finalName := processDateShortcut(m.newItemName)
				finalName = checkName(finalName, m.vfs.Current.Children)

				// Determine if it's a file or directory
				isFile := strings.HasSuffix(finalName, ".do")

				newNode := &Node{
					Name:      finalName,
					IsDir:     !isFile,
					CreatedAt: 1700001000,
					Modified:  1700001000,
					Size:      0,
				}

				if !isFile {
					newNode.Children = []*Node{}
				}

				// Get parent ID
				var parentID *string
				if m.vfs.Current.ID != "root" {
					id := m.vfs.Current.ID
					parentID = &id
				}
				if err := m.db.CreateFile(newNode, parentID); err != nil {
					m.statusMessage = fmt.Sprintf("Error: %v", err)
				} else {
					m.vfs.Current.Children = append(m.vfs.Current.Children, newNode)
					itemType := "folder"
					if isFile {
						itemType = "file"
					}
					m.statusMessage = fmt.Sprintf("Created %s: %s", itemType, finalName)
				}
			}
			m.newItemMode = false
			m.newItemName = ""
			return m
		} else if key == "backspace" {
			if len(m.newItemName) > 0 {
				m.newItemName = m.newItemName[:len(m.newItemName)-1]
			}
			return m
		} else if key == "space" {
			m.newItemName += " "
			return m
		} else if len(key) == 1 && key[0] >= 32 && key[0] <= 126 {
			m.newItemName += key
			return m
		}
		return m
	}

	// Handle search mode
	if m.searchMode {
		if key == "enter" {
			m.searchMode = false
			m.cursor = 0
			return m
		} else if key == "backspace" {
			if len(m.searchQuery) > 0 {
				m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
			}
			return m
		} else if len(key) == 1 && key[0] >= 32 && key[0] <= 126 {
			m.searchQuery += key
			return m
		}
		return m
	}

	// Handle color mode
	if m.colorMode {
		items := m.getDisplayItems()
		if m.cursor >= 0 && m.cursor < len(items) {
			selected := items[m.cursor]
			switch key {
			case "1":
				selected.Color = "red"
				m.statusMessage = "Colored red"
			case "2":
				selected.Color = "blue"
				m.statusMessage = "Colored blue"
			case "3":
				selected.Color = "green"
				m.statusMessage = "Colored green"
			case "4":
				selected.Color = "yellow"
				m.statusMessage = "Colored yellow"
			}
		}
		m.colorMode = false
		return m
	}

	// Handle sort mode
	if m.sortMode {
		newSortBy := m.sortBy
		switch key {
		case "1":
			newSortBy = SortByName
		case "2":
			newSortBy = SortByCreated
		case "3":
			newSortBy = SortByModified
		case "4":
			newSortBy = SortBySize
		}

		// Toggle asc/desc if same sort method
		if newSortBy == m.sortBy {
			m.sortAsc = !m.sortAsc
		} else {
			m.sortBy = newSortBy
			m.sortAsc = true
		}

		// Save sort state for current directory
		m.vfs.SaveSortState(m.sortBy, m.sortAsc)

		m.sortMode = false
		m.statusMessage = fmt.Sprintf("Sort by %s (%s)", []string{"name", "created", "modified", "size"}[m.sortBy], map[bool]string{true: "asc", false: "desc"}[m.sortAsc])
		return m
	}

	// If showing file view, any key goes back
	if m.showingFile {
		m.showingFile = false
		return m
	}

	// Clear status message on most keys
	if m.statusMessage != "" && key != "d" && key != "p" && key != "s" && key != "c" && key != "n" {
		m.statusMessage = ""
	}

	switch key {
	case "q", "Q":
		m.quitting = true
		return m

	case "tab", "down":
		items := m.getDisplayItems()
		maxCursor := len(items) - 1
		if m.cursor == -1 {
			m.cursor = 0
		} else if m.cursor < maxCursor {
			m.cursor++
		} else {
			// Wrap around to beginning
			m.cursor = 0
			if len(m.vfs.Stack) > 1 {
				m.cursor = -1
			}
		}
		// Save cursor position
		if !m.searchMode {
			m.vfs.SaveCursorPosition(m.cursor)
		}
		m.lastKey = key

	case "shifttab", "up":
		items := m.getDisplayItems()
		minIdx := 0
		if len(m.vfs.Stack) > 1 {
			minIdx = -1
		}

		if m.cursor > minIdx {
			m.cursor--
		} else {
			// Wrap around to end
			m.cursor = len(items) - 1
		}
		// Save cursor position
		if !m.searchMode {
			m.vfs.SaveCursorPosition(m.cursor)
		}
		m.lastKey = key

	case "h", "left", "backspace":
		if m.vfs.GoBack() {
			// Load cursor position for parent directory
			if cursor, found := m.vfs.LoadCursorPosition(); found {
				m.cursor = cursor
			} else {
				m.cursor = 0
			}
			m.searchQuery = ""
			m.searchMode = false
			// Load sort state for this directory
			if sortBy, sortAsc, found := m.vfs.LoadSortState(); found {
				m.sortBy = sortBy
				m.sortAsc = sortAsc
			}
		}
		m.lastKey = key

	case "enter":
		// In search mode, enter opens the file/folder
		if m.searchMode {
			items := m.getDisplayItems()
			if m.cursor >= 0 && m.cursor < len(items) {
				selected := items[m.cursor]
				if selected.IsDir {
					// Navigate to the directory
					// Parse the path and navigate there
					m.showingFile = false
					m.statusMessage = "Cannot navigate to folders from search (press backspace to exit search)"
				} else {
					m.showingFile = true
					m.selectedFile = selected
				}
			}
			m.lastKey = key
			return m
		}

		// Handle parent directory
		if m.cursor == -1 {
			if m.vfs.GoBack() {
				// Load cursor position for parent directory
				if cursor, found := m.vfs.LoadCursorPosition(); found {
					m.cursor = cursor
				} else {
					m.cursor = 0
				}
				m.searchQuery = ""
				m.searchMode = false
				// Load sort state for this directory
				if sortBy, sortAsc, found := m.vfs.LoadSortState(); found {
					m.sortBy = sortBy
					m.sortAsc = sortAsc
				}
			}
			m.lastKey = key
			return m
		}

		items := m.getDisplayItems()
		if m.cursor >= 0 && m.cursor < len(items) {
			selected := items[m.cursor]
			if selected.IsDir {
				// Save current cursor position and sort state before leaving
				m.vfs.SaveCursorPosition(m.cursor)
				m.vfs.SaveSortState(m.sortBy, m.sortAsc)

				m.vfs.Enter(selected)

				// Load cursor position for new directory
				if cursor, found := m.vfs.LoadCursorPosition(); found {
					m.cursor = cursor
				} else {
					m.cursor = 0
				}
				m.searchQuery = ""
				m.searchMode = false

				// Load sort state for new directory
				if sortBy, sortAsc, found := m.vfs.LoadSortState(); found {
					m.sortBy = sortBy
					m.sortAsc = sortAsc
				}
			} else {
				m.showingFile = true
				m.selectedFile = selected
			}
		}
		m.lastKey = key

	case "d":
		// Check for double-d
		if m.lastKey == "d" {
			// Cut file - remove from current directory
			items := m.getDisplayItems()
			if m.cursor >= 0 && m.cursor < len(items) {
				cutNode := items[m.cursor]
				m.clipboard = &Clipboard{
					Node: cutNode,
					Path: m.vfs.GetPath(),
				}

				// Remove from current directory
				newChildren := []*Node{}
				for _, child := range m.vfs.Current.Children {
					if child != cutNode {
						newChildren = append(newChildren, child)
					}
				}
				m.vfs.Current.Children = newChildren

				// Adjust cursor if needed
				if m.cursor >= len(newChildren) && len(newChildren) > 0 {
					m.cursor = len(newChildren) - 1
				}

				m.statusMessage = fmt.Sprintf("Cut: %s (clipboard persists)", cutNode.Name)
			}
			m.lastKey = ""
		} else {
			m.lastKey = "d"
		}
		return m

	case "P", "p":
		// Paste file - clipboard persists for duplication
		if m.clipboard != nil {
			var parentID *string
			if m.vfs.Current.ID != "root" {
				id := m.vfs.Current.ID
				parentID = &id
			}

			nodeCopy := m.clipboard.Node
			nodeCopy.Name = checkName(nodeCopy.Name, m.vfs.Current.Children)
			newNode, err := m.db.CopyFile(nodeCopy, parentID, true)
			if err != nil {
				m.statusMessage = fmt.Sprintf("Error: %v", err)
			} else {
				// Add to current directory
				// Check if file with same name exists
				m.vfs.Current.Children = append(m.vfs.Current.Children, newNode)
				m.statusMessage = fmt.Sprintf("Pasted: %s (clipboard persists)", nodeCopy.Name)
			}
		} else {
			m.statusMessage = "Clipboard empty"
		}
		m.lastKey = key

	case "s":
		m.sortMode = true
		m.statusMessage = "Sort by: [1]name [2]created [3]modified [4]size"
		m.lastKey = key

	case "c":
		m.colorMode = true
		m.statusMessage = "Color: [1]red [2]blue [3]green [4]yellow"
		m.lastKey = key

	case "/":
		m.searchMode = true
		m.searchQuery = ""
		m.statusMessage = "Search (recursive):"
		m.lastKey = key

	case "n":
		m.newItemMode = true
		m.newItemIsDir = false // Default to file
		m.newItemName = ""
		m.statusMessage = "New item name (add .do for file, no extension for folder):"
		m.lastKey = key

	case "r":
		// Rename selected file/folder
		items := m.getDisplayItems()
		if m.cursor >= 0 && m.cursor < len(items) {
			m.renameMode = true
			m.renameText = items[m.cursor].Name
			m.statusMessage = "Rename to:"
		}
		m.lastKey = key

	default:
		m.lastKey = key
	}

	return m
}

// getDisplayItems returns items to display (filtered and sorted)
func (m Model) getDisplayItems() []*Node {
	var items []*Node

	if m.searchMode || m.searchQuery != "" {
		// Use recursive search from root
		items = recursiveSearch(m.vfs.Root, m.searchQuery, "")
		// Remove the root itself if it matches
		filtered := []*Node{}
		for _, item := range items {
			if item.Name != m.vfs.Root.Name {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	} else {
		items = m.vfs.GetCurrentItems()
	}

	items = sortNodes(items, m.sortBy, m.sortAsc)

	return items
}

func (m Model) View() string {
	if m.showingFile {
		return m.renderFileView()
	}

	var b strings.Builder

	// Title
	title := fmt.Sprintf("%s%s Captain System%s\n", Bold, Purple, Reset)
	b.WriteString(title)

	// Current path
	path := fmt.Sprintf("%s%s%s %s%s\n", Bold, Green, IconFolderOpen, m.vfs.GetPath(), Reset)
	b.WriteString(path)

	// Sort indicator
	sortName := []string{"Name", "Created", "Modified", "Size"}[m.sortBy]
	sortDir := "↓"
	if m.sortAsc {
		sortDir = "↑"
	}
	b.WriteString(fmt.Sprintf("%sSort: %s %s", Gray, sortName, sortDir))
	if m.clipboard != nil {
		b.WriteString(fmt.Sprintf(" | Clipboard: %s", m.clipboard.Node.Name))
	}
	b.WriteString(fmt.Sprintf("%s\n\n", Reset))

	// Column headers
	b.WriteString(fmt.Sprintf("%s%-32s %-16s %-8s%s\n", Gray, "Name", "Created", "Size", Reset))
	b.WriteString(fmt.Sprintf("%s%s%s\n", Gray, strings.Repeat("─", 60), Reset))

	// Parent directory option
	if len(m.vfs.Stack) > 1 {
		if m.cursor == -1 {
			b.WriteString(fmt.Sprintf("%s%s▶ %s ..%s\n", Bold, BrightBlue, IconFolder, Reset))
		} else {
			b.WriteString(fmt.Sprintf("  %s ..\n", IconFolder))
		}
	}

	// Items
	items := m.getDisplayItems()
	for i, item := range items {
		var icon string
		if item.IsDir {
			icon = IconFolder
		} else {
			icon = getFileIcon(item.Name)
		}

		// Color prefix
		colorCode := getColorCode(item.Color)
		colorReset := ""
		if colorCode != "" {
			colorReset = Reset
		}

		// Format name with padding (account for icon + space = 2 chars width in display)
		displayName := item.Name
		maxNameLen := 28
		if len(displayName) > maxNameLen {
			displayName = displayName[:maxNameLen-3] + "..."
		}

		// Date and size
		created := formatDate(item.CreatedAt)
		sizeStr := "        "
		if !item.IsDir {
			sizeStr = formatSize(item.Size)
		}

		if m.cursor == i {
			// Selected item - with cursor
			b.WriteString(fmt.Sprintf("%s%s▶ %s%s %-28s %-16s %-8s%s%s\n",
				Bold, BrightBlue, colorCode, icon, displayName, created, sizeStr, colorReset, Reset))
		} else {
			// Normal item - with spacing to align with cursor lines
			b.WriteString(fmt.Sprintf("  %s%s %-28s %-16s %-8s%s\n",
				colorCode, icon, displayName, created, sizeStr, Reset))
		}
	}

	// Empty state
	if len(items) == 0 {
		if m.searchMode || m.searchQuery != "" {
			b.WriteString(fmt.Sprintf("  %s(no results)%s\n", Gray, Reset))
		} else {
			b.WriteString(fmt.Sprintf("  %s(empty directory)%s\n", Gray, Reset))
		}
	}

	b.WriteString("\n")

	// Status/Command info at bottom
	if m.renameMode {
		displayText := processDateShortcut(m.renameText)
		b.WriteString(fmt.Sprintf("%sRename to: %s%s\n", Yellow, displayText, Reset))
	} else if m.newItemMode {
		displayText := processDateShortcut(m.newItemName)
		b.WriteString(fmt.Sprintf("%sNew item: %s%s\n", Yellow, displayText, Reset))
	} else if m.searchMode {
		b.WriteString(fmt.Sprintf("%sSearch: %s%s\n", Yellow, m.searchQuery, Reset))
	} else if m.sortMode {
		b.WriteString(fmt.Sprintf("%s%s%s\n", Yellow, m.statusMessage, Reset))
	} else if m.colorMode {
		b.WriteString(fmt.Sprintf("%s%s%s\n", Yellow, m.statusMessage, Reset))
	} else if m.statusMessage != "" {
		b.WriteString(fmt.Sprintf("%s%s%s\n", Cyan, m.statusMessage, Reset))
	}

	// Help at bottom
	b.WriteString(fmt.Sprintf("%s", Gray))
	b.WriteString("Tab/Shift+Tab: scroll • Enter: open • /: search • n: new • r: rename\n")
	b.WriteString("dd: cut • p: paste • s[1-4]: sort • c[1-4]: color • q: quit")
	b.WriteString(fmt.Sprintf("%s\n", Reset))

	return b.String()
}

func (m Model) renderFileView() string {
	var b strings.Builder

	content, err := m.db.GetFileContent(m.selectedFile.ID)
	if err != nil {
		fmt.Printf("Error reading file: %v\n", err)
	}

	b.WriteString(content)

	return b.String()
}

func setupTerminal() {
	// Disable line buffering and echo
	cmd := exec.Command("stty", "-echo", "cbreak")
	cmd.Stdin = os.Stdin
	_ = cmd.Run()
}

func restoreTerminal() {
	// Restore terminal settings
	cmd := exec.Command("stty", "echo", "-cbreak")
	cmd.Stdin = os.Stdin
	_ = cmd.Run()
}

func readKey() string {
	reader := bufio.NewReader(os.Stdin)

	b, err := reader.ReadByte()
	if err != nil {
		return ""
	}

	// Handle single byte inputs
	switch b {
	case 9: // Tab
		return "tab"
	case 'q', 'Q':
		return "q"
	case 'k':
		return "k"
	case 'j':
		return "j"
	case 'h':
		return "h"
	case 'l':
		return "l"
	case 'd':
		return "d"
	case 'p':
		return "p"
	case 'D':
		return "D"
	case 'P':
		return "P"
	case 's':
		return "s"
	case 'c':
		return "c"
	case 'n':
		return "n"
	case 'r':
		return "r"
	case '/':
		return "/"
	case '1', '2', '3', '4':
		return string(b)
	case 13, 10: // Enter
		return "enter"
	case 32: // Space
		return "space"
	case 127: // Backspace/Delete
		return "backspace"
	case 3: // Ctrl+C
		return "q"
	case 27: // ESC - might be arrow key or special sequence
		// Check if there are more bytes
		next, err := reader.ReadByte()
		if err != nil {
			return ""
		}
		if next == 91 { // '['
			arrow, err := reader.ReadByte()
			if err != nil {
				return ""
			}
			switch arrow {
			case 65:
				return "up"
			case 66:
				return "down"
			case 67:
				return "right"
			case 68:
				return "left"
			case 90: // Shift+Tab
				return "shifttab"
			}
		}
	default:
		// Return any printable character for search/new item modes
		if b >= 32 && b <= 126 {
			return string(b)
		}
	}

	return ""
}

func main() {
	// Setup terminal
	setupTerminal()
	defer restoreTerminal()

	// Setup: alternate screen, hide cursor
	fmt.Print("\033[?1049h\033[?25l")

	model := InitialModel()
	model.Init()

	for !model.quitting {
		// Clear screen and render
		fmt.Print("\033[H\033[2J")
		fmt.Print(model.View())

		// Read input
		key := readKey()
		if key != "" {
			model = model.Update(key)
		}
	}

	if err := model.db.SaveVFSToDB(model.vfs); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving: %v\n", err)
	}

	if err := model.db.SaveDirectoryStates(model.vfs); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving states: %v\n", err)
	}

	fmt.Print("\033[?1049l\033[?25h")
	fmt.Println("Goodbye!")
}
