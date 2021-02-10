package client

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestApiClient_GetVaults(t *testing.T) {
	t.Run("Should get vaults", func(t *testing.T) {
		fileName := "get_vaults_response"
		responseBody := ReadTestFile(fileName)

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write(responseBody)
		}))
		defer ts.Close()

		api := NewAPIClientDefaultRef("", ts.URL, "test", "sales", "")
		vaults, err := api.GetVaults()

		assert.NoError(t, err)
		assert.Len(t, vaults, 4)
	})
}

func TestApiClient_GetVault(t *testing.T) {
	t.Run("Should get console vault", func(t *testing.T) {
		fileName := "get_vault_console_response"
		responseBody := ReadTestFile(fileName)

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write(responseBody)
		}))
		defer ts.Close()

		api := NewAPIClientDefaultRef(ts.URL, "", "test", affiliation, "")
		vault, err := api.GetVault("console")
		assert.NoError(t, err)

		assert.Equal(t, "console", vault.Name)
		assert.Len(t, vault.Secrets, 1)
	})
}

func TestApiClient_DeleteVault(t *testing.T) {
	t.Run("Should delete vault", func(t *testing.T) {

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"success":true}`))
		}))
		defer ts.Close()

		api := NewAPIClientDefaultRef(ts.URL, "", "test", affiliation, "")
		err := api.DeleteVault("console")
		assert.NoError(t, err)
	})
}

func TestAPIClient_CreateVault(t *testing.T) {
	t.Run("Should create vault", func(t *testing.T) {
		responseFileName := "createvault_success_response"
		response := ReadTestFile(responseFileName)

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write(response)
		}))
		defer ts.Close()

		api := NewAPIClientDefaultRef("", ts.URL, "test", affiliation, "")
		vault, err := api.CreateVault(Vault{
			Name:        "test-vault",
			Permissions: []string{"utv"},
			HasAccess:   true,
			Secrets: []Secret{{
				Name:          "latest.properties",
				Base64Content: "YWJjMTIz",
			}},
		})

		assert.NoError(t, err)
		assert.Equal(t, "test-vault", vault.Name)
	})
}

func TestApiClient_SaveVault(t *testing.T) {
	t.Run("Should not save vault with no permissions", func(t *testing.T) {

		vault := NewAuroraSecretVault("foo")

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"success":true}`))
		}))
		defer ts.Close()

		api := NewAPIClientDefaultRef(ts.URL, "", "test", affiliation, "")
		err := api.SaveVault(*vault)
		assert.Error(t, err, "SaveVault should return error when there are no permissions")
	})

	t.Run("Should save vault", func(t *testing.T) {

		vault := NewAuroraSecretVault("foo")
		vault.Permissions = append(vault.Permissions, "someTestGroup")

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"success":true}`))
		}))
		defer ts.Close()

		api := NewAPIClientDefaultRef(ts.URL, "", "test", affiliation, "")
		err := api.SaveVault(*vault)
		assert.NoError(t, err)
	})
}

func TestApiClient_UpdateSecretFile(t *testing.T) {
	t.Run("Should update secret file", func(t *testing.T) {

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"success":true}`))
		}))
		defer ts.Close()

		api := NewAPIClientDefaultRef(ts.URL, "", "test", affiliation, "")
		err := api.UpdateSecretFile("console", "latest.properties", "", []byte("Rk9PPVRFU1QK"))
		assert.NoError(t, err)
	})
}

func TestSecrets(t *testing.T) {
	secrets := Secrets{
		"latest.properties": "Rk9PPVRFU1QK",
	}
	secret, err := secrets.GetSecret("latest.properties")
	assert.NoError(t, err)
	assert.Equal(t, secret, "FOO=TEST\n")

	_, err = secrets.GetSecret("latest.properties2")
	assert.Error(t, err)

	secrets.AddSecret("latest.properties2", "FOO=TEST2")
	secret, err = secrets.GetSecret("latest.properties2")
	assert.NoError(t, err)
	assert.Equal(t, secret, "FOO=TEST2")

	secrets.RemoveSecret("latest.properties2")
	_, err = secrets.GetSecret("latest.properties2")
	assert.Error(t, err)
}
