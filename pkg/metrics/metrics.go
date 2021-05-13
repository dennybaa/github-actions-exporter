package metrics

import (
	"github-actions-exporter/pkg/config"
	"context"
	"fmt"
	"strings"
	"log"
	"net/http"
	"net/url"

	"github.com/google/go-github/v33/github"
	//"github.com/gregjones/httpcache"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/oauth2"
	"github.com/bradleyfalzon/ghinstallation"
)

var (
	client *github.Client
	err    error
)

// InitMetrics - register metrics in prometheus lib and start func for monitor
func InitMetrics() {
	prometheus.MustRegister(runnersGauge)
	prometheus.MustRegister(runnersOrganizationGauge)
	prometheus.MustRegister(workflowRunStatusGauge)
	prometheus.MustRegister(workflowRunStatusDeprecatedGauge)
	prometheus.MustRegister(workflowRunDurationGauge)
	prometheus.MustRegister(workflowBillGauge)

	client, err = NewClient()
	if err != nil {		
		log.Fatalln("Error: Client creation failed." + err.Error())
	}

	go workflowCache()

	for {
		if workflows != nil {
			break
		}
	}

	go getBillableFromGithub()
	go getRunnersFromGithub()
	go getRunnersOrganizationFromGithub()
	go getWorkflowRunsFromGithub()
}

// NewClient creates a Github Client
func  NewClient() (*github.Client, error) {
	var (
		httpClient *http.Client
		client     *github.Client
		transport  http.RoundTripper
	)
	//githubBaseURL := "https://github.com/"
	if len(config.Github.Token) > 0 {
		log.Printf("authenticating with Github Token")
		transport = oauth2.NewClient(context.Background(), oauth2.StaticTokenSource(&oauth2.Token{AccessToken: config.Github.Token})).Transport
		httpClient = &http.Client{Transport: transport}
	} else {
		log.Printf("authenticating with Github App")
		tr, err := ghinstallation.NewKeyFromFile(http.DefaultTransport, config.Github.AppID, config.Github.AppInstallationID, config.Github.AppPrivateKey)
		if err != nil {
			return nil, fmt.Errorf("authentication failed: %v", err)
		}
		if config.Github.ApiUrl != "api.github.com" {
			githubAPIURL, err := getEnterpriseApiUrl(config.Github.ApiUrl)
			if err != nil {
				return nil, fmt.Errorf("enterprise url incorrect: %v", err)
			}
			tr.BaseURL = githubAPIURL
		}
		httpClient = &http.Client{Transport: tr}
	}

	if config.Github.ApiUrl != "api.github.com" {
		var err error
		client, err = github.NewEnterpriseClient(config.Github.ApiUrl, config.Github.ApiUrl, httpClient)
		if err != nil {
			return nil, fmt.Errorf("enterprise client creation failed: %v", err)
		}
		//githubBaseURL = fmt.Sprintf("%s://%s%s", client.BaseURL.Scheme, client.BaseURL.Host, strings.TrimSuffix(client.BaseURL.Path, "api/v3/"))
	} else {
		client = github.NewClient(httpClient)
	}

	return client, nil
}

func getEnterpriseApiUrl(baseURL string) (string, error) {
	baseEndpoint, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	if !strings.HasSuffix(baseEndpoint.Path, "/") {
		baseEndpoint.Path += "/"
	}
	if !strings.HasSuffix(baseEndpoint.Path, "/api/v3/") &&
		!strings.HasPrefix(baseEndpoint.Host, "api.") &&
		!strings.Contains(baseEndpoint.Host, ".api.") {
		baseEndpoint.Path += "api/v3/"
	}

	// Trim trailing slash, otherwise there's double slash added to token endpoint
	return fmt.Sprintf("%s://%s%s", baseEndpoint.Scheme, baseEndpoint.Host, strings.TrimSuffix(baseEndpoint.Path, "/")), nil
}