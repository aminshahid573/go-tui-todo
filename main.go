package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

var (
	docStyle = lipgloss.NewStyle().Margin(1, 2)

	cursorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))

	cursorLineStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("57")).
			Foreground(lipgloss.Color("230"))

	placeholderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("238"))

	endOfBufferStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("235"))

	focusedPlaceholderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("99"))

	focusedBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("238"))

	blurredBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.HiddenBorder())

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			MarginTop(1)

	previewTitleStyle = func() lipgloss.Style {
		b := lipgloss.RoundedBorder()
		b.Right = "├"
		return lipgloss.NewStyle().
			BorderStyle(b).
			Padding(0, 1)
	}()

	previewInfoStyle = func() lipgloss.Style {
		b := lipgloss.RoundedBorder()
		b.Left = "┤"
		return previewTitleStyle.BorderStyle(b)
	}()

	appTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("255")).
			Background(lipgloss.Color("99")).
			Padding(0, 1).
			Margin(1, 2)

	todoTitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFDF5")).
			Background(lipgloss.Color("#25A065")).
			Padding(0, 1)

	statusMessageStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#04B575", Dark: "#04B575"}).
				Render
)

type item struct {
	title, desc string
}

func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.desc }
func (i item) FilterValue() string { return i.title }

type todoItem struct {
	filename string
	modTime  string
}

func (i todoItem) Title() string       { return i.filename }
func (i todoItem) Description() string { return i.modTime }
func (i todoItem) FilterValue() string { return i.filename }

type viewState int

const (
	listView viewState = iota
	createTodoView
	editorView
	previewView
	todoListView
)

type delegateKeyMap struct {
	choose key.Binding
	remove key.Binding
}

func newDelegateKeyMap() *delegateKeyMap {
	return &delegateKeyMap{
		choose: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "open"),
		),
		remove: key.NewBinding(
			key.WithKeys("x", "backspace"),
			key.WithHelp("x", "delete"),
		),
	}
}

type todoListKeyMap struct {
	back key.Binding
}

func newTodoListKeyMap() *todoListKeyMap {
	return &todoListKeyMap{
		back: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back"),
		),
	}
}

type model struct {
	mainList     list.Model
	todoList     list.Model
	textInput    textinput.Model
	editor       textarea.Model
	viewport     viewport.Model
	state        viewState
	currentFile  string
	width        int
	height       int
	ready        bool
	delegateKeys *delegateKeyMap
	todoListKeys *todoListKeyMap
}

func newTextarea() textarea.Model {
	t := textarea.New()
	t.Prompt = ""
	t.Placeholder = "Start typing your todo..."
	t.ShowLineNumbers = true
	t.Cursor.Style = cursorStyle
	t.FocusedStyle.Placeholder = focusedPlaceholderStyle
	t.BlurredStyle.Placeholder = placeholderStyle
	t.FocusedStyle.CursorLine = cursorLineStyle
	t.FocusedStyle.Base = focusedBorderStyle
	t.BlurredStyle.Base = blurredBorderStyle
	t.FocusedStyle.EndOfBuffer = endOfBufferStyle
	t.BlurredStyle.EndOfBuffer = endOfBufferStyle
	t.KeyMap.DeleteWordBackward.SetEnabled(false)
	t.Focus()
	return t
}

