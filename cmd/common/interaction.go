package common

import (
	"fmt"
	"github.com/pkg/errors"
	"github.com/skatteetaten/ao/pkg/fuzzy"
	"github.com/skatteetaten/ao/pkg/prompt"
	"sort"
)

func SelectOne(args []string, items []string, withSuffix bool) (string, error) {
	search := args[0]
	if len(args) == 2 {
		search = fmt.Sprintf("%s/%s", args[0], args[1])
	}

	matches := fuzzy.FindMatches(search, items, withSuffix)
	if len(matches) == 0 {
		return "", errors.Errorf("No matches for %s", search)
	}

	sort.Strings(matches)

	selected := matches[0]
	if len(matches) > 1 {
		selected = prompt.Select("Select one", matches)
	}

	if selected == "" {
		return "", errors.New("Nothing selected")
	}

	return selected, nil
}