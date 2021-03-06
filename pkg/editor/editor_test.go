package editor

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestEditor_Edit(t *testing.T) {

	const shouldFailError = "foo:shouldFail is not legal"

	// Should end test after two edit cycles
	cases := []struct {
		Content          string
		Edit1            string
		Edit2            string
		ExpectedModified string
		Errors           string
		Err              error
	}{
		// Should not save because no changes
		{"{}", "{}", "{}", "", "", errors.New(cancelMessage)},

		// Should save, has legal changes
		{"{}", `{"foo":"bar"}`, `{"foo":"bar"}`, `{"foo":"bar"}`, "", nil},
		{"{}", `{"foo":"shouldFail"}`, `{"foo":"bah"}`, `{"foo":"bah"}`, shouldFailError, nil},
	}

	fileEditor := NewEditor(nil)
	fileName := "foo.json"

	for _, tc := range cases {
		currentContent := fmt.Sprintf(editPattern, fileName, "", tc.Content)

		fileEditor.OnSave = func(modifiedContent string) error {

			js := make(map[string]string)
			err := json.Unmarshal([]byte(modifiedContent), &js)
			if err != nil {
				t.Error(err)
			}

			foo := js["foo"]
			if foo == "shouldFail" {
				return errors.New(shouldFailError)
			}

			assert.Equal(t, tc.ExpectedModified, modifiedContent)

			return nil
		}

		cycle := 0
		fileEditor.OpenEditor = func(tempFile string) error {

			data, err := ioutil.ReadFile(tempFile)
			if err != nil {
				t.Error(err)
			}

			assert.Equal(t, currentContent, string(data))

			messages := addErrorMessage(tc.Errors)
			edit := tc.Edit1
			if cycle == 1 {
				edit = tc.Edit2
			}

			afterEditContent := fmt.Sprintf(editPattern, fileName, messages, edit)
			currentContent = afterEditContent

			ioutil.WriteFile(tempFile, []byte(afterEditContent), 0700)

			cycle++

			return nil
		}

		err := fileEditor.Edit(tc.Content, fileName)
		if err != nil {
			assert.EqualError(t, err, tc.Err.Error())
		}
	}
}

func TestAddComments(t *testing.T) {

	messages := "FATAL ERROR"

	expected := "##\n## ERROR:\n## FATAL ERROR\n##\n"
	errs := addErrorMessage(messages)

	assert.Equal(t, expected, errs)
}

func TestStripComments(t *testing.T) {

	content := `## Name: foo.json
{}`

	noComments := stripComments(content)
	assert.Equal(t, "{}", noComments)
}
