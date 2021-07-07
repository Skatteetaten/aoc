package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/skatteetaten/ao/pkg/auroraconfig"
	"github.com/stretchr/testify/assert"
)

func AuroraConfigSuccessResponseHandler(t *testing.T, responseFile string) http.HandlerFunc {
	return func(writer http.ResponseWriter, req *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusOK)
		assert.Contains(t, req.URL.Path, affiliation)
		data := ReadTestFile(responseFile)
		writer.Write(data)
	}
}

func AuroraConfigFailedResponseHandler(t *testing.T, responseFile string, status int) http.HandlerFunc {
	return func(writer http.ResponseWriter, req *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(status)

		assert.Contains(t, req.URL.Path, affiliation)

		data := ReadTestFile(responseFile)
		writer.Write(data)
	}
}

func TestApi_GetAuroraConfig(t *testing.T) {

	t.Run("Successfully get AuroraConfig", func(t *testing.T) {
		fileName := "auroraconfig_paas_success_response"
		ts := httptest.NewServer(AuroraConfigSuccessResponseHandler(t, fileName))
		defer ts.Close()

		api := NewAPIClientDefaultRef(ts.URL, "", "", affiliation, "")
		ac, errResponse := api.GetAuroraConfig()

		assert.Empty(t, errResponse)
		assert.Len(t, ac.Files, 4)
	})
}

func TestApiClient_ValidateAuroraConfig(t *testing.T) {
	t.Run("Successfully validate and save AuroraConfig", func(t *testing.T) {
		fileName := "auroraconfig_paas_success_response"
		ts := httptest.NewServer(AuroraConfigSuccessResponseHandler(t, fileName))
		defer ts.Close()

		api := NewAPIClientDefaultRef(ts.URL, "", "", affiliation, "")

		data := ReadTestFile("auroraconfig_paas_success_validation_request")
		var ac auroraconfig.AuroraConfig
		err := json.Unmarshal(data, &ac)
		if err != nil {
			t.Error(err)
		}
		warnings, err := api.ValidateAuroraConfig(&ac, false)
		assert.NoError(t, err)
		assert.Empty(t, warnings)
	})

	t.Run("Validate AuroraConfig with warnings", func(t *testing.T) {
		fileName := "auroraconfig_paas_warning_validation_response"
		ts := httptest.NewServer(AuroraConfigSuccessResponseHandler(t, fileName))
		defer ts.Close()

		api := NewAPIClientDefaultRef(ts.URL, "", "", affiliation, "")

		data := ReadTestFile("auroraconfig_paas_warning_validation_request")
		var ac auroraconfig.AuroraConfig
		err := json.Unmarshal(data, &ac)
		if err != nil {
			t.Error(err)
		}
		warnings, err := api.ValidateAuroraConfig(&ac, false)
		assert.NoError(t, err)
		assert.NotEmpty(t, warnings)
	})

	t.Run("Validation and save should fail when deploy type is illegal", func(t *testing.T) {
		fileName := "auroraconfig_paas_failed_validation_response"
		ts := httptest.NewServer(AuroraConfigFailedResponseHandler(t, fileName, http.StatusBadRequest))
		defer ts.Close()

		api := NewAPIClientDefaultRef(ts.URL, "", "", affiliation, "")
		data := ReadTestFile("auroraconfig_paas_fail_validation_request")
		var ac auroraconfig.AuroraConfig
		err := json.Unmarshal(data, &ac)
		if err != nil {
			t.Error(err)
		}

		warnings, err := api.ValidateAuroraConfig(&ac, false)
		assert.Error(t, err)
		assert.Empty(t, warnings)
	})
}

func TestApiClient_GetAuroraConfigFile(t *testing.T) {
	t.Run("Should successfully get AuroraConfigFile", func(t *testing.T) {
		fileName := "auroraconfigfile_paas_success_response"
		ts := httptest.NewServer(AuroraConfigSuccessResponseHandler(t, fileName))
		defer ts.Close()

		api := NewAPIClientDefaultRef(ts.URL, "", "", "paas", "")
		// TODO: Test ETag
		file, _, err := api.GetAuroraConfigFile("about.json")
		if err != nil {
			t.Error("Should not get error when fetching AuroraConfigFile")
			return
		}

		assert.Equal(t, "about.json", file.Name)
		assert.NotEmpty(t, file.Contents)
	})

	t.Run("Should return error message when AuroraConfigFile return success false", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			response := `{"success": false}`
			w.Write([]byte(response))
		}))
		defer ts.Close()

		api := NewAPIClientDefaultRef(ts.URL, "", "", "", "")

		// TODO: Test ETag
		file, _, err := api.GetAuroraConfigFile("about.json")
		assert.Error(t, err)
		assert.EqualError(t, err, "Failed getting file about.json")
		assert.Empty(t, file)
	})
}

func TestFileNames_Filter(t *testing.T) {
	fileNames := auroraconfig.FileNames{"about.json", "boober.json", "test/about.json", "test/boober.json"}
	deploymentRefs := fileNames.GetApplicationDeploymentRefs()
	environments := fileNames.GetEnvironments()
	applications := fileNames.GetApplications()

	assert.Len(t, deploymentRefs, 1)
	assert.Len(t, environments, 1)
	assert.Len(t, applications, 1)
	assert.Equal(t, "test/boober", deploymentRefs[0])
	assert.Equal(t, "test", environments[0])
	assert.Equal(t, "boober", applications[0])
}

func TestAuroraConfigFile_ToPrettyJson(t *testing.T) {
	acf := &auroraconfig.File{
		Name:     "about.json",
		Contents: `{"type":"development"}`,
	}

	expected := "{\n  \"type\": \"development\"\n}"
	assert.Equal(t, expected, acf.ToPrettyJSON())
}

func TestApiClient_ValidateRemoteAuroraConfig(t *testing.T) {
	t.Run("Successfully validate remote AuroraConfig", func(t *testing.T) {
		fileName := "auroraconfig_paas_success_response"
		ts := httptest.NewServer(AuroraConfigSuccessResponseHandler(t, fileName))
		defer ts.Close()

		api := NewAPIClientDefaultRef(ts.URL, "", "", affiliation, "")

		warnings, err := api.ValidateRemoteAuroraConfig(false)
		assert.NoError(t, err)
		assert.Empty(t, warnings)

	})

	t.Run("Validation and save should fail when deploy type is illegal", func(t *testing.T) {
		fileName := "auroraconfig_paas_failed_validation_response"
		ts := httptest.NewServer(AuroraConfigFailedResponseHandler(t, fileName, http.StatusBadRequest))
		defer ts.Close()

		api := NewAPIClientDefaultRef(ts.URL, "", "", affiliation, "")

		warnings, err := api.ValidateRemoteAuroraConfig(false)
		assert.Error(t, err)
		assert.Empty(t, warnings)

	})
}
