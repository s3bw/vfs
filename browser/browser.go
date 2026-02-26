package browser

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/s3bw/vfs"
	"gorm.io/gorm"
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
	IconFolder     = "\uf114"
	IconFolderOpen = "\uf115"
	IconText       = "\uf15c"
	IconFile       = "\uf15b"
)

// Clipboard for cut/paste
type Clipboard struct {
	Node *vfs.Node
	Path string
}

// Model represents the application state
type Model struct {
	vfs           *vfs.VFS
	storage       *vfs.GormStorage
	cursor        int
	showingFile   bool
	selectedFile  *vfs.Node
	quitting      bool
	searchMode    bool
	searchQuery   string
	sortBy        vfs.SortBy
	sortAsc       bool
	clipboard     *Clipboard
	statusMessage string
	colorMode     bool
	sortMode      bool
	newItemMode   bool
	newItemName   string
	renameMode    bool
	renameText    string
	lastKey       string
}

// RunBrowser launches the VFS browser with the given database connection
func RunBrowser(db *gorm.DB, filesDir string) error {
	storage := vfs.NewGormStorage(db, filesDir)
	vfsTree, err := storage.LoadVFSFromDB()
	if err != nil {
		return err
	}

	model := Model{
		vfs:     vfsTree,
		storage: storage,
		cursor:  0,
		sortBy:  vfs.SortByName,
		sortAsc: true,
	}

	// Setup terminal
	setupTerminal()
	defer restoreTerminal()

	// Setup: alternate screen, hide cursor
	fmt.Print("\033[?1049h\033[?25l")
	defer fmt.Print("\033[?1049l\033[?25h")

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

	// Before exiting, delete any files still in clipboard (cut but not pasted)
	if model.clipboard != nil && model.clipboard.Node != nil {
		if err := model.storage.DeleteFile(model.clipboard.Node.ID); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not delete cut file: %v\n", err)
		}
	}

	// Save on exit
	if err := model.storage.SaveVFSToDB(model.vfs); err != nil {
		return fmt.Errorf("error saving: %v", err)
	}

	if err := model.storage.SaveDirectoryStates(model.vfs); err != nil {
		return fmt.Errorf("error saving states: %v", err)
	}

	fmt.Println("Goodbye!")
	return nil
}

func setupTerminal() {
	cmd := exec.Command("stty", "-echo", "cbreak")
	cmd.Stdin = os.Stdin
	_ = cmd.Run()
}

func restoreTerminal() {
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
	case 9:
		return "tab"
	case 'q', 'Q':
		return "q"
	case 'h':
		return "h"
	case 'j':
		return "j"
	case 'k':
		return "k"
	case 'l':
		return "l"
	case 'd':
		return "d"
	case 'p', 'P':
		return "p"
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
	case 13, 10:
		return "enter"
	case 32:
		return "space"
	case 127:
		return "backspace"
	case 3:
		return "q"
	case 27: // ESC - might be arrow key
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
			case 90:
				return "shifttab"
			}
		}
	default:
		if b >= 32 && b <= 126 {
			return string(b)
		}
	}

	return ""
}

func checkName(name string, children []*vfs.Node) string {
	for _, child := range children {
		if child.Name == name {
			baseName := name
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

func processDateShortcut(text string) string {
	// For now, just return text as-is
	// You can implement /d -> current date replacement if needed
	return text
}
