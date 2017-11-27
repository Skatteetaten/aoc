package cmd

import (
	"fmt"

	"strings"

	"encoding/json"
	"github.com/pkg/errors"
	"github.com/skatteetaten/ao/pkg/client"
	"github.com/skatteetaten/ao/pkg/editor"
	"github.com/skatteetaten/ao/pkg/prompt"
	"github.com/spf13/cobra"
	"io/ioutil"
	"os"
	"path"
	"sort"
)

var (
	flagAddGroup    string
	flagRemoveGroup string

	ErrEmptyGroups            = errors.New("Cannot find groups in permissions")
	ErrNotValidSecretArgument = errors.New("not a valid argument, must be <vaultname/secret>")
)

var (
	vaultCmd = &cobra.Command{
		Use:         "vault",
		Short:       "Create and perform operations on a vault",
		Annotations: map[string]string{"type": "remote"},
	}

	vaultAddSecretCmd = &cobra.Command{
		Use:   "add-secret <vaultname> <[folder/]file>",
		Short: "Add a secret to an existing vault",
		RunE:  AddSecret,
	}

	vaultCreateCmd = &cobra.Command{
		Use:   "create <vaultname> <folder/file>",
		Short: "Create a new vault with secrets",
		RunE:  CreateVault,
	}

	vaultEditCmd = &cobra.Command{
		Use:   "edit <vaultname/secret>",
		Short: "Edit an existing secret",
		RunE:  EditSecret,
	}

	vaultDeleteCmd = &cobra.Command{
		Use:   "delete <vaultname>",
		Short: "Delete a vault",
		RunE:  DeleteVault,
	}

	vaultDeleteSecretCmd = &cobra.Command{
		Use:   "delete-secret <vaultname/secret>",
		Short: "Delete a secret",
		RunE:  DeleteSecret,
	}

	vaultListCmd = &cobra.Command{
		Use:   "list",
		Short: "list all vaults",
		RunE:  ListVaults,
	}

	vaultPermissionsCmd = &cobra.Command{
		Use:   "permissions <vaultname>",
		Short: "Add or remove permissions on a vault",
		RunE:  VaultPermissions,
	}

	vaultRenameCmd = &cobra.Command{
		Use:   "rename <vaultname> <new vaultname>",
		Short: "Rename a vault",
		RunE:  RenameVault,
	}
	vaultRenameSecretCmd = &cobra.Command{
		Use:   "rename-secret <vaultname/secretname> <new secretname>",
		Short: "Rename a secret",
		RunE:  RenameSecret,
	}
)

func init() {
	RootCmd.AddCommand(vaultCmd)

	vaultCmd.AddCommand(vaultAddSecretCmd)
	vaultCmd.AddCommand(vaultPermissionsCmd)
	vaultCmd.AddCommand(vaultDeleteCmd)
	vaultCmd.AddCommand(vaultDeleteSecretCmd)
	vaultCmd.AddCommand(vaultListCmd)
	vaultCmd.AddCommand(vaultCreateCmd)
	vaultCmd.AddCommand(vaultEditCmd)
	vaultCmd.AddCommand(vaultRenameCmd)
	vaultCmd.AddCommand(vaultRenameSecretCmd)

	vaultPermissionsCmd.Flags().StringVarP(&flagAddGroup, "add-group", "", "", "Add a group permission to the vault")
	vaultPermissionsCmd.Flags().StringVarP(&flagRemoveGroup, "remove-group", "", "", "Remove a group permission from the vault")
}

func AddSecret(cmd *cobra.Command, args []string) error {
	if len(args) != 2 {
		return cmd.Help()
	}

	vault, err := DefaultApiClient.GetVault(args[0])
	if err != nil {
		return err
	}

	err = collectSecrets(args[1], vault, false)
	if err != nil {
		return err
	}

	err = DefaultApiClient.SaveVault(*vault, true)
	if err != nil {
		return err
	}

	cmd.Printf("New secrets has been added to vault %s\n", args[0])
	return nil
}