func (m *model) loadTodoFiles() []list.Item {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return []list.Item{}
	}

	todoDir := filepath.Join(homeDir, "todo")

	// Create directory if it doesn't exist
	if err := os.MkdirAll(todoDir, 0755); err != nil {
		return []list.Item{}
	}

	files, err := os.ReadDir(todoDir)
	if err != nil {
		return []list.Item{}
	}

	var items []list.Item
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".md") {
			filePath := filepath.Join(todoDir, file.Name())
			fileInfo, err := os.Stat(filePath)
			modTimeStr := ""
			if err == nil {
				modTimeStr = "Modified: " + fileInfo.ModTime().Format("Jan 02, 2006 3:04 PM")
			}

			items = append(items, todoItem{
				filename: file.Name(),
				modTime:  modTimeStr,
			})
		}
	}

	return items
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

		// Handle different views
		switch m.state {
		case listView:
			if msg.String() == "enter" {
				// Get selected item
				selected := m.mainList.SelectedItem()
				if selected != nil {
					selectedItem := selected.(item)
					if selectedItem.title == "Create Todo" {
						// Switch to create todo view
						m.state = createTodoView
						m.textInput.Focus()
						return m, textinput.Blink
					} else if selectedItem.title == "List All Todos" {
						// Load todos and switch to todo list view
						items := m.loadTodoFiles()
						delegate := list.NewDefaultDelegate()
						m.todoList = list.New(items, delegate, 0, 0)
						m.todoList.Title = "All Todos"
						m.todoList.Styles.Title = todoTitleStyle

						h, v := docStyle.GetFrameSize()
						m.todoList.SetSize(m.width-h, m.height-v)

						m.state = todoListView
						return m, nil
					}
				}
			}
		case createTodoView:
			switch msg.String() {
			case "enter":
				// Save the filename and switch to editor
				fileName := m.textInput.Value()
				if fileName != "" {
					// Remove any file extension if user typed one
					fileName = strings.TrimSuffix(fileName, filepath.Ext(fileName))

					m.currentFile = fileName
					m.textInput.SetValue("")
					m.state = editorView
					m.editor.Focus()
					return m, textarea.Blink
				}
				return m, nil
			case "esc":
				// Cancel and return to list
				m.textInput.SetValue("")
				m.state = listView
				return m, nil
			}
		case editorView:
			switch msg.String() {
			case "esc":
				// Cancel and return to list without saving
				m.editor.Reset()
				m.state = listView
				return m, nil
			case "ctrl+s":
				// Save file and continue editing
				if err := m.saveFile(); err != nil {
					fmt.Println("Error saving file:", err)
				}
				return m, nil
			case "ctrl+d":
				// Save file and return to list
				if err := m.saveFile(); err != nil {
					fmt.Println("Error saving file:", err)
				}
				m.editor.Reset()
				m.state = listView
				return m, nil
			case "ctrl+p":
				// Switch to preview
				m.state = previewView
				m.ready = false
				return m, nil
			}
		case previewView:
			switch msg.String() {
			case "esc", "q", "ctrl+p":
				// Return to editor
				m.state = editorView
				m.editor.Focus()
				return m, textarea.Blink
			}
		case todoListView:
			switch msg.String() {
			case "esc":
				// Return to main list
				m.state = listView
				return m, nil
			case "ctrl+p":
				// Open selected todo file in preview mode
				selected := m.todoList.SelectedItem()
				if selected != nil {
					selectedTodo := selected.(todoItem)
					fileName := strings.TrimSuffix(selectedTodo.filename, ".md")

					// Load the file content
					homeDir, err := os.UserHomeDir()
					if err == nil {
						filePath := filepath.Join(homeDir, "todo", selectedTodo.filename)
						content, err := os.ReadFile(filePath)
						if err == nil {
							m.currentFile = fileName
							m.editor.SetValue(string(content))
							m.state = previewView
							m.ready = false
							return m, nil
						}
					}
				}
				return m, nil
			case "enter":
				// Open selected todo file
				selected := m.todoList.SelectedItem()
				if selected != nil {
					selectedTodo := selected.(todoItem)
					fileName := strings.TrimSuffix(selectedTodo.filename, ".md")

					// Load the file content
					homeDir, err := os.UserHomeDir()
					if err == nil {
						filePath := filepath.Join(homeDir, "todo", selectedTodo.filename)
						content, err := os.ReadFile(filePath)
						if err == nil {
							m.currentFile = fileName
							m.editor.SetValue(string(content))
							m.state = editorView
							m.editor.Focus()
							return m, textarea.Blink
						}
					}
				}
				return m, nil
			case "x", "backspace":
				// Delete selected todo file
				selected := m.todoList.SelectedItem()
				if selected != nil {
					selectedTodo := selected.(todoItem)
					homeDir, err := os.UserHomeDir()
					if err == nil {
						filePath := filepath.Join(homeDir, "todo", selectedTodo.filename)
						os.Remove(filePath)

						// Reload the list
						items := m.loadTodoFiles()
						cmd := m.todoList.SetItems(items)
						statusCmd := m.todoList.NewStatusMessage(statusMessageStyle("Deleted " + selectedTodo.filename))
						return m, tea.Batch(cmd, statusCmd)
					}
				}
				return m, nil
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		h, v := docStyle.GetFrameSize()
		m.mainList.SetSize(msg.Width-h, msg.Height-v)

		if m.state == todoListView {
			m.todoList.SetSize(msg.Width-h, msg.Height-v)
		}

		// Size the editor to fit the screen (accounting for help text)
		m.editor.SetWidth(msg.Width - h)
		titleHeight := lipgloss.Height(appTitleStyle.Render("Todo App"))
		m.editor.SetHeight(msg.Height - v - 6 - titleHeight)

		// Handle viewport sizing for preview
		if m.state == previewView {
			// Account for app title, pager header, footer, help text and margins
			appTitleHeight := lipgloss.Height(appTitleStyle.Render("Todo App"))
			headerHeight := lipgloss.Height(m.previewHeaderView())
			footerHeight := lipgloss.Height(m.previewFooterView())
			helpHeight := 2    // Help text height
			marginsHeight := 4 // Top and bottom margins (1, 2)

			verticalMarginHeight := appTitleHeight + headerHeight + footerHeight + helpHeight + marginsHeight

			if !m.ready {
				// Render markdown content
				content := m.editor.Value()
				if content == "" {
					content = "# Empty Document\n\nStart typing to see content here."
				}
				rendered, err := glamour.Render(content, "dark")
				if err != nil {
					rendered = content
				}

				m.viewport = viewport.New(msg.Width-h, msg.Height-verticalMarginHeight)
				m.viewport.YPosition = headerHeight
				m.viewport.SetContent(rendered)
				m.ready = true
			} else {
				m.viewport.Width = msg.Width - h
				m.viewport.Height = msg.Height - verticalMarginHeight
			}
		}
	}

	// Update the appropriate component based on state
	switch m.state {
	case listView:
		m.mainList, cmd = m.mainList.Update(msg)
	case createTodoView:
		m.textInput, cmd = m.textInput.Update(msg)
	case editorView:
		m.editor, cmd = m.editor.Update(msg)
	case previewView:
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	case todoListView:
		m.todoList, cmd = m.todoList.Update(msg)
	}

	if len(cmds) > 0 {
		return m, tea.Batch(cmds...)
	}
	return m, cmd
}

