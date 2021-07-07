package cmd

import (
	"github.com/skatteetaten/ao/pkg/deploymentspec"
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_checkForDuplicateSpecs_single(t *testing.T) {
	deploymentSpecs := []deploymentspec.DeploymentSpec{
		deploymentspec.NewDeploymentSpec("app1", "env1", "cluster1", "1"),
	}

	err := checkForDuplicateSpecs(deploymentSpecs)

	if err != nil {
		t.Fatal(err)
	}
}

func Test_checkForDuplicateSpecs_severalunique(t *testing.T) {
	deploymentSpecs := []deploymentspec.DeploymentSpec{
		deploymentspec.NewDeploymentSpec("app1", "env1", "cluster1", "1"),
		deploymentspec.NewDeploymentSpec("app1", "env2", "cluster1", "1"),
		deploymentspec.NewDeploymentSpec("app2", "env1", "cluster1", "1"),
		deploymentspec.NewDeploymentSpec("app1", "env1", "cluster2", "1"),
	}

	err := checkForDuplicateSpecs(deploymentSpecs)

	if err != nil {
		t.Fatal(err)
	}
}

func Test_checkForDuplicateSpecs_duplicate1(t *testing.T) {
	deploymentSpecs := []deploymentspec.DeploymentSpec{
		deploymentspec.NewDeploymentSpec("app1", "env1", "cluster1", "1"),
		deploymentspec.NewDeploymentSpec("app1", "env1", "cluster1", "2"),
	}

	err := checkForDuplicateSpecs(deploymentSpecs)

	assert.Contains(t, err.Error(), "cluster1 env1 app1")
}

func Test_checkForDuplicateSpecs_duplicate2(t *testing.T) {
	deploymentSpecs := []deploymentspec.DeploymentSpec{
		deploymentspec.NewDeploymentSpec("app1", "env1", "cluster1", "1"),
		deploymentspec.NewDeploymentSpec("app2", "env1", "cluster1", "2"),
		deploymentspec.NewDeploymentSpec("app1", "env1", "cluster2", "3"),
		deploymentspec.NewDeploymentSpec("app3", "env1", "cluster1", "4"),
		deploymentspec.NewDeploymentSpec("app1", "env1", "cluster2", "5"),
	}

	err := checkForDuplicateSpecs(deploymentSpecs)

	assert.Contains(t, err.Error(), "cluster2 env1 app1")
}