func RenameSecret(cmd *cobra.Command, args []string) error {
	if len(args) != 2 {
		return cmd.Help()
	}

	split := strings.Split(args[0], "/")
	if len(split) != 2 {
		return ErrNotValidSecretArgument
	}

	newSecretName := args[1]
	vaultName, secretName := split[0], split[1]
	vault, err := DefaultApiClient.GetVault(vaultName)
	if err != nil {
		return err
	}

	_, err = vault.Secrets.GetSecret(secretName)
	if err != nil {
		return err
	}

	_, ok := vault.Secrets[newSecretName]
	if ok {
		return errors.Errorf("Secret %s already exists\n", newSecretName)
	}

	vault.Secrets[newSecretName] = vault.Secrets[secretName]
	vault.Secrets.RemoveSecret(secretName)

	err = DefaultApiClient.SaveVault(*vault, true)
	if err != nil {
		return err
	}

	cmd.Printf("Secret %s has been renamed to %s\n", secretName, newSecretName)
	return nil
}

func RenameVault(cmd *cobra.Command, args []string) error {
	if len(args) != 2 {
		return cmd.Help()
	}

	vault, err := DefaultApiClient.GetVault(args[1])
	if vault != nil {
		return errors.Errorf("Can't rename vault. %s already exists", args[1])
	}

	vault, err = DefaultApiClient.GetVault(args[0])
	if err != nil {
		return err
	}

	vault.Name = args[1]

	err = DefaultApiClient.SaveVault(*vault, false)
	if err != nil {
		return err
	}

	err = DefaultApiClient.DeleteVault(args[0])
	if err != nil {
		return err
	}

	fmt.Printf("%s has been renamed to %s\n", args[0], args[1])
	return nil
}

func CreateVault(cmd *cobra.Command, args []string) error {
	if len(args) != 2 {
		return cmd.Help()
	}

	v, _ := DefaultApiClient.GetVault(args[0])
	if v != nil {
		return errors.Errorf("vault %s already exists", args[0])
	}

	vault := client.NewAuroraSecretVault(args[0])

	err := collectSecrets(args[1], vault, true)
	if err != nil {
		return err
	}

	if flagRemoveGroup != "" {
		err := vault.Permissions.DeleteGroup(flagRemoveGroup)
		if err != nil {
			return err
		}
	}

	if flagAddGroup != "" {
		err := vault.Permissions.AddGroup(flagAddGroup)
		if err != nil {
			return err
		}
	}

	err = DefaultApiClient.SaveVault(*vault, false)
	if err != nil {
		return err
	}

	fmt.Println("Vault", args[0], "created")
	return nil
}

func EditSecret(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return cmd.Help()
	}

	split := strings.Split(args[0], "/")
	if len(split) != 2 {
		return ErrNotValidSecretArgument
	}

	vaultName, secretName := split[0], split[1]
	vault, err := DefaultApiClient.GetVault(vaultName)
	if err != nil {
		return err
	}

	contentToEdit, err := vault.Secrets.GetSecret(secretName)
	if err != nil {
		return err
	}

	secretEditor := editor.NewEditor(func(modifiedContent string) ([]string, error) {
		vault.Secrets.AddSecret(secretName, modifiedContent)

		err := DefaultApiClient.SaveVault(*vault, true)
		if err != nil {
			return []string{err.Error()}, nil
		}

		return nil, nil
	})

	err = secretEditor.Edit(contentToEdit, args[0], false)
	if err != nil {
		return err
	}

	cmd.Printf("Secret %s in vault %s edited\n", secretName, vaultName)
	return nil
}

func DeleteSecret(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return cmd.Help()
	}

	split := strings.Split(args[0], "/")
	if len(split) != 2 {
		return ErrNotValidSecretArgument
	}

	vaultName, secret := split[0], split[1]
	vault, err := DefaultApiClient.GetVault(vaultName)
	if err != nil {
		return err
	}

	message := fmt.Sprintf("Do you want to delete secret %s?", args[0])
	shouldDelete := prompt.Confirm(message)
	if !shouldDelete {
		return nil
	}

	vault.Secrets.RemoveSecret(secret)

	err = DefaultApiClient.SaveVault(*vault, true)
	if err != nil {
		return err
	}

	cmd.Printf("Secret %s deleted\n", args[0])
	return nil
}