func (m *model) saveFile() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	todoDir := filepath.Join(homeDir, "todo")

	// Create the todo directory if it doesn't exist
	if err := os.MkdirAll(todoDir, 0755); err != nil {
		return err
	}

	filePath := filepath.Join(todoDir, m.currentFile+".md")

	return os.WriteFile(filePath, []byte(m.editor.Value()), 0644)
}

func (m model) previewHeaderView() string {
	title := previewTitleStyle.Render(m.currentFile + ".md")
	line := strings.Repeat("─", max(0, m.viewport.Width-lipgloss.Width(title)))
	return lipgloss.JoinHorizontal(lipgloss.Center, title, line)
}

func (m model) previewFooterView() string {
	info := previewInfoStyle.Render(fmt.Sprintf("%3.f%%", m.viewport.ScrollPercent()*100))
	line := strings.Repeat("─", max(0, m.viewport.Width-lipgloss.Width(info)))
	return lipgloss.JoinHorizontal(lipgloss.Center, line, info)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (m model) View() string {
	switch m.state {
	case listView:
		return docStyle.Render(m.mainList.View())
	case createTodoView:
		content := fmt.Sprintf(
			"Enter file name:\n\n%s",
			m.textInput.View(),
		)
		help := helpStyle.Render("(enter to continue, esc to cancel)")
		return docStyle.Render(content + "\n\n" + help)
	case editorView:
		appTitle := appTitleStyle.Render("Todo App")
		header := fmt.Sprintf("\n  Editing: %s.md\n\n", m.currentFile)
		help := helpStyle.Render("ctrl+p: preview | esc: cancel | ctrl+d: save & exit | ctrl+s: save")
		content := appTitle + header + m.editor.View() + "\n\n" + help
		return docStyle.Render(content)
	case previewView:
		if !m.ready {
			return "\n  Initializing preview..."
		}
		appTitle := appTitleStyle.Render("Todo App")
		previewContent := fmt.Sprintf("%s\n%s\n%s", m.previewHeaderView(), m.viewport.View(), m.previewFooterView())
		help := helpStyle.Render("↑/↓: scroll | g/G: top/bottom | ctrl+u/d: half page | ctrl+p/q/esc: back to editor")
		return docStyle.Render(appTitle + "\n" + previewContent + "\n" + help)
	case todoListView:
		return docStyle.Render(m.todoList.View())
	default:
		return ""
	}
}

func main() {
	items := []list.Item{
		item{title: "Create Todo", desc: "add a new todo item"},
		item{title: "List All Todos", desc: "see all your todos"},
	}

	// Initialize text input
	ti := textinput.New()
	ti.Placeholder = "what would you like to call this file"
	ti.CharLimit = 156
	ti.Width = 50

	delegateKeys := newDelegateKeyMap()
	todoListKeys := newTodoListKeyMap()

	m := model{
		mainList:     list.New(items, list.NewDefaultDelegate(), 0, 0),
		textInput:    ti,
		editor:       newTextarea(),
		state:        listView,
		delegateKeys: delegateKeys,
		todoListKeys: todoListKeys,
	}
	m.mainList.Title = "Todo App"

	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())

	if _, err := p.Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
}
