package cmd

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"github.com/skatteetaten/ao/pkg/client"
	"github.com/skatteetaten/ao/pkg/config"
	"github.com/skatteetaten/ao/pkg/log"
	"github.com/spf13/cobra"
	"os"
)

const rootLong = `A command line interface for the Boober API.
  * Deploy one or more ApplicationId (environment/application) to one or more clusters
  * Manipulate AuroraConfig remotely
  * Support modifying AuroraConfig locally
  * Manipulate vaults and secrets`

const (
	helpTemplate = `{{with (or .Long .Short)}}{{. | trimTrailingWhitespaces}}{{end}}

Usage:
  {{.CommandPath}} [command] [flags]{{if gt (len .Aliases) 0}}

Aliases:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

Examples:
{{.Example}}{{end}}{{if hasSubCommandsAnnotation . "actions"}}

OpenShift Action Commands:{{range .Commands}}{{if eq (index .Annotations "type") "actions"}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if hasSubCommandsAnnotation . "remote"}}

Remote AuroraConfig Commands:{{range .Commands}}{{if eq (index .Annotations "type") "remote"}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if hasSubCommandsAnnotation . "local"}}

Local File Commands:{{range .Commands}}{{if eq (index .Annotations "type") "local"}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Commands:{{range .Commands}}{{if (and (eq (index .Annotations "type") "") (ne .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`
)

var (
	pFlagLogLevel  string
	pFlagPrettyLog bool
	pFlagToken     string
	pFlagNoHeader  bool

	// DefaultApiClient will use APICluster from ao config as default values
	// if persistent token and/or server api url is specified these will override default values
	DefaultApiClient *client.ApiClient
	AO               *config.AOConfig
	ConfigLocation   string
)

var RootCmd = &cobra.Command{
	Use:               "ao",
	Short:             "Aurora OpenShift CLI",
	Long:              rootLong,
	PersistentPreRunE: initialize,
}

func init() {
	RootCmd.PersistentFlags().StringVarP(&pFlagLogLevel, "log", "l", "fatal", "Set log level. Valid log levels are [info, debug, warning, error, fatal]")
	RootCmd.PersistentFlags().BoolVarP(&pFlagPrettyLog, "pretty", "p", false, "Pretty print json output for log")
	RootCmd.PersistentFlags().StringVarP(&pFlagToken, "token", "t", "", "Boober authorization token")
	RootCmd.PersistentFlags().BoolVarP(&pFlagNoHeader, "no-header", "", false, "Print tables without headers")
	RootCmd.PersistentFlags().MarkHidden("no-header")

	cobra.AddTemplateFunc("hasSubCommandsAnnotation", func(cmd cobra.Command, annotation string) bool {
		for _, c := range cmd.Commands() {
			t := c.Annotations["type"]
			if t == annotation {
				return true
			}
		}

		return false
	})
	RootCmd.SetHelpTemplate(helpTemplate)
}

func initialize(cmd *cobra.Command, args []string) error {

	// Setting output for cmd.Print methods
	cmd.SetOutput(os.Stdout)
	// Errors will be printed from main
	cmd.SilenceErrors = true
	// Disable print usage when an error occurs
	cmd.SilenceUsage = true

	home, _ := os.LookupEnv("HOME")
	ConfigLocation = home + "/.ao.json"

	err := setLogging(pFlagLogLevel, pFlagPrettyLog)
	if err != nil {
		return err
	}

	aoConfig, err := config.LoadConfigFile(ConfigLocation)
	if err != nil {
		logrus.Error(err)
	}

	if aoConfig == nil {
		logrus.Info("Creating new config")
		aoConfig = &config.DefaultAOConfig
		aoConfig.InitClusters()
		aoConfig.SelectApiCluster()
		err = config.WriteConfig(*aoConfig, ConfigLocation)
		if err != nil {
			return err
		}
	}

	apiCluster := aoConfig.Clusters[aoConfig.APICluster]
	if apiCluster == nil {
		fmt.Printf("Api cluster %s is not available. Check config.\n", aoConfig.APICluster)
		apiCluster = &config.Cluster{}
	}

	api := client.NewApiClient(apiCluster.BooberUrl, apiCluster.Token, aoConfig.Affiliation)

	if aoConfig.Localhost {
		// TODO: Move to config?
		api.Host = "http://localhost:8080"
	}

	if pFlagToken != "" {
		api.Token = pFlagToken
	}

	AO, DefaultApiClient = aoConfig, api

	return nil
}

func setLogging(level string, pretty bool) error {
	logrus.SetOutput(os.Stdout)

	lvl, err := logrus.ParseLevel(level)
	if err != nil {
		return err
	}
	logrus.SetLevel(lvl)

	if pretty {
		logrus.SetFormatter(&log.PrettyFormatter{})
	}

	return nil
}
