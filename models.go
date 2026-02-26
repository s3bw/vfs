package vfs

import "time"

// FileRecord represents a file or directory in the database
type FileRecord struct {
	ID        string    `gorm:"primaryKey"`
	Name      string    `gorm:"not null"`
	ParentID  *string   `gorm:"index"` // null for root-level items
	IsDir     bool      `gorm:"not null"`
	Color     string    `gorm:"default:''"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
	Size      int64     `gorm:"default:0"`
	Deleted   bool      `gorm:"default:false"`
}

// UserPreference stores user preferences
type UserPreference struct {
	ID    uint   `gorm:"primaryKey"`
	Key   string `gorm:"uniqueIndex;not null"`
	Value string
}

// DirectoryState stores directory-specific state (sort, cursor)
type DirectoryState struct {
	ID        uint   `gorm:"primaryKey"`
	Path      string `gorm:"uniqueIndex;not null"`
	SortBy    int    `gorm:"default:0"` // 0=name, 1=created, 2=modified, 3=size
	SortAsc   bool   `gorm:"default:true"`
	CursorPos int    `gorm:"default:0"`
}
