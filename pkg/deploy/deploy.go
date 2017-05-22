package deploy

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/skatteetaten/aoc/pkg/cmdoptions"
	"github.com/skatteetaten/aoc/pkg/configuration"
	"github.com/skatteetaten/aoc/pkg/jsonutil"
	"github.com/skatteetaten/aoc/pkg/serverapi_v2"
	"net/http"
)

type DeployCommand struct {
	Affiliation string                      `json:"affiliation"`
	SetupParams jsonutil.SetupParamsPayload `json:"setupParams"`
}

type DeployClass struct {
	configuration configuration.ConfigurationClass
	initDone      bool
}

func (deployClass *DeployClass) Init() (err error) {
	if deployClass.initDone {
		return
	}
	deployClass.initDone = true
	return
}

func (deployClass *DeployClass) getAffiliation() (affiliation string) {
	if deployClass.configuration.GetOpenshiftConfig() != nil {
		affiliation = deployClass.configuration.GetOpenshiftConfig().Affiliation
	}
	return
}

func (deployClass *DeployClass) ExecuteDeploy(args []string, overrideFiles []string, applist []string, envList []string,
	persistentOptions *cmdoptions.CommonCommandOptions, localDryRun bool) (output string, err error) {

	error := validateDeploy(args)
	if error != nil {
		return
	}
	deployClass.Init()
	if !serverapi_v2.ValidateLogin(deployClass.configuration.GetOpenshiftConfig()) {
		return "", errors.New("Not logged in, please use aoc login")
	}

	var overrideJson []string // = args[1:]

	var affiliation = deployClass.getAffiliation()
	json, error := generateJson(envList, applist, overrideJson, overrideFiles, affiliation, persistentOptions.DryRun)

	var apiEndpoint string = "/affiliation/" + affiliation + "/deploy"
	var responses map[string]string
	var applicationResults []serverapi_v2.ApplicationResult

	if error != nil {
		return
	} else {
		if localDryRun {
			return fmt.Sprintf("%v", string(jsonutil.PrettyPrintJson(json))), nil
		} else {
			responses, err = serverapi_v2.CallApi(http.MethodPut, apiEndpoint, json, persistentOptions.ShowConfig,
				persistentOptions.ShowObjects, false, persistentOptions.Localhost,
				persistentOptions.Verbose, deployClass.configuration.GetOpenshiftConfig(), persistentOptions.DryRun, persistentOptions.Debug)
			if err != nil {
				for server := range responses {
					response, err := serverapi_v2.ParseResponse(responses[server])
					if err != nil {
						return "", err
					}
					if !response.Success {
						output, err = serverapi_v2.ResponsItems2MessageString(response)
					}
				}
				return output, nil
			}
			for server := range responses {
				response, err := serverapi_v2.ParseResponse(responses[server])
				if err != nil {
					return "", err
				}
				if response.Success {
					applicationResults, err = serverapi_v2.ResponseItems2ApplicationResults(response)
				}
				for applicationResultIndex := range applicationResults {
					out, err := serverapi_v2.ApplicationResult2MessageString(applicationResults[applicationResultIndex])
					if err != nil {
						return out, err
					}
					output += out
				}
			}

		}
	}

	return
}

func validateDeploy(args []string) (error error) {
	if len(args) != 0 {
		error = errors.New("Usage: aoc deploy <env>")
	}

	return
}

func generateJson(envList []string, appList []string, overrideJson []string,
	overrideFiles []string, affiliation string, dryRun bool) (jsonStr string, error error) {
	//var apiData ApiInferface
	var setupCommand DeployCommand

	if len(appList) != 0 {
		setupCommand.SetupParams.Apps = appList
	} else {
		setupCommand.SetupParams.Apps = make([]string, 0)
	}
	if len(envList) != 0 {
		setupCommand.SetupParams.Envs = envList
	} else {
		setupCommand.SetupParams.Envs = make([]string, 0)
	}

	setupCommand.SetupParams.DryRun = dryRun
	//setupCommand.SetupParams.Overrides = jsonutil.Overrides2map(overrideJson, overrideFiles)
	setupCommand.SetupParams.Overrides = make(map[string]json.RawMessage, 0)
	setupCommand.Affiliation = affiliation

	var jsonByte []byte

	jsonByte, error = json.Marshal(setupCommand)
	if !(error == nil) {
		return "", errors.New(fmt.Sprintf("Internal error in marshalling SetupCommand: %v\n", error.Error()))
	}

	jsonStr = string(jsonByte)
	return

}
