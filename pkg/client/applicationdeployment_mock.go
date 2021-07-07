package client

import (
	"github.com/pkg/errors"
)

// ApplicationDeploymentClientMock is a base mock type
type ApplicationDeploymentClientMock struct {
	APIClientMock
}

// NewApplicationDeploymentClientMock creates a new ApplicationDeploymentClientMock
func NewApplicationDeploymentClientMock() *ApplicationDeploymentClientMock {
	return &ApplicationDeploymentClientMock{}
}

// Deploy default mock implementation
func (api *ApplicationDeploymentClientMock) Deploy(deployPayload *DeployPayload) (*DeployResults, error) {
	api.Called()
	return &DeployResults{Message: "Successful", Success: true, Results: []DeployResult{}}, nil
}

// Delete default mock implementation
func (api *ApplicationDeploymentClientMock) Delete(deletePayload *DeletePayload) (*DeleteResults, error) {
	api.Called()

	results := make([]DeleteResult, len(deletePayload.ApplicationRefs))

	for i := range deletePayload.ApplicationRefs {
		results[i] = DeleteResult{Success: true, Reason: "OK", ApplicationRef: ApplicationRef{}}
	}

	return &DeleteResults{Message: "Successful", Success: true, Results: results}, nil
}

// Exists default mock implementation
func (api *ApplicationDeploymentClientMock) Exists(existsPayload *ExistsPayload) (*ExistsResults, error) {
	api.Called()

	results := make([]ExistsResult, len(existsPayload.ApplicationDeploymentRefs))

	for i := range existsPayload.ApplicationDeploymentRefs {
		results[i] = ExistsResult{Success: true, Message: "OK", Exists: true, ApplicationRef: ApplicationRef{
			Name: existsPayload.ApplicationDeploymentRefs[i].Application,
		}}
	}

	return &ExistsResults{Message: "Successful", Success: true, Results: results}, nil
}

// GetApplyResult default mock implementation
func (api *ApplicationDeploymentClientMock) GetApplyResult(deployID string) (string, error) {
	return "", errors.New("Not implemented")
}
