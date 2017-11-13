package cmd

import (
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"github.com/skatteetaten/ao/pkg/client"
	"github.com/skatteetaten/ao/pkg/collections"
	"github.com/skatteetaten/ao/pkg/config"
	"github.com/skatteetaten/ao/pkg/fuzzy"
	"github.com/skatteetaten/ao/pkg/prompt"
	"github.com/spf13/cobra"
	"sort"
	"strings"
)

var (
	overrides   []string
	deployAll   bool
	noPrompt    bool
	version     string
	affiliation string
	cluster     string
)

const deployLong = `Deploy applications for the current affiliation.

A Deploy will compare the stored configuration with the running projects in OpenShift, and update the OpenShift
environment to match the specifications in the stored configuration.

If no changes is detected, no updates to OpenShift will be done (except for an update of the resourceVersion in the BuildConfig).

In addition, the command accepts a mixed list of applications and environments on the command line.
The names may be shortened; the command will search the current affiliation for unique matches.

If the command will result in multiple deploys, a confirmation dialog will be shown, listing the result of the command.
The list will contain all the affected applications and environments.  Please note that the two columns are not correlated.
The --force flag will override this, and execute the deploy without confirmation.
`

var deployCmd = &cobra.Command{
	Aliases: []string{"setup"},
	Use:     "deploy",
	Short:   "Deploy applications for the current affiliation",
	Long:    deployLong,
	RunE:    Deploy,
}

func init() {
	RootCmd.AddCommand(deployCmd)

	deployCmd.Flags().StringArrayVarP(&overrides, "overrides",
		"o", []string{}, "Override in the form [env/]file:{<json override>}")

	deployCmd.Flags().BoolVarP(&deployAll, "all",
		"", false, "Will deploy all applications in all clusters reachable")

	deployCmd.Flags().BoolVarP(&noPrompt, "noprompt",
		"", false, "Supress prompts")

	deployCmd.Flags().StringVarP(&version, "version",
		"v", "", "Will update the version tag in the app of base configuration file prior to deploy, depending on which file contains the version tag.  If both files "+
			"files contains the tag, the tag will be updated in the app configuration file.")

	deployCmd.Flags().StringVarP(&affiliation, "affiliation",
		"", "", "Overrides the logged in affiliation")

	deployCmd.Flags().StringVarP(&cluster, "cluster", "c", "", "Limit deploy to given clustername")
}

func Deploy(cmd *cobra.Command, args []string) error {

	// One application
	if len(args) < 1 && !deployAll {
		cmd.Usage()
		return nil
	}

	deployOnce := ao.Localhost || cluster != ""

	overrides, err := parseOverride(overrides)
	if err != nil {
		return err
	}

	if affiliation == "" {
		affiliation = ao.Affiliation
	}

	api := DefaultApiClient
	api.Affiliation = affiliation

	if cluster != "" {
		c := ao.Clusters[cluster]
		if c == nil {
			return errors.New("No such cluster " + cluster)
		}

		api.Host = c.BooberUrl
		api.Token = c.Token
		if persistentToken != "" {
			api.Token = persistentToken
		}
	}

	files, err := api.GetFileNames()
	if err != nil {
		return err
	}

	possibleDeploys := files.GetDeployments()
	var appsToDeploy []string
	if deployAll {
		args = []string{}
		appsToDeploy = possibleDeploys
	}

	for _, arg := range args {
		applications, _ := fuzzy.SearchForApplications(arg, possibleDeploys)
		if !noPrompt && len(applications) > 1 {
			selectedApps := applications
			printDeployments(applications)
			message := fmt.Sprintf("Add all %d application(s) to deploy?", len(applications))
			deployAll := prompt.Confirm(message)
			if !deployAll {
				selectedApps = prompt.MultiSelect("Which applications do you want to deploy?", applications)
			}
			appsToDeploy = append(appsToDeploy, selectedApps...)
		} else {
			appsToDeploy = append(appsToDeploy, applications...)
		}
	}

	// Only deploy unique applications
	deploySet := collections.NewStringSet()
	for _, app := range appsToDeploy {
		deploySet.Add(app)
	}
	appsToDeploy = deploySet.All()

	if len(appsToDeploy) == 0 {
		errors.New("No applications to deploy")
	}

	if !noPrompt {
		printDeployments(appsToDeploy)
		message := fmt.Sprintf("Do you want to deploy %d application(s)?", len(appsToDeploy))
		shouldDeploy := prompt.Confirm(message)
		if !shouldDeploy {
			return nil
		}
	}

	payload, err := client.NewDeployPayload(appsToDeploy, overrides)
	if err != nil {
		return err
	}

	if version != "" {
		if len(appsToDeploy) > 1 {
			return errors.New("Deploy with version does only support one application")
		}
		operation := client.JsonPatchOp{
			OP:    "add",
			Path:  "/version",
			Value: version,
		}

		fileName := appsToDeploy[0] + ".json"
		err := api.PatchAuroraConfigFile(fileName, operation)
		if err != nil {
			return err
		}
	}

	var result []client.DeployResult
	if deployOnce {
		result, err = api.Deploy(payload)
		if err != nil {
			return err
		}
	} else {
		result = deployToReachableClusters(affiliation, persistentToken, ao.Clusters, payload)
	}

	sort.Slice(result, func(i, j int) bool {
		return strings.Compare(result[i].ADS.Name, result[j].ADS.Name) < 1
	})

	PrintDeployResults(result)
	return nil
}

