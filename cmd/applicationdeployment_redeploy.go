package cmd

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/pkg/errors"
	"github.com/skatteetaten/ao/pkg/client"
	"github.com/skatteetaten/ao/pkg/config"
	"github.com/skatteetaten/ao/pkg/prompt"
	"github.com/skatteetaten/ao/pkg/service"
	"github.com/spf13/cobra"
)

var applicationDeploymentRedeployCmd = &cobra.Command{
	Use:   "redeploy <applicationDeploymentRef>",
	Short: "Redeploy running application deployment(s) with the given reference",
	RunE:  redeployApplicationDeployment,
}



type partialRedeployResult struct {
	partition     DeploymentPartition
	redeployResults client.RedeployResults
}

func newDeploymentPartition(deploymentInfos []DeploymentInfo, cluster config.Cluster, auroraConfig string, overrideToken string) *DeploymentPartition {
	return &DeploymentPartition{
		DeploymentInfos: deploymentInfos,
		Partition: Partition{
			Cluster:          cluster,
			AuroraConfigName: auroraConfig,
			OverrideToken:    overrideToken,
		},
	}
}

func newPartialRedeployResults(partition DeploymentPartition, redeployResults client.RedeployResults) partialRedeployResult {
	return partialRedeployResult{
		partition:     partition,
		redeployResults: redeployResults,
	}
}

func init() {
	applicationDeploymentCmd.AddCommand(applicationDeploymentRedeployCmd)
	applicationDeploymentRedeployCmd.Flags().StringVarP(&flagCluster, "cluster", "c", "", "Limit redeploy to given cluster name")
	applicationDeploymentRedeployCmd.Flags().BoolVarP(&flagNoPrompt, "yes", "y", false, "Suppress prompts and accept redeploy")
	applicationDeploymentRedeployCmd.Flags().BoolVarP(&flagNoPrompt, "no-prompt", "", false, "Suppress prompts and accept redeploy")
	applicationDeploymentRedeployCmd.Flags().StringArrayVarP(&flagExcludes, "exclude", "e", []string{}, "Select applications or environments to exclude from redeploy")

	applicationDeploymentRedeployCmd.Flags().BoolVarP(&flagNoPrompt, "force", "f", false, "Suppress prompts")
	applicationDeploymentRedeployCmd.Flags().MarkHidden("force")
	applicationDeploymentRedeployCmd.Flags().StringVarP(&flagAuroraConfig, "affiliation", "", "", "Overrides the logged in affiliation")
	applicationDeploymentRedeployCmd.Flags().MarkHidden("affiliation")
}

func redeployApplicationDeployment(cmd *cobra.Command, args []string) error {

	// TODO: Adapt to redeploy

	if len(args) > 2 || len(args) < 1 {
		return cmd.Usage()
	}

	err := validateRedeployParams()
	if err != nil {
		return err
	}

	search := args[0]
	if len(args) == 2 {
		search = fmt.Sprintf("%s/%s", args[0], args[1])
	}

	auroraConfigName := AO.Affiliation
	if flagAuroraConfig != "" {
		auroraConfigName = flagAuroraConfig
	}

	apiClient, err := getAPIClient(auroraConfigName, pFlagToken, flagCluster)
	if err != nil {
		return err
	}

	applications, err := service.GetApplications(apiClient, search, flagExcludes)
	if err != nil {
		return err
	} else if len(applications) == 0 {
		return errors.New("No applications to redeploy")
	}

	filteredDeploymentSpecs, err := service.GetFilteredDeploymentSpecs(apiClient, applications, flagCluster)
	if err != nil {
		return err
	}

	deployInfos, err := getDeployedApplications(getApplicationDeploymentClient, filteredDeploymentSpecs, auroraConfigName, pFlagToken)
	if err != nil {
		return err
	} else if len(deployInfos) == 0 {
		return errors.New("No applications to redeploy")
	}

	partitions, err := createDeploymentPartitions(auroraConfigName, pFlagToken, AO.Clusters, deployInfos)
	if err != nil {
		return err
	}

	if !getRedeployConfirmation(flagNoPrompt, deployInfos, cmd.OutOrStdout()) {
		return errors.New("No applications to redeploy")
	}

	fullResults, err := redeployFromReachableClusters(getApplicationDeploymentClient, partitions)
	if err != nil {
		return err
	}

	printFullResults(fullResults, cmd.OutOrStdout())

	for _, result := range fullResults {
		if !result.redeployResults.Success {
			return errors.New("One or more redeploy operations failed")
		}
	}

	return nil
}

func validateRedeployParams() error {
	if flagCluster != "" {
		if _, exists := AO.Clusters[flagCluster]; !exists {
			return errors.New(fmt.Sprintf("No such cluster %s", flagCluster))
		}
	}

	return nil
}

