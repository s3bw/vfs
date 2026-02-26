package browser

import (
	"fmt"
	"strings"
)

func (m Model) View() string {
	if m.showingFile {
		return m.renderFileView()
	}

	var b strings.Builder

	// Title
	title := fmt.Sprintf("%s%s Captain VFS Browser%s\n", Bold, Purple, Reset)
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

		// Format name with padding
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
	b.WriteString("j/k/↑/↓: move • h/←: back • l/Enter: open • /: search • n: new • r: rename\n")
	b.WriteString("dd: cut • p: paste • s[1-4]: sort • c[1-4]: color • q: quit")
	b.WriteString(fmt.Sprintf("%s\n", Reset))

	return b.String()
}

func (m Model) renderFileView() string {
	var b strings.Builder

	content, err := m.storage.GetFileContent(m.selectedFile.ID)
	if err != nil {
		fmt.Printf("Error reading file: %v\n", err)
	}

	b.WriteString(content)

	return b.String()
}
