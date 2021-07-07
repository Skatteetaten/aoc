package client

import (
	"github.com/skatteetaten/ao/pkg/auroraconfig"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestApiClient_CreateAuroraConfigFile(t *testing.T) {
	t.Run("Should create a new aurora config file", func(t *testing.T) {
		fileName := "basic_auroraconfig"
		filecontent := ReadTestFile(fileName)
		acf := &auroraconfig.File{
			Name:     "testconfig.json",
			Contents: string(filecontent),
		}
		responseFileName := "createauroraconfigfile_success_response"
		response := ReadTestFile(responseFileName)

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write(response)
		}))
		defer ts.Close()

		api := NewAPIClientDefaultRef("", ts.URL, "test", affiliation, "")
		err := api.CreateAuroraConfigFile(acf)
		assert.NoError(t, err)
	})
	t.Run("Should fail to create a new aurora config file", func(t *testing.T) {
		fileName := "basic_auroraconfig"
		filecontent := ReadTestFile(fileName)
		acf := &auroraconfig.File{
			Name:     "testconfig.json",
			Contents: string(filecontent),
		}
		responseFileName := "createauroraconfigfile_error_response"
		response := ReadTestFile(responseFileName)

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write(response)
		}))
		defer ts.Close()

		api := NewAPIClientDefaultRef("", ts.URL, "test", affiliation, "")
		err := api.CreateAuroraConfigFile(acf)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Could not add file")
	})
}
func TestApiClient_UpdateAuroraConfigFile(t *testing.T) {
	t.Run("Should update a aurora config file", func(t *testing.T) {
		fileName := "basic_auroraconfig"
		filecontent := ReadTestFile(fileName)
		acf := &auroraconfig.File{
			Name:     "testconfig.json",
			Contents: string(filecontent),
		}
		etag := "ETag"
		responseFileName := "updateauroraconfigfile_success_response"
		response := ReadTestFile(responseFileName)

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write(response)
		}))
		defer ts.Close()

		api := NewAPIClientDefaultRef("", ts.URL, "test", affiliation, "")
		err := api.UpdateAuroraConfigFile(acf, etag)
		assert.NoError(t, err)
	})
	t.Run("Should fail to update a aurora config file", func(t *testing.T) {
		fileName := "basic_auroraconfig"
		filecontent := ReadTestFile(fileName)
		acf := &auroraconfig.File{
			Name:     "testconfig.json",
			Contents: string(filecontent),
		}
		etag := "ETag"
		responseFileName := "updateauroraconfigfile_error_response"
		response := ReadTestFile(responseFileName)

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write(response)
		}))
		defer ts.Close()

		api := NewAPIClientDefaultRef("", ts.URL, "test", affiliation, "")
		err := api.UpdateAuroraConfigFile(acf, etag)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Could not update file")
	})
}

func TestApiClient_GetFileNames(t *testing.T) {
	t.Run("Should get all filenames in AuroraConfig for a given affiliation", func(t *testing.T) {
		responseFileName := "auroraconfig_files_name_response"
		response := ReadTestFile(responseFileName)

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write(response)
		}))
		defer ts.Close()

		api := NewAPIClientDefaultRef("", ts.URL, "test", affiliation, "")
		fileNames, err := api.GetFileNames()
		assert.NoError(t, err)
		assert.Len(t, fileNames, 4)
	})
}
