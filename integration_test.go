package vimtea

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEditorIntegration(t *testing.T) {
	initialContent := "Hello, world!"
	editor := NewEditor(WithContent(initialContent))

	assert.Equal(t, ModeNormal, editor.GetMode(), "Initial mode should be Normal")

	buffer := editor.GetBuffer()
	assert.Equal(t, initialContent, buffer.Text(), "Buffer content should match initial content")

	editor.SetMode(ModeInsert)
	assert.Equal(t, ModeInsert, editor.GetMode(), "Mode should be Insert after setting")

	model := editor.(*editorModel)
	model.buffer.insertAt(0, 13, " This is a test.")

	expectedContent := "Hello, world! This is a test."
	assert.Equal(t, expectedContent, buffer.Text(), "Buffer content should match expected after insertion")

	editor.SetMode(ModeNormal)
	assert.Equal(t, ModeNormal, editor.GetMode(), "Mode should be Normal after setting")

	testStatusMsg := "Test status"
	cmd := editor.SetStatusMessage(testStatusMsg)
	cmd()

	assert.Equal(t, testStatusMsg, model.statusMessage, "Status message should match set message")
}

func TestViewportIntegration(t *testing.T) {
	var content string
	for i := 0; i < 30; i++ {
		content += "Line " + string(rune('A'+i%26)) + "\n"
	}

	editor := NewEditor(WithContent(content))
	model := editor.(*editorModel)

	model.width = 80
	model.height = 20
	model.viewport.Width = 80
	model.viewport.Height = 20

	model.cursor = newCursor(25, 0)

	model.ensureCursorVisible()

	assert.GreaterOrEqual(t, model.cursor.Row, model.viewport.YOffset,
		"Cursor row should be within or after viewport start")
	assert.Less(t, model.cursor.Row, model.viewport.YOffset+model.viewport.Height,
		"Cursor row should be within viewport end")
}

func TestKeyBindingsIntegration(t *testing.T) {
	editor := NewEditor()
	model := editor.(*editorModel)

	testBindingCalled := false
	editor.AddBinding(KeyBinding{
		Key:         "ctrl+t",
		Mode:        ModeNormal,
		Description: "Test binding",
		Handler: func(b Buffer) tea.Cmd {
			testBindingCalled = true
			return nil
		},
	})

	iBinding := model.registry.FindExact("i", ModeNormal)
	assert.NotNil(t, iBinding, "Default binding for 'i' should exist")

	ctrlTBinding := model.registry.FindExact("ctrl+t", ModeNormal)
	require.NotNil(t, ctrlTBinding, "Custom binding for 'ctrl+t' should exist")

	_ = ctrlTBinding.Command(model)

	assert.True(t, testBindingCalled, "Custom binding command should have been executed")
}

func TestCommandsIntegration(t *testing.T) {
	editor := NewEditor()
	model := editor.(*editorModel)

	commandCalled := false
	editor.AddCommand("test", func(b Buffer, args []string) tea.Cmd {
		commandCalled = true
		if len(args) > 0 && args[0] == "arg" {
			return nil
		}
		return nil
	})

	model.commandBuffer = "test arg"
	model.Update(CommandMsg{Command: "test"})

	assert.True(t, commandCalled, "Command should have been executed")
}
