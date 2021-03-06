package auroraconfig

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/pkg/errors"
	"github.com/skatteetaten/ao/pkg/collections"
)

// FileNames holds an array of filenames
type FileNames []string

// GetApplicationDeploymentRefs filters application deployment references from FileNames
func (f FileNames) GetApplicationDeploymentRefs() []string {
	var filteredFiles []string
	for _, file := range f.WithoutExtension() {
		if strings.ContainsRune(file, '/') && !strings.Contains(file, "about") {
			filteredFiles = append(filteredFiles, file)
		}
	}
	sort.Strings(filteredFiles)
	return filteredFiles
}

// GetApplications gets unique application names from FileNames
func (f FileNames) GetApplications() []string {
	unique := collections.NewStringSet()
	for _, file := range f.WithoutExtension() {
		if !strings.ContainsRune(file, '/') && !strings.Contains(file, "about") {
			unique.Add(file)
		}
	}
	filteredFiles := unique.All()
	sort.Strings(filteredFiles)
	return filteredFiles
}

// GetEnvironments gets unique environment names from FileNames
func (f FileNames) GetEnvironments() []string {
	unique := collections.NewStringSet()
	for _, file := range f {
		if strings.ContainsRune(file, '/') && !strings.Contains(file, "about") {
			split := strings.Split(file, "/")
			unique.Add(split[0])
		}
	}
	filteredFiles := unique.All()
	sort.Strings(filteredFiles)
	return filteredFiles
}

// WithoutExtension gets the FileNames, stripped for extensions
func (f FileNames) WithoutExtension() []string {
	var withoutExt []string
	for _, file := range f {
		withoutExt = append(withoutExt, strings.TrimSuffix(file, filepath.Ext(file)))
	}
	return withoutExt
}

// Find returns a filename if it exists in the list
func (f FileNames) Find(name string) (string, error) {
	for _, fileName := range f {
		fileNameWithoutExtension := strings.TrimSuffix(fileName, filepath.Ext(fileName))
		if name == fileName || name == fileNameWithoutExtension {
			return fileName, nil
		}
	}
	return "", errors.Errorf("could not find %s in AuroraConfig", name)
}