func deployToReachableClusters(affiliation, token string, clusters map[string]*config.Cluster, payload *client.DeployPayload) []client.DeployResult {

	reachableClusters := 0
	deployResult := make(chan []client.DeployResult)
	deployErrors := make(chan error)
	for _, c := range clusters {
		if !c.Reachable {
			continue
		}
		reachableClusters++

		clusterToken := c.Token
		if token != "" {
			clusterToken = token
		}

		cli := client.NewApiClient(c.BooberUrl, clusterToken, affiliation)

		go func() {
			result, err := cli.Deploy(payload)
			if err != nil {
				deployErrors <- err
			} else {
				deployResult <- result
			}
		}()
	}

	var allResults []client.DeployResult
	for i := 0; i < reachableClusters; i++ {
		select {
		case err := <-deployErrors:
			fmt.Println(err)
		case result := <-deployResult:
			allResults = append(allResults, result...)
		}
	}

	return allResults
}

func parseOverride(override []string) (returnMap map[string]json.RawMessage, err error) {
	returnMap = make(map[string]json.RawMessage)

	for i := 0; i < len(override); i++ {
		indexByte := strings.IndexByte(override[i], ':')
		filename := override[i][:indexByte]

		jsonOverride := override[i][indexByte+1:]
		if !json.Valid([]byte(jsonOverride)) {
			msg := fmt.Sprintf("%s is not a valid json", jsonOverride)
			return nil, errors.New(msg)
		}
		returnMap[filename] = json.RawMessage(jsonOverride)
	}
	return returnMap, err
}

func PrintDeployResults(deploys []client.DeployResult) {
	results := []string{"\x1b[00mSTATUS\x1b[0m\tAPPLICATION\tENVIRONMENT\tCLUSTER\tDEPLOY_ID\t"}
	for _, item := range deploys {
		ads := item.ADS
		pattern := "%s\t%s\t%s\t%s\t%s\t"
		status := "\x1b[32mDeployed\x1b[0m"
		if !item.Success {
			status = "\x1b[31mFailed\x1b[0m"
		}
		result := fmt.Sprintf(pattern, status, ads.Name, ads.Namespace, ads.Cluster, item.DeployId)
		results = append(results, result)
	}

	if len(deploys) > 0 {
		DefaultTablePrinter(results)
	}
}

func printDeployments(deployments []string) {
	sort.Strings(deployments)
	lines := GetDeploymentTable(deployments)
	DefaultTablePrinter(lines)
}
