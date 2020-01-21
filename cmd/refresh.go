package cmd

import (
	"context"
	"fmt"
	"github.com/machinebox/graphql"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"log"
	"strings"
)

const refreshLong = `Refresh information about application deployment status in mokey for the deployed application 
identified by <applicationDeploymentRef> 
`

var refreshCmd = &cobra.Command{
	Aliases:     []string{"refreshad", "adrefresh"},
	Use:         "refresh <applicationDeploymentRef>",
	Short:       "Refresh information in mokey for the given ApplicationDeploymentRef (via gobo)",
	Long:        refreshLong,
	Annotations: map[string]string{"type": "actions"},
	RunE:        refresh,
}

func init() {
	RootCmd.AddCommand(refreshCmd)
}

type RefreshResponse struct {
	RefreshApplicationDeployment bool
}

const RefreshGraphqlRequest = `
    mutation ($applicationDeploymentId: String!) {
      refreshApplicationDeployment(input: {
        applicationDeploymentId: $applicationDeploymentId
      })
    }
    `

func refresh(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return cmd.Usage()
	}

	client := DefaultApiClient.GetGraphQlClient()
	// make a request
	req := graphql.NewRequest(RefreshGraphqlRequest)
	req.Var("applicationDeploymentId", args[0])
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Add("Authorization", "Bearer "+DefaultApiClient.Token)

	// run it and capture the response
	ctx := context.Background()
	var refreshResponse RefreshResponse
	if err := client.Run(ctx, req, &refreshResponse); err != nil {
		log.Fatal(err)
		if strings.HasSuffix(err.Error(), "not found") {
			return errors.Errorf("could not find application deployment to refresh")
		}
		return err
	}

	parsedResult := parseRefreshResponse(refreshResponse)
	cmd.Println(parsedResult)

	return nil
}

func parseRefreshResponse(refreshResponse RefreshResponse) string {
	var parsedResponse = "Nothing found"
	if refreshResponse.RefreshApplicationDeployment {
		parsedResponse = fmt.Sprintln("Refreshed")
	} else {
		parsedResponse = fmt.Sprintln("Not refreshed")
	}
	return parsedResponse
}
