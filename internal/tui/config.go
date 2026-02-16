// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package tui

import (
	"reflect"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"grimm.is/flywall/internal/config"
)

type ConfigModel struct {
	Backend Backend
	List    list.Model
	Form    *huh.Form

	// State
	Editing       bool
	ActiveSection string
	Config        *config.Config
	LastError     error

	Width  int
	Height int
}

type sectionItem struct {
	title string
	desc  string
	field string // Name of the field in config.Config
}

func (i sectionItem) Title() string       { return i.title }
func (i sectionItem) Description() string { return i.desc }
func (i sectionItem) FilterValue() string { return i.title }

func NewConfigModel(backend Backend) ConfigModel {
	items := []list.Item{
		sectionItem{title: "API Settings", desc: "Manage HTTP/HTTPS API configuration", field: "API"},
		sectionItem{title: "Web Server", desc: "Web UI and Proxy settings", field: "Web"},
		sectionItem{title: "Features", desc: "Enable/Disable core features", field: "Features"},
		sectionItem{title: "System Tuning", desc: "System identity and sysctl profiles", field: "System"},
	}

	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Configuration Sections"
	l.Styles.Title = StyleTitle

	return ConfigModel{
		Backend: backend,
		List:    l,
	}
}

type ConfigLoadError struct {
	Err error
}

func (m ConfigModel) Init() tea.Cmd {
	return func() tea.Msg {
		cfg, err := m.Backend.GetConfig()
		if err != nil {
			DebugLog("Config Init Failed: %v", err)
			return ConfigLoadError{Err: err}
		}
		return cfg
	}
}

type ConfigSaveSuccess struct{}

func (m ConfigModel) Update(msg tea.Msg) (ConfigModel, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case BackendError:
		m.LastError = msg.Err
		return m, nil

	case ConfigSaveSuccess:
		m.LastError = nil
		// Maybe show a toast?
		return m, nil

	case *config.Config:
		m.Config = msg
		m.LastError = nil
		return m, nil

	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		m.List.SetSize(msg.Width-4, msg.Height-4)
		return m, nil

	case tea.KeyMsg:
		if m.Editing {
			if msg.Type == tea.KeyEsc {
				m.Editing = false
				m.Form = nil
				return m, nil
			}

			// Update form
			var formCmd tea.Cmd
			form, formCmd := m.Form.Update(msg)
			if f, ok := form.(*huh.Form); ok {
				m.Form = f
			}

			if m.Form.State == huh.StateCompleted {
				m.Editing = false
				m.Form = nil
				// Save config back to backend
				return m, func() tea.Msg {
					err := m.Backend.ApplyConfig(m.Config)
					if err != nil {
						return BackendError{Err: err}
					}
					return ConfigSaveSuccess{}
				}
			}

			return m, formCmd
		}

		switch msg.String() {
		case "enter":
			if m.Config == nil {
				return m, nil
			}

			// Enter critical editing mode
			item := m.List.SelectedItem()

			selected, ok := item.(sectionItem)
			if ok {
				m.ActiveSection = selected.field
				m.Editing = true

				// Reflection magic to get the field
				val := reflect.ValueOf(m.Config).Elem()
				fieldVal := val.FieldByName(selected.field)

				if !fieldVal.IsValid() || fieldVal.IsNil() {
					// Initialize if nil? Or show error?
					m.Editing = false
					return m, nil
				}

				// AutoForm expects a pointer to a struct
				// fieldVal is likely a pointer (e.g. *APIConfig)
				m.Form = AutoForm(fieldVal.Interface())
				m.Form.Init()
			}
			return m, nil
		}
	}

	if !m.Editing {
		m.List, cmd = m.List.Update(msg)
	}

	return m, cmd
}

func (m ConfigModel) View() string {
	if m.Editing && m.Form != nil {
		return lipgloss.JoinVertical(lipgloss.Left,
			StyleHeader.Render("EDITING: "+m.ActiveSection),
			StyleCard.Render(m.Form.View()),
			StyleSubtitle.Render("Esc to Cancel, Enter to Save"),
		)
	}

	if m.Config == nil {
		if m.LastError != nil {
			return lipgloss.JoinVertical(lipgloss.Left,
				StyleHeader.Render("CONFIG EXPLORER"),
				StyleStatusBad.Render("Failed to load configuration:"),
				StyleCard.Render(m.LastError.Error()),
				StyleSubtitle.Render("Check connectivity or try again."),
			)
		}
		return lipgloss.JoinVertical(lipgloss.Left,
			StyleHeader.Render("CONFIG EXPLORER"),
			StyleSubtitle.Render("Loading configuration..."),
		)
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		StyleHeader.Render("CONFIG EXPLORER"),
		StyleSubtitle.Render("Select a section to edit"),
		StyleCard.Render(m.List.View()),
	)
}
