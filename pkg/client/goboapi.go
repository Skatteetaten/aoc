package client

import (
	"context"
	"fmt"
	"github.com/machinebox/graphql"
	"github.com/sirupsen/logrus"
)

// RunGraphQl performs a GraphQl based API call
func (api *APIClient) RunGraphQl(graphQlRequest string, response interface{}) error {
	client := api.getGraphQlClient()
	req := api.newRequest(graphQlRequest)
	ctx := context.Background()

	if err := client.Run(ctx, req, response); err != nil {
		return err
	}
	return nil
}

func (api *APIClient) RunGraphQlMutation(graphQlRequest *graphql.Request, response interface{}) error {
	client := api.getGraphQlClient()
	ctx := context.Background()
	graphQlRequest.Header.Set("Cache-Control", "no-cache")
	graphQlRequest.Header.Add("Authorization", "Bearer "+api.Token)

	if err := client.Run(ctx, graphQlRequest, response); err != nil {
		return err
	}
	return nil
}

func (api *APIClient) getGraphQlClient() *graphql.Client {
	endpoint := fmt.Sprintf("%s/graphql", api.GoboHost)
	client := graphql.NewClient(endpoint)
	logrus.Debugf("Got graphql client for %s", api.GoboHost)
	client.Log = func(logEntry string) { logrus.Debug(logEntry) }
	return client
}

func (api *APIClient) newRequest(graphqlRequest string) *graphql.Request {
	req := graphql.NewRequest(graphqlRequest)
	req.Header.Set("Cache-Control", "no-cache")
	return req
}
