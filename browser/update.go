package browser

import (
	"fmt"
	"strings"
	"time"

	"github.com/s3bw/vfs"
)

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

					if err := m.storage.RenameFile(selected.ID, newName); err != nil {
						m.statusMessage = fmt.Sprintf("Error: %v", err)
					} else {
						selected.Name = newName
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
				finalName := processDateShortcut(m.newItemName)
				finalName = checkName(finalName, m.vfs.Current.Children)

				isFile := strings.HasSuffix(finalName, ".do")
				_, err := m.vfs.CreateFile(finalName, !isFile)
				if err != nil {
					m.statusMessage = fmt.Sprintf("Error: %v", err)
				} else {
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
			newSortBy = vfs.SortByName
		case "2":
			newSortBy = vfs.SortByCreated
		case "3":
			newSortBy = vfs.SortByModified
		case "4":
			newSortBy = vfs.SortBySize
		}

		if newSortBy == m.sortBy {
			m.sortAsc = !m.sortAsc
		} else {
			m.sortBy = newSortBy
			m.sortAsc = true
		}

		m.vfs.SaveSortState(m.sortBy, m.sortAsc)
		m.sortMode = false
		m.statusMessage = fmt.Sprintf("Sort by %s (%s)",
			[]string{"name", "created", "modified", "size"}[m.sortBy],
			map[bool]string{true: "asc", false: "desc"}[m.sortAsc])
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

	case "j", "tab", "down":
		items := m.getDisplayItems()
		maxCursor := len(items) - 1
		if m.cursor == -1 {
			m.cursor = 0
		} else if m.cursor < maxCursor {
			m.cursor++
		} else {
			m.cursor = 0
			if len(m.vfs.Stack) > 1 {
				m.cursor = -1
			}
		}
		if !m.searchMode {
			m.vfs.SaveCursorPosition(m.cursor)
		}
		m.lastKey = key

	case "k", "shifttab", "up":
		items := m.getDisplayItems()
		minIdx := 0
		if len(m.vfs.Stack) > 1 {
			minIdx = -1
		}

		if m.cursor > minIdx {
			m.cursor--
		} else {
			m.cursor = len(items) - 1
		}
		if !m.searchMode {
			m.vfs.SaveCursorPosition(m.cursor)
		}
		m.lastKey = key

	case "h", "left", "backspace":
		if m.vfs.GoBack() {
			if cursor, found := m.vfs.LoadCursorPosition(); found {
				m.cursor = cursor
			} else {
				m.cursor = 0
			}
			m.searchQuery = ""
			m.searchMode = false
			if sortBy, sortAsc, found := m.vfs.LoadSortState(); found {
				m.sortBy = sortBy
				m.sortAsc = sortAsc
			}
		}
		m.lastKey = key

	case "l", "enter":
		if m.cursor == -1 {
			if m.vfs.GoBack() {
				if cursor, found := m.vfs.LoadCursorPosition(); found {
					m.cursor = cursor
				} else {
					m.cursor = 0
				}
				m.searchQuery = ""
				m.searchMode = false
			}
			m.lastKey = key
			return m
		}

		items := m.getDisplayItems()
		if m.cursor >= 0 && m.cursor < len(items) {
			selected := items[m.cursor]
			if selected.IsDir {
				m.vfs.SaveCursorPosition(m.cursor)
				m.vfs.SaveSortState(m.sortBy, m.sortAsc)

				m.vfs.Enter(selected)

				if cursor, found := m.vfs.LoadCursorPosition(); found {
					m.cursor = cursor
				} else {
					m.cursor = 0
				}
				m.searchQuery = ""
				m.searchMode = false

				if sortBy, sortAsc, found := m.vfs.LoadSortState(); found {
					m.sortBy = sortBy
					m.sortAsc = sortAsc
				}
			} else {
				// Open file in $EDITOR
				if err := openInEditor(m.storage, selected); err != nil {
					m.statusMessage = fmt.Sprintf("Error opening file: %v", err)
				} else {
					m.statusMessage = fmt.Sprintf("Opened %s in editor", selected.Name)
				}
			}
		}
		m.lastKey = key

	case "d":
		if m.lastKey == "d" {
			items := m.getDisplayItems()
			if m.cursor >= 0 && m.cursor < len(items) {
				cutNode := items[m.cursor]

				// If there's already a file in clipboard, delete it first
				// (it was cut but never pasted, so it's orphaned)
				if m.clipboard != nil && m.clipboard.Node != nil {
					if err := m.storage.DeleteFile(m.clipboard.Node.ID); err != nil {
						m.statusMessage = fmt.Sprintf("Warning: could not delete previous cut file: %v", err)
					}
				}

				m.clipboard = &Clipboard{
					Node: cutNode,
					Path: m.vfs.GetPath(),
				}

				newChildren := []*vfs.Node{}
				for _, child := range m.vfs.Current.Children {
					if child.ID != cutNode.ID {
						newChildren = append(newChildren, child)
					}
				}
				m.vfs.Current.Children = newChildren

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
		if m.clipboard != nil {
			currentPath := m.vfs.GetPath()

			// Check if pasting into the same directory (copy) or different directory (move)
			if currentPath == m.clipboard.Path {
				// Same directory - create a copy with new ID
				nodeCopy := *m.clipboard.Node
				nodeCopy.Name = checkName(nodeCopy.Name, m.vfs.Current.Children)
				nodeCopy.ID = ""
				nodeCopy.CreatedAt = time.Now().Unix()
				nodeCopy.Modified = time.Now().Unix()

				var parentID *string
				if m.vfs.Current.ID != "root" {
					id := m.vfs.Current.ID
					parentID = &id
				}

				if err := m.storage.CreateFile(&nodeCopy, parentID); err != nil {
					m.statusMessage = fmt.Sprintf("Error: %v", err)
				} else {
					// Copy content if it's a file
					if !nodeCopy.IsDir {
						if err := m.storage.CopyFileContent(m.clipboard.Node.ID, nodeCopy.ID); err != nil {
							m.statusMessage = fmt.Sprintf("Error copying content: %v", err)
						}
					}
					m.vfs.Current.Children = append(m.vfs.Current.Children, &nodeCopy)
					m.statusMessage = fmt.Sprintf("Copied: %s (clipboard persists)", nodeCopy.Name)
				}
			} else {
				// Different directory - move the file (just update parent)
				var parentID *string
				if m.vfs.Current.ID != "root" {
					id := m.vfs.Current.ID
					parentID = &id
				}

				if err := m.storage.MoveFile(m.clipboard.Node.ID, parentID); err != nil {
					m.statusMessage = fmt.Sprintf("Error: %v", err)
				} else {
					// Add to current directory
					m.vfs.Current.Children = append(m.vfs.Current.Children, m.clipboard.Node)
					m.statusMessage = fmt.Sprintf("Moved: %s (clipboard cleared)", m.clipboard.Node.Name)
					// Clear clipboard after move
					m.clipboard = nil
				}
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
		m.newItemName = ""
		m.statusMessage = "New item name (add .do for file, no extension for folder):"
		m.lastKey = key

	case "r":
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

func (m Model) getDisplayItems() []*vfs.Node {
	var items []*vfs.Node

	if m.searchMode || m.searchQuery != "" {
		items = recursiveSearch(m.vfs.Root, m.searchQuery, "")
		filtered := []*vfs.Node{}
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
