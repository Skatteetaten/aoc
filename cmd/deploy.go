package cmd

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/pkg/errors"
	"github.com/skatteetaten/ao/pkg/client"
	"github.com/skatteetaten/ao/pkg/config"
	"github.com/skatteetaten/ao/pkg/fuzzy"
	"github.com/skatteetaten/ao/pkg/prompt"
	"github.com/spf13/cobra"
)

type deploymentUnit struct {
	ApplicationList []string
	Cluster         *config.Cluster
	Affiliation     string
	OverrideToken   string
}

type deploymentPlan struct {
	DeploymentUnits map[string]map[string]*deploymentUnit
	UnitCount       int
}

var (
	flagAffiliation string
	flagOverrides   []string
	flagNoPrompt    bool
	flagVersion     string
	flagCluster     string
	flagExcludes    []string
)

const deployLong = `Deploys applications from the current AuroraConfig.
For use in CI environments use --no-prompt to disable interactivity.
`

const exampleDeploy = `  Given the following AuroraConfig:
    - about.json
    - foobar.json
    - bar.json
    - foo/about.json
    - foo/bar.json
		- foo/foobar.json
		- ref/about.json
    - ref/bar.json

  # Fuzzy matching: deploy foo/bar and foo/foobar
  ao deploy fo/ba

  # Exact matching: deploy foo/bar
  ao deploy foo/bar

  # Deploy an application with override for application file
  ao deploy foo/bar -o 'foo/bar.json:{"pause": true}'
	
  # Exclude application(s) from foo environment (regexp)
  ao deploy foo -e .*/bar -e .*/baz

  # Exclude environment(s) when deploying an application across environments (regexp)
  ao deploy bar -e ref/.*
`

var deployCmd = &cobra.Command{
	Aliases:     []string{"setup", "apply"},
	Use:         "deploy <applicationId>",
	Short:       "Deploy one or more ApplicationId (environment/application) to one or more clusters",
	Long:        deployLong,
	Example:     exampleDeploy,
	Annotations: map[string]string{"type": "actions"},
	RunE:        deploy,
}

func init() {
	RootCmd.AddCommand(deployCmd)

	deployCmd.Flags().StringVarP(&flagAffiliation, "auroraconfig", "a", "", "Overrides the logged in AuroraConfig")
	deployCmd.Flags().StringVarP(&flagCluster, "cluster", "c", "", "Limit deploy to given cluster name")
	deployCmd.Flags().BoolVarP(&flagNoPrompt, "no-prompt", "", false, "Suppress prompts")
	deployCmd.Flags().StringArrayVarP(&flagOverrides, "overrides", "o", []string{}, "Override in the form '[env/]file:{<json override>}'")
	deployCmd.Flags().StringArrayVarP(&flagExcludes, "exclude", "e", []string{}, "Select applications or environments to exclude from deploy")
	deployCmd.Flags().StringVarP(&flagVersion, "version", "v", "", "Set the given version in AuroraConfig before deploy")

	deployCmd.Flags().BoolVarP(&flagNoPrompt, "force", "f", false, "Suppress prompts")
	deployCmd.Flags().MarkHidden("force")
	deployCmd.Flags().StringVarP(&flagAffiliation, "affiliation", "", "", "Overrides the logged in affiliation")
	deployCmd.Flags().MarkHidden("affiliation")
}

