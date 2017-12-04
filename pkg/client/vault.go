package client

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"net/http"
)

type (
	Secrets     map[string]string
	Permissions map[string][]string

	AuroraVaultInfo struct {
		Name        string      `json:"name"`
		Permissions Permissions `json:"permissions"`
		Secrets     []string    `json:"secrets"`
		Admin       bool        `json:"admin"`
	}

	AuroraSecretVault struct {
		Name        string            `json:"name"`
		Permissions Permissions       `json:"permissions"`
		Secrets     Secrets           `json:"secrets"`
		Versions    map[string]string `json:"versions"`
	}
)

func NewAuroraSecretVault(name string) *AuroraSecretVault {
	return &AuroraSecretVault{
		Name:        name,
		Permissions: make(Permissions),
		Secrets:     make(Secrets),
		Versions:    make(map[string]string),
	}
}

func (api *ApiClient) GetVaults() ([]*AuroraVaultInfo, error) {
	endpoint := fmt.Sprintf("/vault/%s", api.Affiliation)

	response, err := api.Do(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	var vaults []*AuroraVaultInfo
	err = response.ParseItems(&vaults)
	if err != nil {
		return nil, err
	}

	return vaults, nil
}

func (api *ApiClient) GetVault(vaultName string) (*AuroraSecretVault, error) {
	endpoint := fmt.Sprintf("/vault/%s/%s", api.Affiliation, vaultName)

	response, err := api.Do(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	var vault AuroraSecretVault
	err = response.ParseFirstItem(&vault)
	if err != nil {
		return nil, err
	}

	return &vault, nil
}

func (api *ApiClient) DeleteVault(vaultName string) error {
	endpoint := fmt.Sprintf("/vault/%s/%s", api.Affiliation, vaultName)

	response, err := api.Do(http.MethodDelete, endpoint, nil)
	if err != nil {
		return err
	}

	if !response.Success {
		return errors.New(response.Message)
	}

	return nil
}

func (api *ApiClient) SaveVault(vault AuroraSecretVault, validate bool) error {
	endpoint := fmt.Sprintf("/vault/%s", api.Affiliation)

	payload := struct {
		Vault            AuroraSecretVault `json:"vault"`
		ValidateVersions bool              `json:"validateVersions"`
	}{
		Vault:            vault,
		ValidateVersions: validate,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	response, err := api.Do(http.MethodPut, endpoint, data)
	if err != nil {
		return err
	}

	if !response.Success {
		return errors.New(response.Message)
	}

	return nil
}

func (api *ApiClient) UpdateSecretFile(vault, secret string, content []byte) error {
	endpoint := fmt.Sprintf("/vault/%s/%s/secret/%s", api.Affiliation, vault, secret)

	response, err := api.Do(http.MethodPut, endpoint, content)
	if err != nil {
		return err
	}

	if !response.Success {
		return errors.New(response.Message)
	}

	return nil
}

func (s Secrets) GetSecret(name string) (string, error) {
	secret, found := s[name]
	if !found {
		return "", errors.Errorf("Did not find secret %s", name)
	}
	data, err := base64.StdEncoding.DecodeString(secret)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

func (s Secrets) AddSecret(name, content string) {
	encoded := base64.StdEncoding.EncodeToString([]byte(content))
	s[name] = encoded
}

func (s Secrets) RemoveSecret(name string) {
	delete(s, name)
}

func (p Permissions) AddGroup(group string) error {
	groups := p["groups"]
	for _, g := range groups {
		if g == group {
			return errors.Errorf("Group %s already exists", group)
		}
	}
	p["groups"] = append(groups, group)

	return nil
}

func (p Permissions) DeleteGroup(group string) error {
	groups := p["groups"]
	for i, g := range groups {
		if g == group {
			p["groups"] = append(groups[:i], groups[i+1:]...)
			return nil
		}
	}
	return errors.Errorf("Did not find group %s", group)
}

func (p Permissions) GetGroups() []string {
	return p["groups"]
}