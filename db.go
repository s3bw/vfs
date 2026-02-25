package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Database models
type FileRecord struct {
	ID        string    `gorm:"primaryKey"`
	Name      string    `gorm:"not null"`
	ParentID  *string   `gorm:"index"` // null for root-level items
	IsDir     bool      `gorm:"not null"`
	Color     string    `gorm:"default:''"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
	Size      int64     `gorm:"default:0"`
}

// User preferences
type UserPreference struct {
	ID    uint   `gorm:"primaryKey"`
	Key   string `gorm:"uniqueIndex;not null"`
	Value string
}

// Directory state (sort, cursor)
type DirectoryState struct {
	ID        uint   `gorm:"primaryKey"`
	Path      string `gorm:"uniqueIndex;not null"`
	SortBy    int    `gorm:"default:0"` // 0=name, 1=created, 2=modified, 3=size
	SortAsc   bool   `gorm:"default:true"`
	CursorPos int    `gorm:"default:0"`
}

// Database manager
type DB struct {
	conn     *gorm.DB
	filesDir string
}

// Initialize database and file storage
func InitDB(dbPath string, filesDir string) (*DB, error) {
	// Ensure directories exist
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filesDir, 0755); err != nil {
		return nil, err
	}

	// Open database
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	// Auto-migrate schemas
	if err := db.AutoMigrate(&FileRecord{}, &UserPreference{}, &DirectoryState{}); err != nil {
		return nil, err
	}

	return &DB{
		conn:     db,
		filesDir: filesDir,
	}, nil
}

// LoadVFSFromDB loads the entire file tree from database
func (db *DB) LoadVFSFromDB() (*VFS, error) {
	// Load all file records
	var records []FileRecord
	if err := db.conn.Order("parent_id, is_dir DESC, name").Find(&records).Error; err != nil {
		return nil, err
	}

	// Build node map
	nodeMap := make(map[string]*Node)
	var root *Node

	// First pass: create all nodes
	for _, record := range records {
		node := &Node{
			ID:        record.ID,
			Name:      record.Name,
			IsDir:     record.IsDir,
			Color:     record.Color,
			CreatedAt: record.CreatedAt.Unix(),
			Modified:  record.UpdatedAt.Unix(),
			Size:      record.Size,
			Children:  []*Node{},
		}
		nodeMap[record.ID] = node

		// Root items have nil parent
		if record.ParentID == nil {
			if root == nil {
				// Create root directory
				root = &Node{
					ID:        "root",
					Name:      "root",
					IsDir:     true,
					CreatedAt: time.Now().Unix(),
					Modified:  time.Now().Unix(),
					Children:  []*Node{},
				}
			}
			root.Children = append(root.Children, node)
		}
	}

	// Second pass: build parent-child relationships
	for _, record := range records {
		if record.ParentID != nil {
			parent, exists := nodeMap[*record.ParentID]
			if exists {
				node := nodeMap[record.ID]
				parent.Children = append(parent.Children, node)
			}
		}
	}

	// If no root exists, create empty one
	if root == nil {
		root = &Node{
			ID:        "root",
			Name:      "root",
			IsDir:     true,
			CreatedAt: time.Now().Unix(),
			Modified:  time.Now().Unix(),
			Children:  []*Node{},
		}
	}

	// Create VFS
	vfs := &VFS{
		Root:       root,
		Current:    root,
		Stack:      []*Node{root},
		DirSortBy:  make(map[string]SortBy),
		DirSortAsc: make(map[string]bool),
		DirCursor:  make(map[string]int),
	}

	// Load directory states
	var states []DirectoryState
	if err := db.conn.Find(&states).Error; err == nil {
		for _, state := range states {
			vfs.DirSortBy[state.Path] = SortBy(state.SortBy)
			vfs.DirSortAsc[state.Path] = state.SortAsc
			vfs.DirCursor[state.Path] = state.CursorPos
		}
	}

	return vfs, nil
}

// SaveVFSToDB saves the entire file tree to database
func (db *DB) SaveVFSToDB(vfs *VFS) error {
	return db.conn.Transaction(func(tx *gorm.DB) error {
		// Recursively save all nodes
		return db.saveNodeRecursive(tx, vfs.Root, nil)
	})
}

func (db *DB) saveNodeRecursive(tx *gorm.DB, node *Node, parentID *string) error {
	// Skip root node
	if node.ID == "root" {
		for _, child := range node.Children {
			if err := db.saveNodeRecursive(tx, child, nil); err != nil {
				return err
			}
		}
		return nil
	}

	record := FileRecord{
		ID:        node.ID,
		Name:      node.Name,
		ParentID:  parentID,
		IsDir:     node.IsDir,
		Color:     node.Color,
		Size:      node.Size,
		CreatedAt: time.Unix(node.CreatedAt, 0),
		UpdatedAt: time.Unix(node.Modified, 0),
	}

	// Upsert
	if err := tx.Save(&record).Error; err != nil {
		return err
	}

	// Save children
	if node.IsDir {
		nodeID := node.ID
		for _, child := range node.Children {
			if err := db.saveNodeRecursive(tx, child, &nodeID); err != nil {
				return err
			}
		}
	}

	return nil
}

// SaveDirectoryStates saves all directory states (sort, cursor)
func (db *DB) SaveDirectoryStates(vfs *VFS) error {
	return db.conn.Transaction(func(tx *gorm.DB) error {
		// Clear old states
		if err := tx.Where("1 = 1").Delete(&DirectoryState{}).Error; err != nil {
			return err
		}

		// Save new states
		for path, sortBy := range vfs.DirSortBy {
			state := DirectoryState{
				Path:      path,
				SortBy:    int(sortBy),
				SortAsc:   vfs.DirSortAsc[path],
				CursorPos: vfs.DirCursor[path],
			}
			if err := tx.Create(&state).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

// CreateFile creates a new file in the database and filesystem
func (db *DB) CreateFile(node *Node, parentID *string) error {
	// Generate UUID for file
	if node.ID == "" {
		node.ID = generateUUID()
	}

	// Create database record
	record := FileRecord{
		ID:        node.ID,
		Name:      node.Name,
		ParentID:  parentID,
		IsDir:     node.IsDir,
		Color:     node.Color,
		Size:      node.Size,
		CreatedAt: time.Unix(node.CreatedAt, 0),
		UpdatedAt: time.Unix(node.Modified, 0),
	}

	if err := db.conn.Create(&record).Error; err != nil {
		return err
	}

	// Create empty file if it's a .do file
	if !node.IsDir {
		filePath := filepath.Join(db.filesDir, node.ID+".do")
		if err := os.WriteFile(filePath, []byte(""), 0644); err != nil {
			return err
		}
	}

	return nil
}

// CopyFile creates a copy of a file/folder with new UUID and optionally copies content
// For folders, recursively copies all children
func (db *DB) CopyFile(sourceNode *Node, parentID *string, copyContent bool) (*Node, error) {
	// Create a new node with new UUID
	newNode := &Node{
		ID:        generateUUID(), // Generate new UUID
		Name:      sourceNode.Name,
		IsDir:     sourceNode.IsDir,
		Color:     sourceNode.Color,
		CreatedAt: time.Now().Unix(),
		Modified:  time.Now().Unix(),
		Size:      sourceNode.Size,
		Children:  []*Node{},
	}

	// Create in database
	if err := db.CreateFile(newNode, parentID); err != nil {
		return nil, err
	}

	// If it's a file, copy content
	if !sourceNode.IsDir && copyContent {
		content, err := db.GetFileContent(sourceNode.ID)
		if err == nil {
			db.SetFileContent(newNode.ID, content)
		}
	}

	// If it's a folder, recursively copy all children
	if sourceNode.IsDir && sourceNode.Children != nil {
		newNodeID := newNode.ID
		for _, child := range sourceNode.Children {
			// Recursively copy each child
			copiedChild, err := db.CopyFile(child, &newNodeID, copyContent)
			if err != nil {
				// Log error but continue with other children
				fmt.Fprintf(os.Stderr, "Warning: failed to copy child %s: %v\n", child.Name, err)
				continue
			}
			newNode.Children = append(newNode.Children, copiedChild)
		}
	}

	return newNode, nil
}

// DeleteFile deletes a file from database and filesystem
func (db *DB) DeleteFile(fileID string) error {
	return db.conn.Transaction(func(tx *gorm.DB) error {
		// Get file record
		var record FileRecord
		if err := tx.First(&record, "id = ?", fileID).Error; err != nil {
			return err
		}

		// Delete from database
		if err := tx.Delete(&record).Error; err != nil {
			return err
		}

		// Delete file from filesystem if it's not a directory
		if !record.IsDir {
			filePath := filepath.Join(db.filesDir, fileID+".do")
			os.Remove(filePath) // Ignore error if file doesn't exist
		}

		return nil
	})
}

// RenameFile renames a file in the database
func (db *DB) RenameFile(fileID string, newName string) error {
	return db.conn.Model(&FileRecord{}).
		Where("id = ?", fileID).
		Update("name", newName).Error
}

// GetFileContent reads file content from filesystem
func (db *DB) GetFileContent(fileID string) (string, error) {
	filePath := filepath.Join(db.filesDir, fileID+".do")
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// SetFileContent writes file content to filesystem
func (db *DB) SetFileContent(fileID string, content string) error {
	filePath := filepath.Join(db.filesDir, fileID+".do")

	// Update size in database
	size := int64(len(content))
	if err := db.conn.Model(&FileRecord{}).
		Where("id = ?", fileID).
		Updates(map[string]interface{}{
			"size":       size,
			"updated_at": time.Now(),
		}).Error; err != nil {
		return err
	}

	return os.WriteFile(filePath, []byte(content), 0644)
}

// Simple UUID generator (you might want to use github.com/google/uuid)
func generateUUID() string {
	// Simple timestamp-based ID for demo
	// In production, use: uuid.New().String()
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
