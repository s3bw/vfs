package vfs

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gorm.io/gorm"
)

// Storage interface defines the contract for VFS storage backends
type Storage interface {
	CreateFile(node *Node, parentID *string) error
	DeleteFile(fileID string) error
	HardDeleteFile(fileID string) error
	RestoreFile(fileID string) error
	GetDeletedFiles() ([]FileRecord, error)
	RenameFile(fileID string, newName string) error
	GetFileContent(fileID string) (string, error)
	SetFileContent(fileID string, content string) error
	MoveFile(fileID string, newParentID *string) error
	CopyFileContent(sourceID string, destID string) error
	LoadVFSFromDB() (*VFS, error)
	SaveVFSToDB(vfs *VFS) error
	SaveDirectoryStates(vfs *VFS) error
}

// GormStorage implements Storage using GORM
type GormStorage struct {
	db       *gorm.DB
	filesDir string
}

// NewGormStorage creates a new GORM storage backend
func NewGormStorage(db *gorm.DB, filesDir string) *GormStorage {
	// Ensure files directory exists
	os.MkdirAll(filesDir, 0755)
	return &GormStorage{
		db:       db,
		filesDir: filesDir,
	}
}

// LoadVFSFromDB loads the entire file tree from database
func (s *GormStorage) LoadVFSFromDB() (*VFS, error) {
	// Load all non-deleted file records
	var records []FileRecord
	if err := s.db.Where("deleted = ?", false).Order("parent_id, is_dir DESC, name").Find(&records).Error; err != nil {
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
		storage:    s,
		DirSortBy:  make(map[string]SortBy),
		DirSortAsc: make(map[string]bool),
		DirCursor:  make(map[string]int),
	}

	// Load directory states
	var states []DirectoryState
	if err := s.db.Find(&states).Error; err == nil {
		for _, state := range states {
			vfs.DirSortBy[state.Path] = SortBy(state.SortBy)
			vfs.DirSortAsc[state.Path] = state.SortAsc
			vfs.DirCursor[state.Path] = state.CursorPos
		}
	}

	return vfs, nil
}

// SaveVFSToDB saves the entire file tree to database
func (s *GormStorage) SaveVFSToDB(vfs *VFS) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		return s.saveNodeRecursive(tx, vfs.Root, nil)
	})
}

func (s *GormStorage) saveNodeRecursive(tx *gorm.DB, node *Node, parentID *string) error {
	// Skip root node
	if node.ID == "root" {
		for _, child := range node.Children {
			if err := s.saveNodeRecursive(tx, child, nil); err != nil {
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
			if err := s.saveNodeRecursive(tx, child, &nodeID); err != nil {
				return err
			}
		}
	}

	return nil
}

// SaveDirectoryStates saves all directory states (sort, cursor)
func (s *GormStorage) SaveDirectoryStates(vfs *VFS) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
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
func (s *GormStorage) CreateFile(node *Node, parentID *string) error {
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

	if err := s.db.Create(&record).Error; err != nil {
		return err
	}

	// Create empty file if it's a .do file
	if !node.IsDir {
		filePath := filepath.Join(s.filesDir, node.ID+".do")
		if err := os.WriteFile(filePath, []byte(""), 0644); err != nil {
			return err
		}
	}

	return nil
}

// DeleteFile soft deletes a file by marking it as deleted
func (s *GormStorage) DeleteFile(fileID string) error {
	return s.db.Model(&FileRecord{}).
		Where("id = ?", fileID).
		Update("deleted", true).Error
}

// HardDeleteFile permanently deletes a file from database and filesystem
func (s *GormStorage) HardDeleteFile(fileID string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
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
			filePath := filepath.Join(s.filesDir, fileID+".do")
			os.Remove(filePath) // Ignore error if file doesn't exist
		}

		return nil
	})
}

// RenameFile renames a file in the database
func (s *GormStorage) RenameFile(fileID string, newName string) error {
	return s.db.Model(&FileRecord{}).
		Where("id = ?", fileID).
		Update("name", newName).Error
}

// GetFileContent reads file content from filesystem
func (s *GormStorage) GetFileContent(fileID string) (string, error) {
	filePath := filepath.Join(s.filesDir, fileID+".do")
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// SetFileContent writes file content to filesystem
func (s *GormStorage) SetFileContent(fileID string, content string) error {
	filePath := filepath.Join(s.filesDir, fileID+".do")

	// Update size in database
	size := int64(len(content))
	if err := s.db.Model(&FileRecord{}).
		Where("id = ?", fileID).
		Updates(map[string]interface{}{
			"size":       size,
			"updated_at": time.Now(),
		}).Error; err != nil {
		return err
	}

	return os.WriteFile(filePath, []byte(content), 0644)
}

// MoveFile moves a file to a new parent directory
func (s *GormStorage) MoveFile(fileID string, newParentID *string) error {
	return s.db.Model(&FileRecord{}).
		Where("id = ?", fileID).
		Update("parent_id", newParentID).Error
}

// RestoreFile undeletes a soft-deleted file
func (s *GormStorage) RestoreFile(fileID string) error {
	return s.db.Model(&FileRecord{}).
		Where("id = ?", fileID).
		Update("deleted", false).Error
}

// GetDeletedFiles returns all soft-deleted files
func (s *GormStorage) GetDeletedFiles() ([]FileRecord, error) {
	var records []FileRecord
	if err := s.db.Where("deleted = ?", true).Order("updated_at DESC").Find(&records).Error; err != nil {
		return nil, err
	}
	return records, nil
}

// CopyFileContent copies content from one file to another
func (s *GormStorage) CopyFileContent(sourceID string, destID string) error {
	content, err := s.GetFileContent(sourceID)
	if err != nil {
		return err
	}
	return s.SetFileContent(destID, content)
}

// generateUUID creates a simple timestamp-based UUID
func generateUUID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
