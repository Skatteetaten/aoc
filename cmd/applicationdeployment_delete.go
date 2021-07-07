package cmd

import (
	"fmt"
	"github.com/skatteetaten/ao/pkg/config"
	"io"
	"sort"
	"strings"

	"github.com/pkg/errors"
	"github.com/skatteetaten/ao/pkg/client"
	"github.com/skatteetaten/ao/pkg/prompt"
	"github.com/skatteetaten/ao/pkg/service"
	"github.com/spf13/cobra"
)

var applicationDeploymentDeleteCmd = &cobra.Command{
	Use:   "delete <applicationDeploymentRef>",
	Short: "Delete application deployment with the given reference",
	RunE:  deleteApplicationDeployment,
}

type partialDeleteResult struct {
	partition     DeploymentPartition
	deleteResults client.DeleteResults
}

// DeploymentPartition structures information about a deployment partition
type DeploymentPartition struct {
	Partition
	DeploymentInfos []DeploymentInfo
}

func newPartialDeleteResults(partition DeploymentPartition, deleteResults client.DeleteResults) partialDeleteResult {
	return partialDeleteResult{
		partition:     partition,
		deleteResults: deleteResults,
	}
}

func init() {
	applicationDeploymentCmd.AddCommand(applicationDeploymentDeleteCmd)
	applicationDeploymentDeleteCmd.Flags().StringVarP(&flagCluster, "cluster", "c", "", "Limit deletion to given cluster name")
	applicationDeploymentDeleteCmd.Flags().BoolVarP(&flagNoPrompt, "yes", "y", false, "Suppress prompts and accept deletion")
	applicationDeploymentDeleteCmd.Flags().BoolVarP(&flagNoPrompt, "no-prompt", "", false, "Suppress prompts and accept deletion")
	applicationDeploymentDeleteCmd.Flags().StringArrayVarP(&flagExcludes, "exclude", "e", []string{}, "Select applications or environments to exclude from deletion")

	applicationDeploymentDeleteCmd.Flags().BoolVarP(&flagNoPrompt, "force", "f", false, "Suppress prompts")
	applicationDeploymentDeleteCmd.Flags().MarkHidden("force")
	applicationDeploymentDeleteCmd.Flags().StringVarP(&flagAuroraConfig, "affiliation", "", "", "Overrides the logged in affiliation")
	applicationDeploymentDeleteCmd.Flags().MarkHidden("affiliation")
}

func deleteApplicationDeployment(cmd *cobra.Command, args []string) error {

	if len(args) > 2 || len(args) < 1 {
		return cmd.Usage()
	}

	err := validateDeleteParams()
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
		return errors.New("No applications to delete")
	}

	filteredDeploymentSpecs, err := service.GetFilteredDeploymentSpecs(apiClient, applications, flagCluster)
	if err != nil {
		return err
	}

	deployInfos, err := getDeployedApplications(getApplicationDeploymentClient, filteredDeploymentSpecs, auroraConfigName, pFlagToken)
	if err != nil {
		return err
	} else if len(deployInfos) == 0 {
		return errors.New("No applications to delete")
	}

	partitions, err := createDeploymentPartitions(auroraConfigName, pFlagToken, AO.Clusters, deployInfos)
	if err != nil {
		return err
	}

	if !getDeleteConfirmation(flagNoPrompt, deployInfos, cmd.OutOrStdout()) {
		return errors.New("No applications to delete")
	}

	fullResults, err := deleteFromReachableClusters(getApplicationDeploymentClient, partitions)
	if err != nil {
		return err
	}

	printFullDeleteResults(fullResults, cmd.OutOrStdout())

	for _, result := range fullResults {
		if !result.deleteResults.Success {
			return errors.New("One or more delete operations failed")
		}
	}

	return nil
}

func validateDeleteParams() error {
	if flagCluster != "" {
		if _, exists := AO.Clusters[flagCluster]; !exists {
			return errors.New(fmt.Sprintf("No such cluster %s", flagCluster))
		}
	}

	return nil
}

func deleteFromReachableClusters(getClient func(partition Partition) client.ApplicationDeploymentClient, partitions []DeploymentPartition) ([]partialDeleteResult, error) {
	partitionResult := make(chan partialDeleteResult)

	for _, partition := range partitions {
		go performDelete(getClient(partition.Partition), partition, partitionResult)
	}

	var allResults []partialDeleteResult
	for i := 0; i < len(partitions); i++ {
		allResults = append(allResults, <-partitionResult)
	}

	return allResults, nil
}

func performDelete(deployClient client.ApplicationDeploymentClient, partition DeploymentPartition, partitionResult chan<- partialDeleteResult) {
	if !partition.Cluster.Reachable {
		partitionResult <- getErrorDeleteResults("Cluster is not reachable", partition)
		return
	}

	var applicationRefs []client.ApplicationRef
	for _, info := range partition.DeploymentInfos {
		applicationRefs = append(applicationRefs, *client.NewApplicationRef(info.Namespace, info.Name))
	}

	results, err := deployClient.Delete(client.NewDeletePayload(applicationRefs))

	if err != nil {
		partitionResult <- getErrorDeleteResults(err.Error(), partition)
	} else {
		partitionResult <- newPartialDeleteResults(partition, *results)
	}
}

func getErrorDeleteResults(reason string, partition DeploymentPartition) partialDeleteResult {
	var results []client.DeleteResult

	for _, info := range partition.DeploymentInfos {
		result := client.DeleteResult{
			Success:        false,
			Reason:         reason,
			ApplicationRef: *client.NewApplicationRef(info.Namespace, info.Name),
		}

		results = append(results, result)
	}

	deleteResults := client.DeleteResults{
		Message: reason,
		Success: false,
		Results: results,
	}

	return newPartialDeleteResults(partition, deleteResults)
}

func printFullDeleteResults(allResults []partialDeleteResult, out io.Writer) {
	header, rows := getDeleteResultTableContent(allResults)
	DefaultTablePrinter(header, rows, out)
}

func getDeleteResultTableContent(allResults []partialDeleteResult) (string, []string) {
	header := "\x1b[00mSTATUS\x1b[0m\tCLUSTER\tNAMESPACE\tAPPLICATION\tMESSAGE"

	type viewItem struct {
		cluster, namespace, name, reason string
		success                          bool
	}

	var tableData []viewItem

	for _, partitionResult := range allResults {
		for _, deleteResult := range partitionResult.deleteResults.Results {
			item := viewItem{
				cluster:   partitionResult.partition.Cluster.Name,
				namespace: deleteResult.ApplicationRef.Namespace,
				name:      deleteResult.ApplicationRef.Name,
				success:   deleteResult.Success,
				reason:    deleteResult.Reason,
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
		status := "\x1b[32mDeleted\x1b[0m"
		if !item.success {
			status = "\x1b[31mFailed\x1b[0m"
		}
		result := fmt.Sprintf(pattern, status, item.cluster, item.namespace, item.name, item.reason)
		rows = append(rows, result)
	}

	return header, rows
}

func getDeleteConfirmation(force bool, deployInfos []DeploymentInfo, out io.Writer) bool {
	header, rows := getDeleteConfirmationTableContent(deployInfos)
	DefaultTablePrinter(header, rows, out)

	shouldDeploy := true
	if !force {
		defaultAnswer := len(rows) == 1
		message := fmt.Sprintf("Do you want to delete %d application(s) in affiliation %s?", len(rows), AO.Affiliation)
		shouldDeploy = prompt.Confirm(message, defaultAnswer)
	}

	return shouldDeploy
}

func getDeleteConfirmationTableContent(infos []DeploymentInfo) (string, []string) {
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
