package cmd

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"

	"ao/pkg/client"
	"github.com/pkg/errors"
)

// DefaultTablePrinter prints a table on screen
func DefaultTablePrinter(header string, rows []string, out io.Writer) {
	w := tabwriter.NewWriter(out, 0, 0, 3, ' ', tabwriter.TabIndent)

	if !pFlagNoHeader {
		fmt.Fprintln(w, header)
	}

	for _, line := range rows {
		fmt.Fprintln(w, line)
	}
	w.Flush()
}

// GetApplicationDeploymentRefTable creates a table of deployments
func GetApplicationDeploymentRefTable(deployments []string) (string, []string) {
	var rows []string
	last := ""
	for _, app := range deployments {
		sp := strings.Split(app, "/")
		env := sp[0]
		app := sp[1]
		if env == last {
			env = " "
		}
		line := fmt.Sprintf("%s\t%s", env, app)
		rows = append(rows, line)
		last = sp[0]
	}

	return "ENVIRONMENT\tAPPLICATION", rows
}

// GetFilesTable creates a table of files
func GetFilesTable(files []string) (string, []string) {
	var single []string
	var envApp []string

	for _, file := range files {
		if strings.ContainsRune(file, '/') {
			envApp = append(envApp, file)
		} else {
			single = append(single, file)
		}
	}

	sort.Strings(single)
	sort.Strings(envApp)
	return "FILES", append(single, envApp...)
}

func getAPIClient(auroraConfig, overrideToken, overrideCluster string) (*client.ApiClient, error) {
	api := DefaultAPIClient
	api.Affiliation = auroraConfig

	if overrideCluster != "" && !AO.Localhost {
		c := AO.Clusters[overrideCluster]
		if !c.Reachable {
			return nil, errors.Errorf("%s cluster is not reachable", overrideCluster)
		}

		api.Host = c.BooberUrl
		api.GoboHost = c.GoboUrl
		api.Token = c.Token
		if overrideToken != "" {
			api.Token = overrideToken
		}
	}

	return api, nil
}
