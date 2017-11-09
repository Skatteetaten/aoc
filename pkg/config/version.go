package config

import (
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
)

var BuildStamp string
var Branch string
var GitHash string
var Version string

type AOVersion struct {
	Version    string `json:"version"`
	Branch     string `json:"branch"`
	GitHash    string `json:"gitHash"`
	BuildStamp string `json:"buildStamp"`
}

func (v *AOVersion) IsNewVersion(version string) bool {
	// TODO: Should do better check then this
	return v.Version != version
}

var DefaultAOVersion = AOVersion{
	Version:    Version,
	BuildStamp: BuildStamp,
	Branch:     Branch,
	GitHash:    GitHash,
}

const aoDownloadPath = "/api/ao"
const aoCurrentVersionPath = "/api/version"

func (ao *AOConfig) getUpdateUrl() (string, error) {
	var updateCluster string
	for _, c := range ao.AvailableUpdateClusters {
		available, found := ao.Clusters[c]
		logrus.WithField("exists", found).Info("update server", c)
		if found && available.Reachable {
			updateCluster = c
			break
		}
	}

	if updateCluster == "" {
		return "", errors.New("No update servers available")
	}

	return fmt.Sprintf(ao.UpdateUrlPattern, updateCluster), nil
}

func (ao *AOConfig) GetCurrentVersionFromServer() (*AOVersion, error) {
	url, err := ao.getUpdateUrl()
	if err != nil {
		return nil, err
	}

	logrus.WithField("url", url).Info("Request")
	req, err := http.NewRequest(http.MethodGet, url+aoCurrentVersionPath, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)

	logrus.WithField("url", url).WithField("status", res.StatusCode).Info("Response")
	if res.StatusCode != http.StatusOK {
		return nil, errors.New(res.Status)
	}

	defer res.Body.Close()
	data, err := ioutil.ReadAll(res.Body)

	var aoVersion AOVersion
	err = json.Unmarshal(data, &aoVersion)
	if err != nil {
		return nil, err
	}

	return &aoVersion, nil
}

func (ao *AOConfig) GetNewAOClient() ([]byte, error) {
	url, err := ao.getUpdateUrl()
	if err != nil {
		return nil, err
	}

	logrus.WithField("url", url).Info("Request")
	req, err := http.NewRequest(http.MethodGet, url+aoDownloadPath, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/octet-stream")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Error downloading update from %v: %v", url, err))
	}

	logrus.WithField("url", url).WithField("status", res.StatusCode).Info("Response")
	if res.StatusCode != http.StatusOK {
		return nil, errors.New(res.Status)
	}

	defer res.Body.Close()
	file, err := ioutil.ReadAll(res.Body)

	return file, err
}