func createDeploymentPartitions(auroraConfig, overrideToken string, clusters map[string]*config.Cluster, deployInfos []DeploymentInfo) ([]DeploymentPartition, error) {
	type deploymentPartitionID struct {
		namespace, clusterName string
	}

	partitionMap := make(map[deploymentPartitionID]*DeploymentPartition)

	for _, info := range deployInfos {
		clusterName := info.ClusterName
		namespace := info.Namespace

		partitionID := deploymentPartitionID{clusterName, namespace}

		if _, exists := partitionMap[partitionID]; !exists {
			if _, exists := clusters[clusterName]; !exists {
				return nil, errors.New(fmt.Sprintf("No such cluster %s", clusterName))
			}
			cluster := clusters[clusterName]
			partition := newDeploymentPartition([]DeploymentInfo{}, *cluster, auroraConfig, overrideToken)
			partitionMap[partitionID] = partition
		}

		partitionMap[partitionID].DeploymentInfos = append(partitionMap[partitionID].DeploymentInfos, info)
	}

	partitions := make([]DeploymentPartition, len(partitionMap))

	idx := 0
	for _, partition := range partitionMap {
		partitions[idx] = *partition
		idx++
	}

	return partitions, nil
}

func redeployFromReachableClusters(getClient func(partition Partition) client.ApplicationDeploymentClient, partitions []DeploymentPartition) ([]partialRedeployResult, error) {
	partitionResult := make(chan partialRedeployResult)

	for _, partition := range partitions {
		go performRedeploy(getClient(partition.Partition), partition, partitionResult)
	}

	var allResults []partialRedeployResult
	for i := 0; i < len(partitions); i++ {
		allResults = append(allResults, <-partitionResult)
	}

	return allResults, nil
}

func performRedeploy(deployClient client.ApplicationDeploymentClient, partition DeploymentPartition, partitionResult chan<- partialRedeployResult) {
	if !partition.Cluster.Reachable {
		partitionResult <- getErrorRedeployResults("Cluster is not reachable", partition)
		return
	}

	var applicationRefs []client.ApplicationRef
	for _, info := range partition.DeploymentInfos {
		applicationRefs = append(applicationRefs, *client.NewApplicationRef(info.Namespace, info.Name))
	}

	results, err := deployClient.Redeploy(client.NewRedeployPayload(applicationRefs))

	if err != nil {
		partitionResult <- getErrorRedeployResults(err.Error(), partition)
	} else {
		partitionResult <- newPartialRedeployResults(partition, *results)
	}
}

func getErrorRedeployResults(reason string, partition DeploymentPartition) partialRedeployResult {
	var results []client.RedeployResult

	for _, info := range partition.DeploymentInfos {
		result := client.RedeployResult{
			Success:        false,
			Reason:         reason,
			ApplicationRef: *client.NewApplicationRef(info.Namespace, info.Name),
		}

		results = append(results, result)
	}

	redeployResults := client.RedeployResults{
		Message: reason,
		Success: false,
		Results: results,
	}

	return newPartialRedeployResults(partition, redeployResults)
}

func printFullResults(allResults []partialRedeployResult, out io.Writer) {
	header, rows := getRedeployResultTableContent(allResults)
	DefaultTablePrinter(header, rows, out)
}

func getRedeployResultTableContent(allResults []partialRedeployResult) (string, []string) {
	header := "\x1b[00mSTATUS\x1b[0m\tCLUSTER\tNAMESPACE\tAPPLICATION\tMESSAGE"

	type viewItem struct {
		cluster, namespace, name, reason string
		success                          bool
	}

	var tableData []viewItem

	for _, partitionResult := range allResults {
		for _, redeployResult := range partitionResult.redeployResults.Results {
			item := viewItem{
				cluster:   partitionResult.partition.Cluster.Name,
				namespace: redeployResult.ApplicationRef.Namespace,
				name:      redeployResult.ApplicationRef.Name,
				success:   redeployResult.Success,
				reason:    redeployResult.Reason,
			}

			tableData = append(tableData, item)
		}
	}

	sort.Slice(tableData, func(i, j int) bool {
		nameA := tableData[i].name
		nameB := tableData[j].name
		return strings.Compare(nameA, nameB) < 1
	})

	rows := []string{}
	pattern := "%s\t%s\t%s\t%s\t%s"

	for _, item := range tableData {
		status := "\x1b[32mRedeployd\x1b[0m"
		if !item.success {
			status = "\x1b[31mFailed\x1b[0m"
		}
		result := fmt.Sprintf(pattern, status, item.cluster, item.namespace, item.name, item.reason)
		rows = append(rows, result)
	}

	return header, rows
}

func getRedeployConfirmation(force bool, deployInfos []DeploymentInfo, out io.Writer) bool {
	header, rows := getRedeployConfirmationTableContent(deployInfos)
	DefaultTablePrinter(header, rows, out)

	shouldDeploy := true
	if !force {
		defaultAnswer := len(rows) == 1
		message := fmt.Sprintf("Do you want to redeploy %d application(s) in affiliation %s?", len(rows), AO.Affiliation)
		shouldDeploy = prompt.Confirm(message, defaultAnswer)
	}

	return shouldDeploy
}

func getRedeployConfirmationTableContent(infos []DeploymentInfo) (string, []string) {
	var rows []string
	header := "CLUSTER\tNAMESPACE\tAPPLICATION"
	pattern := "%v\t%v\t%v"
	sort.Slice(infos, func(i, j int) bool {
		return strings.Compare(infos[i].Name, infos[j].Name) != 1
	})
	for _, info := range infos {
		row := fmt.Sprintf(
			pattern,
			info.ClusterName,
			info.Namespace,
			info.Name,
		)
		rows = append(rows, row)
	}
	return header, rows
}