func deploy(cmd *cobra.Command, args []string) error {

	if len(args) > 2 || len(args) < 1 {
		return cmd.Usage()
	}

	search := args[0]
	if len(args) == 2 {
		search = fmt.Sprintf("%s/%s", args[0], args[1])
	}

	overrides, err := parseOverride(flagOverrides)
	if err != nil {
		return err
	}

	if flagAffiliation == "" {
		flagAffiliation = AO.Affiliation
	}

	api := DefaultApiClient
	api.Affiliation = flagAffiliation

	if flagCluster != "" && !AO.Localhost {
		c := AO.Clusters[flagCluster]
		if c == nil {
			return errors.New("No such cluster " + flagCluster)
		}
		if !c.Reachable {
			return errors.Errorf("%s cluster is not reachable", flagCluster)
		}

		api.Host = c.BooberUrl
		api.Token = c.Token
		if pFlagToken != "" {
			api.Token = pFlagToken
		}
	}

	files, err := api.GetFileNames()
	if err != nil {
		return err
	}

	possibleDeploys := files.GetApplicationIds()
	applications := fuzzy.SearchForApplications(search, possibleDeploys)

	applications, err = filterExcludes(flagExcludes, applications)
	if err != nil {
		return err
	}

	if len(applications) == 0 {
		return errors.New("No applications to deploy")
	}

	if flagVersion != "" {
		if len(applications) > 1 {
			return errors.New("Deploy with version does only support one application")
		}
		fileName, err := files.Find(applications[0])
		if err != nil {
			return err
		}

		err = Set(cmd, []string{fileName, "/version", flagVersion})
		if err != nil {
			return err
		}
	}

	deploySpecs, err := api.GetAuroraDeploySpec(applications, true)
	if err != nil {
		return err
	}
	var filteredDeploymentSpecs []client.AuroraDeploySpec
	if flagCluster != "" {
		for _, spec := range deploySpecs {
			if spec.Value("/cluster").(string) == flagCluster {
				filteredDeploymentSpecs = append(filteredDeploymentSpecs, spec)
			}
		}
	} else {
		filteredDeploymentSpecs = deploySpecs
	}
	header, rows := GetDeploySpecTable(filteredDeploymentSpecs)
	DefaultTablePrinter(header, rows, cmd.OutOrStdout())

	var filteredApplications []string
	for _, spec := range filteredDeploymentSpecs {
		appID := spec.Value("applicationId").(string)
		filteredApplications = append(filteredApplications, appID)
	}

	shouldDeploy := true
	if !flagNoPrompt {
		defaultAnswer := len(filteredApplications) == 1
		message := fmt.Sprintf("Do you want to deploy %d application(s)?", len(filteredApplications))
		shouldDeploy = prompt.Confirm(message, defaultAnswer)
	}

	if !shouldDeploy {
		return errors.New("No applications to deploy")
	}

	deploymentPlan := createDeploymentPlan(filteredDeploymentSpecs, flagAffiliation, pFlagToken)

	result, err := deployToReachableClusters(deploymentPlan, overrides)
	if err != nil {
		return err
	}

	var results []client.DeployResult
	for _, r := range result {
		results = append(results, r.Results...)
	}

	if len(results) == 0 {
		return errors.New("No deploys were made")
	}

	sort.Slice(results, func(i, j int) bool {
		return strings.Compare(results[i].ADS.Name, results[j].ADS.Name) < 1
	})

	header, rows = getDeployResultTable(results)
	if len(rows) == 0 {
		return nil
	}

	DefaultTablePrinter(header, rows, cmd.OutOrStdout())
	for _, deploy := range results {
		if !deploy.Success {
			return errors.New("One or more deploys failed")
		}
	}

	return nil
}

func createDeploymentPlan(deploymentSpecs []client.AuroraDeploySpec, affiliation, token string) *deploymentPlan {
	units := make(map[string]map[string]*deploymentUnit)
	unitCount := 0
	for _, spec := range deploymentSpecs {
		appID := spec.Value("applicationId").(string)
		clusterName := spec.Value("cluster").(string)
		envName := spec.Value("envName").(string)

		if _, exists := units[clusterName]; !exists {
			units[clusterName] = make(map[string]*deploymentUnit)
		}

		if _, exists := units[clusterName][envName]; !exists {
			cluster := AO.Clusters[clusterName]

			overrideToken := ""
			if token != "" {
				overrideToken = token
			}

			unit := &deploymentUnit{Affiliation: affiliation, Cluster: cluster, OverrideToken: overrideToken, ApplicationList: []string{}}

			units[clusterName][envName] = unit
			unitCount++
		}

		units[clusterName][envName].ApplicationList = append(units[clusterName][envName].ApplicationList, appID)
	}

	return &deploymentPlan{DeploymentUnits: units, UnitCount: unitCount}
}

