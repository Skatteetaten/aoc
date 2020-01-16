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

var affiliationsCmd = &cobra.Command{
	Use:   "affiliations",
	Short: "List available affiliations (via gobo)",
	RunE:  affiliations,
}

func init() {
	RootCmd.AddCommand(affiliationsCmd)
}

type AffiliationsResponse struct {
	Affiliations struct {
		TotalCount int
		Edges      []struct {
			Node struct {
				Name string
			}
		}
	}
}

const AffiliationsGraphqlRequest = `
	    {
	      affiliations {
	        totalCount
	        edges {
	          cursor
	          node {
	            name
	          }
	        }
	      }
	    }
	    `

func affiliations(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		return cmd.Usage()
	}

	client := DefaultApiClient.GetGraphQlClient()
	// make a request
	req := graphql.NewRequest(AffiliationsGraphqlRequest)
	req.Header.Set("Cache-Control", "no-cache")

	// run it and capture the response
	ctx := context.Background()
	var affiliationsResponse AffiliationsResponse
	if err := client.Run(ctx, req, &affiliationsResponse); err != nil {
		log.Fatal(err)
		if strings.HasSuffix(err.Error(), "not found") {
			return errors.Errorf("could not find affiliations")
		}
		return err
	}

	parsedResult := parseAffiliationsResponse(affiliationsResponse)
	cmd.Println(parsedResult)

	return nil
}

func parseAffiliationsResponse(affiliationsResponse AffiliationsResponse) string {
	var affiliations = affiliationsResponse.Affiliations
	var parsedResponse = "Nothing found"
	if affiliations.TotalCount > 0 {
		parsedResponse = fmt.Sprintf("Count: %v\n\n", affiliations.TotalCount)
		for _, affiliation := range affiliations.Edges {
			parsedResponse += fmt.Sprintf("%v\n", affiliation.Node.Name)
		}
	}
	return parsedResponse
}
