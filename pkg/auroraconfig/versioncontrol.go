package auroraconfig

import (
	"fmt"
	"github.com/howeyc/gopass"
	"github.com/pkg/errors"
	"github.com/skatteetaten/ao/pkg/cmdoptions"
	"github.com/skatteetaten/ao/pkg/configuration"
	"github.com/skatteetaten/ao/pkg/serverapi_v2"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	"gopkg.in/src-d/go-git.v4/plumbing/transport/http"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func Clone(affiliation string, username string, outputPath string, url string) error {

	fmt.Printf("Cloning AuroraConfig for affiliation %s\n", affiliation)
	fmt.Printf("%s\n\n", url)

	basicAuth := authenticateUser(username)

	_, err := git.PlainClone(outputPath, false, &git.CloneOptions{
		URL:      url,
		Progress: os.Stdout,
		Auth:     basicAuth,
	})

	if err != nil {
		return errors.Wrap(err, "Clone failed")
	}

	currentDir, _ := os.Getwd()
	os.Chdir(outputPath)

	cmd := exec.Command("git", "branch", "--set-upstream-to=origin/master", "master")
	if err = cmd.Run(); err != nil {
		return err
	}

	os.Chdir(currentDir)

	return nil
}

func Commit(username string, persistentOptions *cmdoptions.CommonCommandOptions) error {

	wd, _ := os.Getwd()

	repository, err := git.PlainOpen(wd)
	if err != nil {
		return err
	}

	basicAuth := authenticateUser(username)
	// returns error if repository is already up to date
	fetchOrigin(repository, basicAuth)

	if err = checkForNewCommits(); err != nil {
		return err
	}

	if err = checkRepoForChanges(repository); err != nil {
		return err
	}

	if err = handleAuroraConfigCommit(repository, persistentOptions); err != nil {
		return err
	}

	if err = exec.Command("git", "checkout", ".").Run(); err != nil {
		return err
	}

	wt, _ := repository.Worktree()
	wt.Pull(&git.PullOptions{
		Auth: basicAuth,
	})

	return nil
}

func authenticateUser(username string) *http.BasicAuth {
	fmt.Print("Enter password: ")
	password, _ := gopass.GetPasswdMasked()

	fmt.Println()

	return http.NewBasicAuth(username, string(password))
}

func handleAuroraConfigCommit(repository *git.Repository, persistentOptions *cmdoptions.CommonCommandOptions) error {

	var config configuration.ConfigurationClass
	config.Init(persistentOptions)

	ac, err := GetAuroraConfig(&config)

	if err != nil {
		return errors.Wrap(err, "Failed getting AuroraConfig")
	}

	if err = addFilesToAuroraConfig(&ac); err != nil {
		return errors.Wrap(err, "Failed adding files to AuroraConfig")
	}

	head, _ := repository.Head()
	removeFilesFromAuroraConfig(repository, &ac, head.Hash())

	if err = PutAuroraConfig(ac, &config); err != nil {
		return errors.Wrap(err, "Failed committing AuroraConfig")
	}

	return nil
}

func checkRepoForChanges(repository *git.Repository) error {
	wt, _ := repository.Worktree()
	status, _ := wt.Status()
	if status.IsClean() {
		return errors.New("Nothing to commit")
	}

	return nil
}

func fetchOrigin(repository *git.Repository, auth *http.BasicAuth) error {

	return repository.Fetch(&git.FetchOptions{
		Auth:       auth,
		RemoteName: "origin",
	})
}

func checkForNewCommits() error {

	if err := compareGitLog("origin/master..HEAD"); err != nil {
		return errors.New(`You have committed local changes.
Please revert them with: git reset HEAD^`)
	}

	if err := compareGitLog("HEAD..origin/master"); err != nil {
		return errors.New(`Please update to latest configuration with: git pull`)
	}

	return nil
}

func compareGitLog(compare string) error {

	cmd := exec.Command("git", "log", compare, "--oneline")
	out, err := cmd.Output()

	if err != nil {
		return err
	}

	output := string(out)

	if len(output) > 0 {
		return errors.New("new commits")
	}

	return nil
}

func addFilesToAuroraConfig(ac *serverapi_v2.AuroraConfig) error {
	wd, _ := os.Getwd()

	return filepath.Walk(wd, func(path string, info os.FileInfo, err error) error {

		filename := strings.TrimPrefix(path, wd+"/")

		if strings.Contains(filename, ".git") || strings.Contains(filename, ".secret") || info.IsDir() {
			return nil
		}

		file, err := ioutil.ReadFile(wd + "/" + filename)

		if err != nil {
			return errors.Wrap(err, "Could not read file "+filename)
		}

		ac.Files[filename] = file

		return nil
	})
}

func removeFilesFromAuroraConfig(repository *git.Repository, ac *serverapi_v2.AuroraConfig, hash plumbing.Hash) error {

	wt, _ := repository.Worktree()
	status, _ := wt.Status()
	commit, _ := repository.CommitObject(hash)

	headFiles, _ := commit.Files()
	return headFiles.ForEach(func(file *object.File) error {
		code := status.File(file.Name).Worktree

		if code == git.Deleted {
			delete(ac.Files, file.Name)
		}

		return nil
	})
}