package client

import (
	"fmt"
	"github.com/machinebox/graphql"
)

func (api *ApiClient) GetGraphQlClient() *graphql.Client {
	endpoint := fmt.Sprintf("%s/graphql", api.GoboHost)
	client := graphql.NewClient(endpoint)
	return client
}
