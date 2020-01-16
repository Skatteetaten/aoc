package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/skatteetaten/ao/pkg/config"
)

const (
	BooberApiVersion    = "/v1"
	ErrAccessDenied     = "Access Denied"
	ErrfTokenHasExpired = "Token has expired for (%s). Please login: ao login <affiliation>"
)

type Doer interface {
	Do(method string, endpoint string, payload []byte) (*BooberResponse, error)
	DoWithHeader(method string, endpoint string, header map[string]string, payload []byte) (*ResponseBundle, error)
}

type ResponseBundle struct {
	BooberResponse *BooberResponse
	HttpResponse   *http.Response
}

type ApiClient struct {
	Host        string
	GoboHost    string
	Token       string
	Affiliation string
	RefName     string
}

func NewApiClientDefaultRef(host, token, affiliation string) *ApiClient {
	return NewApiClient(host, token, affiliation, "master")
}

func NewApiClient(host, token, affiliation, refName string) *ApiClient {
	return &ApiClient{
		Host:        host,
		Token:       token,
		Affiliation: affiliation,
		RefName:     refName,
	}
}

func (api *ApiClient) Do(method string, endpoint string, payload []byte) (*BooberResponse, error) {
	bundle, err := api.DoWithHeader(method, endpoint, nil, payload)
	if bundle == nil {
		return nil, err
	}
	return bundle.BooberResponse, nil
}

func (api *ApiClient) DoWithHeader(method string, endpoint string, header map[string]string, payload []byte) (*ResponseBundle, error) {

	url := api.Host + BooberApiVersion + endpoint
	logrus.WithFields(logrus.Fields{
		"method": method,
		"url":    url,
	}).Info("Request")

	if len(payload) == 0 {
		logrus.Debug("No payload")
	} else {
		logrus.Debug("Payload", string(payload))
	}

	req, err := http.NewRequest(method, url, bytes.NewBuffer(payload))
	if err != nil {
		return nil, err
	}

	userAgentHeader := fmt.Sprintf("Go-http-client/1.1 ao/%s", config.Version)
	req.Header.Set("User-Agent", userAgentHeader)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+api.Token)
	req.Header.Set("Ref-Name", api.RefName)

	for key, value := range header {
		req.Header.Set(key, value)
	}

	client := http.DefaultClient
	res, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "Error connecting to api")
	}

	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	var fields logrus.Fields
	err = json.Unmarshal(body, &fields)
	if err != nil {
		logrus.Debug("Response is not JSON: ", string(body))
	}

	if res.StatusCode > 399 {
		logrus.WithFields(fields).Error("Request Error")
	}

	switch res.StatusCode {
	case http.StatusNotFound:
		return nil, errors.Errorf("Resource %s not found", BooberApiVersion+endpoint)
	case http.StatusForbidden:
		return nil, handleForbiddenError(body, api.Host)
	case http.StatusInternalServerError:
		return nil, handleInternalServerError(body, url)
	case http.StatusServiceUnavailable:
		return nil, errors.Errorf("Service unavailable %s", api.Host)
	case http.StatusPreconditionFailed:
		return nil, errors.Errorf("File has changed since edit")
	}

	var booberRes BooberResponse
	if len(body) > 0 {
		err = json.Unmarshal(body, &booberRes)
		if err != nil {
			return nil, errors.Wrap(err, "response unmarshal")
		}
	}

	logrus.WithFields(logrus.Fields{
		"status":  res.StatusCode,
		"url":     url,
		"success": booberRes.Success,
		"message": booberRes.Message,
		"count":   booberRes.Count,
	}).Info("Response")

	logrus.WithFields(fields).Debug("ResponseBody")

	return &ResponseBundle{
		BooberResponse: &booberRes,
		HttpResponse:   res,
	}, nil
}

func handleInternalServerError(body []byte, url string) error {
	internalError := struct {
		Message   string `json:"message"`
		Exception string `json:"exception"`
	}{}
	err := json.Unmarshal(body, &internalError)
	if err != nil {
		return err
	}

	return errors.Errorf("Unexpected error from %s\nMessage: %s\nException: %s", url, internalError.Message, internalError.Exception)
}

func handleForbiddenError(body []byte, host string) error {
	forbiddenError := struct {
		Message string `json:"message"`
	}{}
	err := json.Unmarshal(body, &forbiddenError)
	if err != nil {
		return err
	}

	if forbiddenError.Message == ErrAccessDenied {
		return errors.Errorf(ErrfTokenHasExpired, host)
	}

	return errors.Errorf("Forbidden: %s", forbiddenError.Message)
}
