package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"runtime"

	"path/filepath"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/skatteetaten/ao/pkg/prompt"
)

type AOConfig struct {
	RefName     string              `json:"refName"`
	APICluster  string              `json:"apiCluster"`
	Affiliation string              `json:"affiliation"`
	Localhost   bool                `json:"localhost"`
	Clusters    map[string]*Cluster `json:"clusters"`

	AvailableClusters       []string `json:"availableClusters"`
	PreferredAPIClusters    []string `json:"preferredApiClusters"`
	AvailableUpdateClusters []string `json:"availableUpdateClusters"`
	ClusterUrlPattern       string   `json:"clusterUrlPattern"`
	BooberUrlPattern        string   `json:"booberUrlPattern"`
	UpdateUrlPattern        string   `json:"updateUrlPattern"`
	GoboUrlPattern          string   `json:"goboUrlPattern"`
}

var DefaultAOConfig = AOConfig{
	RefName:                 "master",
	Clusters:                make(map[string]*Cluster),
	AvailableClusters:       []string{"utv", "utv-relay", "test", "test-relay", "prod", "prod-relay"},
	PreferredAPIClusters:    []string{"utv", "test"},
	AvailableUpdateClusters: []string{"utv", "test"},
	ClusterUrlPattern:       "https://%s-master.paas.skead.no:8443",
	BooberUrlPattern:        "http://boober-aurora.%s.paas.skead.no",
	UpdateUrlPattern:        "http://ao-aurora-tools.%s.paas.skead.no",
	GoboUrlPattern:          "http://gobo.aurora.%s.paas.skead.no",
}

func LoadConfigFile(configLocation string) (*AOConfig, error) {
	raw, err := ioutil.ReadFile(configLocation)
	if err != nil {
		return nil, err
	}

	var c *AOConfig
	err = json.Unmarshal(raw, &c)
	if err != nil {
		return nil, err
	}

	return c, nil
}

func WriteConfig(ao AOConfig, configLocation string) error {
	data, err := json.MarshalIndent(ao, "", "  ")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(configLocation, data, 0644)
}

func (ao *AOConfig) SelectApiCluster() {
	if ao.APICluster != "" {
		return
	}

	for _, name := range ao.PreferredAPIClusters {
		cluster, found := ao.Clusters[name]
		if !found {
			continue
		}

		if cluster.Reachable {
			ao.APICluster = name
			return
		}
	}

	for k, cluster := range ao.Clusters {
		if cluster.Reachable {
			ao.APICluster = k
			return
		}
	}
}

func (ao *AOConfig) Update(noPrompt bool) error {
	url := ao.getUpdateUrl()
	if url == "" {
		return errors.New("No update server is available, check config")
	}
	serverVersion, err := GetCurrentVersionFromServer(url)
	if err != nil {
		return err
	}

	if !serverVersion.IsNewVersion() {
		return errors.New("No update available")
	}

	if !noPrompt {
		if runtime.GOOS == "windows" {
			message := fmt.Sprintf("New version of AO is available (%s) - please download from %s", serverVersion.Version, url)
			fmt.Println(message)
			return nil
		} else {
			message := fmt.Sprintf("Do you want update AO from version %s -> %s?", Version, serverVersion.Version)
			update := prompt.Confirm(message, true)
			if !update {
				return errors.New("Update aborted")
			}
		}
	}

	data, err := GetNewAOClient(url)
	if err != nil {
		return err
	}

	err = ao.replaceAO(data)
	if err != nil {
		return err
	}

	return nil
}

func (ao *AOConfig) replaceAO(data []byte) error {
	executablePath, err := os.Executable()
	if err != nil {
		return err
	}

	var releasePath string
	// First, we try to write the update to a file in the executable path
	releasePath = executablePath + "_" + "update"
	err = ioutil.WriteFile(releasePath, data, 0755)
	if err != nil {
		// Could not write to executable path, typically because binary is installed in /usr/bin or /usr/local/bin
		// Try the OS Temp Dir
		releasePath = filepath.Join(os.TempDir(), "ao_update")
		err = ioutil.WriteFile(releasePath, data, 0755)
		if err != nil {
			return err
		}
	}
	err = os.Rename(releasePath, executablePath)
	if err != nil {
		err = errors.New("Could not update AO because it is installed in a different file system than temp: " + err.Error())
		return err
	}
	return nil
}

func (ao *AOConfig) getUpdateUrl() string {
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
		return ""
	}

	return fmt.Sprintf(ao.UpdateUrlPattern, updateCluster)
}
