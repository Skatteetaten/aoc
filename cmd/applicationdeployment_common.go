package cmd

import (
	"fmt"
	"github.com/spf13/cobra"

	"github.com/pkg/errors"
	"github.com/skatteetaten/ao/pkg/client"
	"github.com/skatteetaten/ao/pkg/config"
	"github.com/skatteetaten/ao/pkg/deploymentspec"
)

var (
	flagAuroraConfig string
	flagOverrides    []string
	flagNoPrompt     bool
	flagVersion      string
	flagCluster      string
	flagExcludes     []string
)

var applicationDeploymentCmd = &cobra.Command{
	Use:         "applicationdeployment",
	Aliases:     []string{"ad"},
	Short:       "Perform operations on an application deployment",
	Annotations: map[string]string{"type": "actions"},
}

// DeploymentInfo structures information about a deployment
type DeploymentInfo struct {
	Namespace   string
	Name        string
	ClusterName string
}

// Partition structures information about a Cluster+AuroraConfigName+Environment-partition
type Partition struct {
	Cluster          config.Cluster
	AuroraConfigName string
	Environment      string
	OverrideToken    string
}

// DeploySpecPartition structures deployment specifications in a partition
type DeploySpecPartition struct {
	Partition
	DeploySpecs []deploymentspec.DeploymentSpec
}

func init() {
	RootCmd.AddCommand(applicationDeploymentCmd)
}

func newDeploymentInfo(namespace, name, cluster string) *DeploymentInfo {
	return &DeploymentInfo{
		Namespace:   namespace,
		Name:        name,
		ClusterName: cluster,
	}
}

func newDeploySpecPartition(deploySpecs []deploymentspec.DeploymentSpec, cluster config.Cluster, auroraConfig string, environment string, overrideToken string) *DeploySpecPartition {
	return &DeploySpecPartition{
		DeploySpecs: deploySpecs,
		Partition: Partition{
			Cluster:          cluster,
			AuroraConfigName: auroraConfig,
			Environment:      environment,
			OverrideToken:    overrideToken,
		},
	}
}

func createDeploySpecPartitions(auroraConfig, overrideToken string, clusters map[string]*config.Cluster, deploySpecs []deploymentspec.DeploymentSpec) ([]DeploySpecPartition, error) {
	type deploySpecPartitionID struct {
		envName, clusterName string
	}

	partitionMap := make(map[deploySpecPartitionID]*DeploySpecPartition)

	for _, spec := range deploySpecs {
		clusterName := spec.Cluster()
		envName := spec.Environment()

		partitionID := deploySpecPartitionID{clusterName, envName}

		if _, exists := partitionMap[partitionID]; !exists {
			if _, exists := clusters[clusterName]; !exists {
				return nil, errors.New(fmt.Sprintf("No such cluster %s", clusterName))
			}
			cluster := clusters[clusterName]
			partition := newDeploySpecPartition([]deploymentspec.DeploymentSpec{}, *cluster, auroraConfig, envName, overrideToken)
			partitionMap[partitionID] = partition
		}

		partitionMap[partitionID].DeploySpecs = append(partitionMap[partitionID].DeploySpecs, spec)
	}

	partitions := make([]DeploySpecPartition, len(partitionMap))

	idx := 0
	for _, partition := range partitionMap {
		partitions[idx] = *partition
		idx++
	}

	return partitions, nil
}

func getApplicationDeploymentClient(partition Partition) client.ApplicationDeploymentClient {
	var cli *client.APIClient
	if AO.Localhost {
		cli = DefaultAPIClient
		cli.Affiliation = partition.AuroraConfigName
	} else {
		token := partition.Cluster.Token
		if partition.OverrideToken != "" {
			token = partition.OverrideToken
		}
		cli = client.NewAPIClient(partition.Cluster.BooberURL, partition.Cluster.GoboURL, token, partition.AuroraConfigName, DefaultAPIClient.RefName, DefaultAPIClient.Korrelasjonsid)
	}

	return cli
}
