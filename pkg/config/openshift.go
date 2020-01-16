package config

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const authenticationUrlSuffix = "/oauth/authorize?client_id=openshift-challenging-client&response_type=token"

var (
	transport = http.Transport{
		Dial: func(network, addr string) (net.Conn, error) {
			timeout := time.Duration(1 * time.Second)
			return net.DialTimeout(network, addr, timeout)
		},
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	client = http.Client{
		Transport: &transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
)

type Cluster struct {
	Name      string `json:"name"`
	Url       string `json:"url"`
	Token     string `json:"token"`
	Reachable bool   `json:"reachable"`
	BooberUrl string `json:"booberUrl"`
	GoboUrl   string `json:"goboUrl"`
}

func (ao *AOConfig) InitClusters() {
	ao.Clusters = make(map[string]*Cluster)
	ch := make(chan *Cluster)

	for _, cluster := range ao.AvailableClusters {
		name := cluster
		booberURL := fmt.Sprintf(ao.BooberUrlPattern, name)
		clusterURL := fmt.Sprintf(ao.ClusterUrlPattern, name)
		goboUrl := fmt.Sprintf(ao.GoboUrlPattern, name)
		go func() {
			reachable := false
			resp, err := client.Get(booberURL)
			if err == nil && resp != nil && resp.StatusCode < 500 {
				resp, err = client.Get(clusterURL)
				if err == nil && resp != nil && resp.StatusCode < 500 {
					reachable = true
				}
			}

			logrus.WithField("reachable", reachable).Info(booberURL)
			ch <- &Cluster{
				Name:      name,
				Url:       fmt.Sprintf(ao.ClusterUrlPattern, name),
				Reachable: reachable,
				BooberUrl: booberURL,
				GoboUrl:   goboUrl,
			}
		}()
	}

	for {
		select {
		case c := <-ch:
			ao.Clusters[c.Name] = c
			if len(ao.Clusters) == len(ao.AvailableClusters) {
				return
			}
		}
	}
}

func (c *Cluster) HasValidToken() bool {
	if c.Token == "" {
		return false
	}

	clusterUrl := fmt.Sprintf("%s/%s", c.Url, "oapi")
	req, err := http.NewRequest("GET", clusterUrl, nil)
	if err != nil {
		return false
	}

	req.Header.Add("Authorization", "Bearer "+c.Token)
	logrus.WithField("url", clusterUrl).Debug("Check for valid token")
	resp, err := client.Do(req)
	if err != nil {
		return false
	}

	if resp.StatusCode != http.StatusOK {
		return false
	}
	return true
}

func GetToken(host string, username string, password string) (string, error) {
	clusterUrl := host + authenticationUrlSuffix
	resp, err := getBasicAuth(clusterUrl, username, password)
	if err != nil {
		return "", err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return "", errors.New("Not authorized")
	}
	redirectUrl := resp.Header.Get("Location")
	token, err := oauthAuthorizeResult(redirectUrl)
	if err != nil {
		return "", err
	}
	return token, nil
}

func getBasicAuth(url string, username string, password string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(username, password)
	return client.Do(req)
}

func oauthAuthorizeResult(location string) (string, error) {
	u, err := url.Parse(location)
	if err != nil {
		return "", err
	}

	if errorCode := u.Query().Get("error"); len(errorCode) > 0 {
		errorDescription := u.Query().Get("error_description")
		return "", errors.New(errorCode + " " + errorDescription)
	}

	fragmentValues, err := url.ParseQuery(u.Fragment)
	if err != nil {
		return "", err
	}
	accessToken := fragmentValues.Get("access_token")
	if len(accessToken) == 0 {
		return "", errors.New("token is empty")
	}
	return accessToken, nil
}