func filterExcludes(expressions, applications []string) ([]string, error) {
	apps := make([]string, len(applications))
	copy(apps, applications)

	for _, expr := range expressions {
		r, err := regexp.Compile(expr)
		if err != nil {
			return nil, err
		}
		tmp := apps[:0]
		for _, app := range apps {
			match := r.MatchString(app)
			if !match {
				tmp = append(tmp, app)
			}
		}
		apps = tmp
	}

	return apps, nil
}

func deployToReachableClusters(deploymentPlan *deploymentPlan, overrides map[string]string) ([]*client.DeployResults, error) {
	deployResult := make(chan *client.DeployResults)
	deployErrors := make(chan error)

	for _, units := range deploymentPlan.DeploymentUnits {
		go deployToCluster(units, overrides, deployResult, deployErrors)
	}

	var allResults []*client.DeployResults
	for i := 0; i < deploymentPlan.UnitCount; i++ {
		select {
		case err := <-deployErrors:
			return nil, err
		case result := <-deployResult:
			allResults = append(allResults, result)
		}
	}

	return allResults, nil
}

func deployToCluster(clusterUnits map[string]*deploymentUnit, overrides map[string]string, deployResult chan<- *client.DeployResults, deployErrors chan<- error) {
	for _, unit := range clusterUnits {
		if !unit.Cluster.Reachable {
			continue
		}
		payload := client.NewDeployPayload(unit.ApplicationList, overrides)
		cli := getClient(unit)

		result, err := cli.Deploy(payload)
		if err != nil {
			deployErrors <- err
		} else {
			deployResult <- result
		}
	}
}

func getClient(unit *deploymentUnit) *client.ApiClient {
	var cli *client.ApiClient
	var effectiveToken string

	if unit.OverrideToken != "" {
		effectiveToken = unit.OverrideToken
	} else {
		effectiveToken = unit.Cluster.Token
	}

	if AO.Localhost {
		cli = DefaultApiClient
		cli.Affiliation = unit.Affiliation
	} else {
		cli = &client.ApiClient{
			Host:        unit.Cluster.BooberUrl,
			Token:       effectiveToken,
			Affiliation: unit.Affiliation,
			RefName:     DefaultApiClient.RefName,
		}
	}

	return cli
}

func parseOverride(override []string) (map[string]string, error) {
	returnMap := make(map[string]string)
	for i := 0; i < len(override); i++ {
		indexByte := strings.IndexByte(override[i], ':')
		filename := override[i][:indexByte]
		jsonOverride := override[i][indexByte+1:]

		if !json.Valid([]byte(jsonOverride)) {
			msg := fmt.Sprintf("%s is not a valid json", jsonOverride)
			return nil, errors.New(msg)
		}

		returnMap[filename] = jsonOverride
	}
	return returnMap, nil
}

func getDeployResultTable(deploys []client.DeployResult) (string, []string) {
	var rows []string
	for _, item := range deploys {
		if item.Ignored {
			continue
		}
		ads := item.ADS
		pattern := "%s\t%s\t%s\t%s\t%s\t%s\t%s"
		status := "\x1b[32mDeployed\x1b[0m"
		if !item.Success {
			status = "\x1b[31mFailed\x1b[0m"
		}
		result := fmt.Sprintf(pattern, status, ads.Cluster, ads.Environment.Namespace, ads.Name, ads.Deploy.Version, item.DeployId, item.Reason)
		rows = append(rows, result)
	}

	header := "\x1b[00mSTATUS\x1b[0m\tCLUSTER\tENVIRONMENT\tAPPLICATION\tVERSION\tDEPLOY_ID\tMESSAGE"
	return header, rows
}
