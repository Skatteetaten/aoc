package service

import (
	"ao/pkg/auroraconfig"
	"ao/pkg/client"
)

// GetApplications returns list of applications
func GetApplications(apiClient client.AuroraConfigClient, pattern string, excludes []string) ([]string, error) {
	filenames, err := apiClient.GetFileNames()
	if err != nil {
		return nil, err
	}

	applications, err := auroraconfig.GetApplicationRefs(filenames, pattern, excludes)
	if err != nil {
		return nil, err
	}

	return applications, nil
}
