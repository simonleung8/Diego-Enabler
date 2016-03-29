package listhelpers

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"os"

	"github.com/cloudfoundry-incubator/diego-enabler/api"
	"github.com/cloudfoundry-incubator/diego-enabler/models"
	"github.com/cloudfoundry-incubator/diego-enabler/thingdoer"
	"github.com/cloudfoundry-incubator/diego-enabler/ui"
	"github.com/cloudfoundry/cli/cf/terminal"
	"github.com/cloudfoundry/cli/cf/trace"
	"github.com/cloudfoundry/cli/plugin"
)

func OrgNotFound(org string) error {
	return fmt.Errorf("Organization not found: %s", org)
}

type ListApps struct {
	Opts            ListAppOpts
	Runtime         ui.Runtime
	AppsGetterFunc  thingdoer.AppsGetterFunc
	ListAppsCommand *ui.ListAppsCommand
}

type ListAppOpts struct {
	Organization string
}

func (cmd *ListApps) Execute(cliConnection plugin.CliConnection) error {

	if err := verifyLoggedIn(cliConnection); err != nil {
		return err
	}

	accessToken, err := cliConnection.AccessToken()
	if err != nil {
		return err
	}

	cmd.ListAppsCommand.BeforeAll()

	pageParser := api.PageParser{}
	appsParser := models.ApplicationsParser{}
	spacesParser := models.SpacesParser{}

	apiEndpoint, err := cliConnection.ApiEndpoint()
	if err != nil {
		return err
	}

	apiClient, err := api.NewApiClient(apiEndpoint, accessToken)
	if err != nil {
		return err
	}

	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	appRequestFactory := apiClient.HandleFiltersAndParameters(
		apiClient.Authorize(apiClient.NewGetAppsRequest),
	)

	apps, err := cmd.AppsGetterFunc(
		appsParser,
		&api.PaginatedRequester{
			RequestFactory: appRequestFactory,
			Client:         httpClient,
			PageParser:     pageParser,
		},
	)
	if err != nil {
		return err
	}

	spaceRequestFactory := apiClient.HandleFiltersAndParameters(
		apiClient.Authorize(apiClient.NewGetSpacesRequest),
	)

	spaces, err := thingdoer.Spaces(
		spacesParser,
		&api.PaginatedRequester{
			RequestFactory: spaceRequestFactory,
			Client:         httpClient,
			PageParser:     pageParser,
		},
	)
	if err != nil {
		return err
	}

	spaceMap := make(map[string]models.Space)
	for _, space := range spaces {
		spaceMap[space.Guid] = space
	}

	var appPrinters []ui.ApplicationPrinter
	for _, a := range apps {
		appPrinters = append(appPrinters, &appPrinter{
			app:    a,
			spaces: spaceMap,
		})
	}

	cmd.ListAppsCommand.AfterAll(appPrinters)

	return nil
}

func NewListAppsCommand(cliConnection plugin.CliConnection, orgName string, runtime ui.Runtime) (ui.ListAppsCommand, error) {
	username, err := cliConnection.Username()
	if err != nil {
		return ui.ListAppsCommand{}, err
	}

	traceEnv := os.Getenv("CF_TRACE")
	traceLogger := trace.NewLogger(false, traceEnv, "")
	tUI := terminal.NewUI(os.Stdin, terminal.NewTeePrinter(), traceLogger)

	cmd := ui.ListAppsCommand{
		Username:     username,
		Organization: orgName,
		UI:           tUI,
		Runtime:      runtime,
	}
	return cmd, nil
}
