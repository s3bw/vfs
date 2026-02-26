package vfs

import (
	"strings"
	"time"
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
	storage Storage
	// Sort state per directory
	DirSortBy  map[string]SortBy
	DirSortAsc map[string]bool
	// Cursor position per directory
	DirCursor map[string]int
}

// NewVFS creates a new VFS with the given storage backend
func NewVFS(storage Storage) (*VFS, error) {
	return storage.LoadVFSFromDB()
}

// GetCurrentItems returns the children of the current directory
func (vfs *VFS) GetCurrentItems() []*Node {
	if vfs.Current == nil {
		return nil
	}
	return vfs.Current.Children
}

// Enter navigates into a directory
func (vfs *VFS) Enter(node *Node) bool {
	if !node.IsDir {
		return false
	}
	vfs.Stack = append(vfs.Stack, node)
	vfs.Current = node
	return true
}

// GoBack navigates to parent directory
func (vfs *VFS) GoBack() bool {
	if len(vfs.Stack) <= 1 {
		return false
	}
	vfs.Stack = vfs.Stack[:len(vfs.Stack)-1]
	vfs.Current = vfs.Stack[len(vfs.Stack)-1]
	return true
}

// GetPath returns the current path as a string
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

// CreateFile creates a new file or directory at the current location
func (vfs *VFS) CreateFile(name string, isDir bool) (*Node, error) {
	node := &Node{
		Name:      name,
		IsDir:     isDir,
		CreatedAt: time.Now().Unix(),
		Modified:  time.Now().Unix(),
		Size:      0,
		Children:  []*Node{},
	}

	// Get parent ID
	var parentID *string
	if vfs.Current.ID != "root" {
		id := vfs.Current.ID
		parentID = &id
	}

	// Create in storage
	if err := vfs.storage.CreateFile(node, parentID); err != nil {
		return nil, err
	}

	// Add to current directory
	vfs.Current.Children = append(vfs.Current.Children, node)

	return node, nil
}

// DeleteFile deletes a file or directory
func (vfs *VFS) DeleteFile(node *Node) error {
	if err := vfs.storage.DeleteFile(node.ID); err != nil {
		return err
	}

	// Remove from current directory
	newChildren := []*Node{}
	for _, child := range vfs.Current.Children {
		if child.ID != node.ID {
			newChildren = append(newChildren, child)
		}
	}
	vfs.Current.Children = newChildren

	return nil
}

// RenameFile renames a file or directory
func (vfs *VFS) RenameFile(node *Node, newName string) error {
	if err := vfs.storage.RenameFile(node.ID, newName); err != nil {
		return err
	}
	node.Name = newName
	return nil
}

// GetFileContent retrieves the content of a file
func (vfs *VFS) GetFileContent(node *Node) (string, error) {
	return vfs.storage.GetFileContent(node.ID)
}

// SetFileContent sets the content of a file
func (vfs *VFS) SetFileContent(node *Node, content string) error {
	if err := vfs.storage.SetFileContent(node.ID, content); err != nil {
		return err
	}
	node.Size = int64(len(content))
	node.Modified = time.Now().Unix()
	return nil
}

// Save persists all changes to storage
func (vfs *VFS) Save() error {
	if err := vfs.storage.SaveVFSToDB(vfs); err != nil {
		return err
	}
	return vfs.storage.SaveDirectoryStates(vfs)
}
