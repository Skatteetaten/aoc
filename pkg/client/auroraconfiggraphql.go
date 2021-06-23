package client

import (
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/skatteetaten/ao/pkg/auroraconfig"
	"github.com/skatteetaten/graphql"
	"net/http"
)

// AuroraConfigFileValidationResponse is core of response from the graphql "createAuroraConfigFile" and "updateAuroraConfigFile"
type AuroraConfigFileValidationResponse struct {
	Message string `json:"message"`
	Success bool   `json:"success"`
}

const createAuroraConfigFileRequestString = `mutation createAuroraConfigFile($newAuroraConfigFileInput: NewAuroraConfigFileInput!){
  createAuroraConfigFile(input: $newAuroraConfigFileInput)
  {
    message
    success
  }
}`

// NewAuroraConfigFileInput is input to the graphql createAuroraConfigFile interface
type NewAuroraConfigFileInput struct {
	AuroraConfigName      string `json:"auroraConfigName"`
	AuroraConfigReference string `json:"auroraConfigReference"`
	FileName              string `json:"fileName"`
	Contents              string `json:"contents"`
}

// CreateAuroraConfigFileResponse is response from the named graphql mutation "createAuroraConfigFile"
type CreateAuroraConfigFileResponse struct {
	CreateAuroraConfigFile AuroraConfigFileValidationResponse `json:"createAuroraConfigFile"`
}

// CreateAuroraConfigFile creates an Aurora config file via API call (graphql)
func (api *APIClient) CreateAuroraConfigFile(file *auroraconfig.File) error {
	createAuroraConfigFileRequest := graphql.NewRequest(createAuroraConfigFileRequestString)
	newAuroraConfigFileInput := NewAuroraConfigFileInput{
		AuroraConfigName:      api.Affiliation,
		AuroraConfigReference: api.RefName,
		FileName:              file.Name,
		Contents:              file.Contents,
	}
	createAuroraConfigFileRequest.Var("newAuroraConfigFileInput", newAuroraConfigFileInput)

	var createAuroraConfigFileResponse CreateAuroraConfigFileResponse
	if err := api.RunGraphQlMutation(createAuroraConfigFileRequest, &createAuroraConfigFileResponse); err != nil {
		return err
	}
	if !createAuroraConfigFileResponse.CreateAuroraConfigFile.Success {
		return errors.Errorf("Remote error: %s\n", createAuroraConfigFileResponse.CreateAuroraConfigFile.Message)
	}

	return nil
}

const updateAuroraConfigFileRequestString = `mutation updateAuroraConfigFile($updateAuroraConfigFileInput: UpdateAuroraConfigFileInput!){
  updateAuroraConfigFile(input: $updateAuroraConfigFileInput)
  {
    message
    success
  }
}`

// UpdateAuroraConfigFileInput is input to the graphql updateAuroraConfigFile interface
type UpdateAuroraConfigFileInput struct {
	AuroraConfigName      string `json:"auroraConfigName"`
	AuroraConfigReference string `json:"auroraConfigReference"`
	FileName              string `json:"fileName"`
	Contents              string `json:"contents"`
	ExistingHash          string `json:"existingHash"`
}

// UpdateAuroraConfigFileResponse is response from the named graphql mutation "updateAuroraConfigFile"
type UpdateAuroraConfigFileResponse struct {
	UpdateAuroraConfigFile AuroraConfigFileValidationResponse `json:"updateAuroraConfigFile"`
}

// UpdateAuroraConfigFile updates an Aurora config file via API call (graphql)
func (api *APIClient) UpdateAuroraConfigFile(file *auroraconfig.File, eTag string) error {
	logrus.Debugf("UpdateAuroraConfigFile: ETag: %s", eTag)
	updateAuroraConfigFileRequest := graphql.NewRequest(updateAuroraConfigFileRequestString)

	if err := validateFileContentIsJSON(file); err != nil {
		return err
	}

	updateAuroraConfigFileInput := UpdateAuroraConfigFileInput{
		AuroraConfigName:      api.Affiliation,
		AuroraConfigReference: api.RefName,
		FileName:              file.Name,
		Contents:              file.Contents,
		ExistingHash:          "",
	}
	if eTag != "" {
		updateAuroraConfigFileInput.ExistingHash = eTag
	}

	updateAuroraConfigFileRequest.Var("updateAuroraConfigFileInput", updateAuroraConfigFileInput)

	var updateAuroraConfigFileResponse UpdateAuroraConfigFileResponse
	if err := api.RunGraphQlMutation(updateAuroraConfigFileRequest, &updateAuroraConfigFileResponse); err != nil {
		return err
	}
	if !updateAuroraConfigFileResponse.UpdateAuroraConfigFile.Success {
		return errors.Errorf("Remote error: %s\n", updateAuroraConfigFileResponse.UpdateAuroraConfigFile.Message)
	}

	return nil
}

const getFileNamesRequest = `query auroraConfig($auroraConfigName: String!){
    auroraConfig(name: $auroraConfigName) {
    files {
            name
        }
    }
}`
type getFileNamesResponse struct {
	AuroraAPIMetadata struct {
		ConfigNames []string
	}
}

// GetFileNames gets file names via API calls
func (api *APIClient) GetFileNames() (auroraconfig.FileNames, error) {
	endpoint := fmt.Sprintf("/auroraconfig/%s/filenames", api.Affiliation)

	api.RunGraphQl()

		response, err := api.Do(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	var fileNames auroraconfig.FileNames
	err = response.ParseItems(&fileNames)
	if err != nil {
		return nil, err
	}

	return fileNames, nil
}

func validateFileContentIsJSON(file *auroraconfig.File) error {
	_, err := json.Marshal(file.Contents)
	if err != nil {
		return err
	}
	return nil
}
