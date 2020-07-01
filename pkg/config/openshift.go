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

const authenticationURLSuffix = "/oauth/authorize?client_id=openshift-challenging-client&response_type=token"

var (
	transport = http.Transport{
		Dial: func(network, addr string) (net.Conn, error) {
			timeout := time.Duration(1 * time.Second)
			return net.DialTimeout(network, addr, timeout)
		},
		//TODO: Change to InsecureSkipVerify: false when presence of CA certificates are validated on platforms using ao
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	client = http.Client{
		Transport: &transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
)

// Cluster holds information of Openshift cluster
type Cluster struct {
	Name      string `json:"name"`
	URL       string `json:"url"`
	Token     string `json:"token"`
	Reachable bool   `json:"reachable"`
	BooberURL string `json:"booberUrl"`
	GoboURL   string `json:"goboUrl"`
}

// InitClusters initializes Cluster objects for AOConfig
func (ao *AOConfig) InitClusters() {
	ao.Clusters = make(map[string]*Cluster)
	ch := make(chan *Cluster)

	for _, cluster := range ao.AvailableClusters {
		name := cluster
		booberURL := fmt.Sprintf(ao.BooberURLPattern, name)
		clusterURL := fmt.Sprintf(ao.ClusterURLPattern, name)

		//TODO: Change as more clusters get TLS enabled gobo API end points
		goboURL := fmt.Sprintf(ao.GoboURLPattern, name)
		if name == "utv" {
			goboURL = fmt.Sprintf(ao.GoboURLSecurePattern, name)
		}

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
				URL:       fmt.Sprintf(ao.ClusterURLPattern, name),
				Reachable: reachable,
				BooberURL: booberURL,
				GoboURL:   goboURL,
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

// HasValidToken performs a test call to verify validity of token
func (c *Cluster) HasValidToken() bool {
	if c.Token == "" {
		return false
	}

	clusterURL := fmt.Sprintf("%s/%s", c.URL, "oapi")
	req, err := http.NewRequest("GET", clusterURL, nil)
	if err != nil {
		return false
	}

	req.Header.Add("Authorization", "Bearer "+c.Token)
	logrus.WithField("url", clusterURL).Debug("Check for valid token")
	resp, err := client.Do(req)
	if err != nil {
		return false
	}

	if resp.StatusCode != http.StatusOK {
		return false
	}
	return true
}

// GetToken gets a token for a host
func GetToken(host string, username string, password string) (string, error) {
	clusterURL := host + authenticationURLSuffix
	resp, err := getBasicAuth(clusterURL, username, password)
	if err != nil {
		return "", err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return "", errors.New("Not authorized")
	}
	redirectURL := resp.Header.Get("Location")
	token, err := oauthAuthorizeResult(redirectURL)
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
