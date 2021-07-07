package client

import (
	"encoding/json"
	"fmt"
	"github.com/sirupsen/logrus"
	"net/http"

	"github.com/pkg/errors"
	"github.com/skatteetaten/ao/pkg/auroraconfig"
)

// AuroraConfigClient is a an internal client facade for external aurora configuration API calls
type AuroraConfigClient interface {
	Doer
	GetFileNames() (auroraconfig.FileNames, error)
	GetAuroraConfig() (*auroraconfig.AuroraConfig, error)
	GetAuroraConfigNames() (*auroraconfig.Names, error)
	PutAuroraConfig(endpoint string, payload []byte) (string, error)
	ValidateAuroraConfig(ac *auroraconfig.AuroraConfig, fullValidation bool) (string, error)
	GetAuroraConfigFile(fileName string) (*auroraconfig.File, string, error)
}

// GetAuroraConfig gets an aurora config via API calls
func (api *APIClient) GetAuroraConfig() (*auroraconfig.AuroraConfig, error) {
	endpoint := fmt.Sprintf("/auroraconfig/%s", api.Affiliation)

	response, err := api.Do(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	var ac auroraconfig.AuroraConfig
	err = response.ParseFirstItem(&ac)
	if err != nil {
		return nil, errors.Wrap(err, "aurora config")
	}

	return &ac, nil
}

// GetAuroraConfigNames gets Aurora configuration names via API calls
func (api *APIClient) GetAuroraConfigNames() (*auroraconfig.Names, error) {
	// Deprecated: Remove when it is fully replaced by graphql
	endpoint := fmt.Sprintf("/auroraconfignames")

	response, err := api.Do(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	var acn auroraconfig.Names
	err = response.ParseItems(&acn)
	if err != nil {
		return nil, errors.Wrap(err, "aurora config names")
	}
	return &acn, nil
}

// PutAuroraConfig sets aurora configuration via API calls
func (api *APIClient) PutAuroraConfig(endpoint string, payload []byte) (string, error) {

	response, err := api.Do(http.MethodPut, endpoint, payload)
	if err != nil {
		return "", err
	}

	if !response.Success {
		return "", response.Error()
	}

	//for validation you can also have warnings
	warnings, err := response.toWarningResponse()
	if err != nil {
		return "", err
	}

	if warnings != nil {
		return formatWarnings(warnings), nil
	}

	return "", nil

}

// ValidateAuroraConfig validates an aurora configuration via API calls
func (api *APIClient) ValidateAuroraConfig(ac *auroraconfig.AuroraConfig, fullValidation bool) (string, error) {
	resourceValidation := "false"
	if fullValidation {
		resourceValidation = "true"
	}
	endpoint := fmt.Sprintf("/auroraconfig/%s/validate?resourceValidation=%s", api.Affiliation, resourceValidation)

	payload, err := json.Marshal(ac)
	if err != nil {
		return "", err
	}
	return api.PutAuroraConfig(endpoint, payload)

}

// ValidateRemoteAuroraConfig validates a remote aurora configuration via API calls
func (api *APIClient) ValidateRemoteAuroraConfig(fullValidation bool) (string, error) {
	resourceValidation := "false"
	if fullValidation {
		resourceValidation = "true"
	}
	endpoint := fmt.Sprintf("/auroraconfig/%s/validate?resourceValidation=%s&mergeWithRemoteConfig=true", api.Affiliation, resourceValidation)

	return api.PutAuroraConfig(endpoint, nil)
}

func formatWarnings(warnings []string) string {
	var status string

	messages := warnings
	for i, message := range messages {
		status += message
		if i != len(messages)-1 {
			status += "\n\n"
		}
	}

	return status
}

// GetAuroraConfigFile gets an aurora configuration via API calls
func (api *APIClient) GetAuroraConfigFile(fileName string) (*auroraconfig.File, string, error) {
	endpoint := fmt.Sprintf("/auroraconfig/%s/%s", api.Affiliation, fileName)

	bundle, err := api.DoWithHeader(http.MethodGet, endpoint, nil, nil)
	if err != nil || bundle == nil {
		return nil, "", err
	}

	if !bundle.BooberResponse.Success {
		return nil, "", errors.New("Failed getting file " + fileName)
	}

	var file auroraconfig.File
	err = bundle.BooberResponse.ParseFirstItem(&file)
	if err != nil {
		return nil, "", errors.Wrap(err, "aurora config file")
	}

	eTag := bundle.HTTPResponse.Header.Get("ETag")
	logrus.Debugf("GetAuroraConfigFile: Got ETag: %s", eTag)

	return &file, eTag, nil
}