func DeleteVault(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return cmd.Help()
	}

	err := DefaultApiClient.DeleteVault(args[0])
	if err != nil {
		return err
	}

	message := fmt.Sprintf("Do you want to delete vault %s?", args[0])
	shouldDelete := prompt.Confirm(message)
	if !shouldDelete {
		return nil
	}

	cmd.Printf("Vault %s deleted\n", args[0])
	return nil
}

func ListVaults(cmd *cobra.Command, args []string) error {
	if len(args) > 1 {
		return cmd.Help()
	}

	vaults, err := DefaultApiClient.GetVaults()
	if err != nil {
		return err
	}

	header, rows := getVaultTable(vaults)
	if len(rows) == 0 {
		return errors.New("No vaults available")
	}
	DefaultTablePrinter(header, rows, cmd.OutOrStdout())

	return nil
}

func VaultPermissions(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return cmd.Help()
	}

	if flagRemoveGroup == "" && flagAddGroup == "" {
		return errors.New("Please specify --add-group <group> or/and --remove-group <group>")
	}

	vault, err := DefaultApiClient.GetVault(args[0])
	if err != nil {
		return err
	}

	if flagRemoveGroup != "" {
		err = vault.Permissions.DeleteGroup(flagRemoveGroup)
		if err != nil {
			return err
		}
	}

	if flagAddGroup != "" {
		err = vault.Permissions.AddGroup(flagAddGroup)
		if err != nil {
			return err
		}
	}

	err = DefaultApiClient.SaveVault(*vault, true)
	if err != nil {
		return err
	}

	cmd.Printf("Vault %s saved\n", args[0])
	return nil
}

func collectSecrets(filePath string, vault *client.AuroraSecretVault, includePermissions bool) error {
	root, err := os.Stat(filePath)
	if err != nil {
		return err
	}

	var files []os.FileInfo
	if root.IsDir() {
		files, err = ioutil.ReadDir(filePath)
		if err != nil {
			return err
		}
	} else {
		files = append(files, root)
	}

	for _, f := range files {
		if f.IsDir() {
			continue
		}

		currentFilePath := filePath
		if root.IsDir() {
			currentFilePath = path.Join(filePath, f.Name())
		}

		if strings.Contains(f.Name(), "permission") && includePermissions {
			groups, err := readPermissionFile(currentFilePath)
			if err != nil {
				return err
			}
			vault.Permissions["groups"] = groups
		} else {
			secret, err := readSecretFile(currentFilePath)
			if err != nil {
				return err
			}
			vault.Secrets.AddSecret(f.Name(), secret)
		}
	}

	return nil
}

func readSecretFile(fileName string) (string, error) {
	data, err := ioutil.ReadFile(fileName)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

func readPermissionFile(path string) ([]string, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	permissions := struct {
		Groups []string `json:"groups"`
	}{}

	err = json.Unmarshal(data, &permissions)
	if err != nil {
		return nil, err
	}
	if permissions.Groups == nil {
		return nil, ErrEmptyGroups
	}

	return permissions.Groups, nil
}

func getVaultTable(vaults []*client.AuroraVaultInfo) (string, []string) {

	sort.Slice(vaults, func(i, j int) bool {
		return strings.Compare(vaults[i].Name, vaults[j].Name) < 1
	})

	var rows []string
	for _, vault := range vaults {
		name := vault.Name
		permissions := vault.Permissions.GetGroups()

		for _, secret := range vault.Secrets {
			line := fmt.Sprintf("%s\t%s\t%s\t%v", name, permissions, secret, vault.Admin)
			rows = append(rows, line)
			name = " "
		}
	}

	header := "VAULT\tPERMISSIONS\tSECRET\tACCESS"
	return header, rows
}
