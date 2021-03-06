package cmd

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/skatteetaten/ao/pkg/auroraconfig"
	"github.com/skatteetaten/ao/pkg/editor"
	"github.com/spf13/cobra"
)

const editLong = `Edit a single file in the current AuroraConfig.`

const exampleEdit = `  Given the following AuroraConfig:
    - about.json
    - foobar.json
    - bar.json
    - foo/about.json
    - foo/bar.json
    - foo/foobar.json

  # Exact matching: will open foo/bar.json in editor
  ao edit foo/bar

  # Fuzzy matching: will open foo/foobar.json in editor
  ao edit fofoba
`

var editCmd = &cobra.Command{
	Use:         "edit [env/]file",
	Short:       "Edit a single file in the AuroraConfig repository",
	Long:        editLong,
	Annotations: map[string]string{"type": "remote"},
	Example:     exampleEdit,
	RunE:        EditFile,
}

func init() {
	RootCmd.AddCommand(editCmd)
}

// EditFile is the main method for the `edit` cli command
func EditFile(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return cmd.Usage()
	}

	fileNames, err := DefaultAPIClient.GetFileNames()
	if err != nil {
		return err
	}

	search := args[0]
	if len(args) == 2 {
		search = fmt.Sprintf("%s/%s", args[0], args[1])
	}

	matches := auroraconfig.FindMatches(search, fileNames, true)
	if len(matches) == 0 {
		return errors.Errorf("No matches for %s", search)
	} else if len(matches) > 1 {
		return errors.Errorf("Search matched more than one file. Search must be more specific.\n%v", matches)
	}

	fileName := matches[0]
	file, eTag, err := DefaultAPIClient.GetAuroraConfigFile(fileName)
	if err != nil {
		return err
	}

	fileEditor := editor.NewEditor(func(modified string) error {
		file.Contents = modified

		// Save config file (Gobo)
		if err = DefaultAPIClient.UpdateAuroraConfigFile(file, eTag); err != nil {
			return err
		}
		return nil
	})

	err = fileEditor.Edit(string(file.Contents), file.Name)
	if err != nil {
		return err
	}

	fmt.Println(fileName, "edited")
	return nil
}
