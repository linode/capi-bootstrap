package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/google/go-github/v63/github"
	v1 "k8s.io/client-go/tools/clientcmd/api/v1"
	"k8s.io/klog/v2"
	k8syaml "sigs.k8s.io/yaml"

	"capi-bootstrap/types"
	"capi-bootstrap/utils"
	capiYaml "capi-bootstrap/yaml"
)

const (
	defaultRepoName   = "capi-bootstrap"
	defaultOrgName    = "linode"
	defaultBranchName = "main"
)

func NewBackend() *Backend {
	return &Backend{
		Name:       "github",
		Repo:       os.Getenv("GITHUB_REPO"),
		Org:        os.Getenv("GITHUB_ORG"),
		Token:      os.Getenv("GITHUB_TOKEN"),
		clusters:   make(map[string]*v1.Config),
		branchName: os.Getenv("GITHUB_BRANCH"),
	}
}

type CreateBranchOptions struct {
	Ref string `json:"ref"`
	Sha string `json:"sha"`
}

type Backend struct {
	Name  string
	Org   string
	Repo  string
	Token string

	client     *github.Client
	user       *github.User
	branch     *github.Branch
	branchName string
	clusters   map[string]*v1.Config
}

func (b *Backend) PreCmd(ctx context.Context, clusterName string) error {
	if b.Token == "" {
		return errors.New("GITHUB_TOKEN is required")
	}

	klog.V(4).Infof("[github backend] opts: %+v", b)
	if b.Org == "" {
		klog.Infof("GITHUB_ORG is not set, defaulting to %s", defaultOrgName)
		b.Org = defaultOrgName
	}

	if b.Repo == "" {
		klog.Infof("GITHUB_REPO is not set, defaulting to %s", defaultRepoName)
		b.Repo = defaultRepoName
	}

	if b.branchName == "" {
		klog.Infof("GITHUB_BRANCH is not set, defaulted to %s", defaultBranchName)
		b.branchName = defaultBranchName
	}

	klog.V(4).Infof("[github backend] trying to validate existing state repo %s/%s for cluster %s", b.Org, b.Repo, clusterName)

	client, user, err := authenticate(ctx, b.Token, b.Org, b.Repo)
	if err != nil {
		return fmt.Errorf("[github backend] failed to authenticate to repo %s/%s: %v", b.Org, b.Repo, err)
	}
	b.client = client
	b.user = user

	branch, httpResp, err := b.client.Repositories.GetBranch(context.Background(), b.Org, b.Repo, b.branchName, 2)
	if err != nil && httpResp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("[github backend] unexpected error when checking for branch %s: %v", b.branchName, err)
	} else {
		b.branch = branch
	}

	klog.Infof("[github backend] successfully authenticated to state repo %s/%s using branch %s", b.Org, b.Repo, b.branchName)

	return nil
}

func (b *Backend) Read(ctx context.Context, clusterName string) (*v1.Config, error) {
	client := b.client
	klog.V(4).Infof("[github backend] trying to read state file %s/%s from branch %s in repo %s/%s", clusterName, "kubeconfig.yaml", b.branchName, b.Org, b.Org)

	if b.branch != nil {
		file, _, _, err := client.Repositories.GetContents(ctx, b.Org, b.Repo, path.Join("clusters", clusterName, "kubeconfig.yaml"), &github.RepositoryContentGetOptions{
			Ref: b.branchName,
		})
		if err != nil {
			return nil, err
		}

		rawStateFile, err := file.GetContent()
		if err != nil {
			return nil, err
		}

		kubeconfig := strings.NewReader(rawStateFile)

		rawKubeconfig, err := io.ReadAll(kubeconfig)
		if err != nil {
			return nil, err
		}

		js, err := k8syaml.YAMLToJSON(rawKubeconfig)
		if err != nil {
			return nil, err
		}

		var config v1.Config
		if err := json.Unmarshal(js, &config); err != nil {
			return nil, err
		}

		return &config, nil
	}

	return nil, fmt.Errorf("[github backend] branch %s may not exist in repo %s/%s", b.branchName, b.Org, b.Repo)
}

func (b *Backend) Delete(ctx context.Context, clusterName string) error {
	klog.V(4).Infof("[github backend] trying to delete state files in remote repo: %s", clusterName)

	branch, _, err := b.client.Repositories.GetBranch(ctx, b.Org, b.Repo, b.branchName, 2)
	if err != nil {
		return err
	}
	b.branch = branch
	tree, _, err := b.client.Git.GetTree(ctx, b.Org, b.Repo, b.branch.Commit.GetSHA(), true)
	if err != nil {
		return err
	}

	for _, entry := range tree.Entries {
		if strings.HasPrefix(entry.GetPath(), path.Join("clusters", clusterName)) {
			// set content and sha to nil, which tells git you are deleting this file
			entry.SHA = nil
			entry.Content = nil
			continue
		}
	}

	nt, _, err := b.client.Git.CreateTree(ctx, b.Org, b.Repo, b.branch.Commit.GetSHA(), tree.Entries)
	if err != nil {
		return err
	}

	// If you use the `/repos/{owner}/{repo}/git/trees` endpoint to add, delete, or modify the file contents in a tree,
	// you will need to commit the tree and then update a branch to point to the commit.
	// For more information see "Create a commit" and "Update a reference."

	commit := &github.Commit{
		SHA:  nt.SHA,
		Tree: nt,
		Author: &github.CommitAuthor{
			Date: &github.Timestamp{
				Time: time.Now(),
			},
			Name:  b.user.Name,
			Email: b.user.Email,
			Login: b.user.Login,
		},
		Parents: b.branch.GetCommit().Parents,
		Message: PointerTo(fmt.Sprintf("deleting state files for cluster %s", clusterName)),
		//Verification: nil, // TODO sign commits
	}

	newCommit, _, err := b.client.Git.CreateCommit(ctx, b.Org, b.Repo, commit, &github.CreateCommitOptions{})
	if err != nil {
		return err
	}

	ref := &github.Reference{
		Ref: PointerTo(path.Join("heads", b.branch.GetName())),
		Object: &github.GitObject{
			Type: PointerTo("commit"),
			SHA:  newCommit.SHA,
		},
	}

	if _, _, err = b.client.Git.UpdateRef(ctx, b.Org, b.Repo, ref, true); err != nil {
		return err
	}

	klog.Infof("[github backend] deleted all state files from branch %s in github repo %s/%s ", clusterName, b.Org, b.Repo)

	return nil
}

func (b *Backend) ListClusters(ctx context.Context) ([]types.ClusterInfo, error) {
	_, clusterConfigs, _, err := b.client.Repositories.GetContents(ctx, b.Org, b.Repo, "clusters", &github.RepositoryContentGetOptions{
		Ref: b.branchName,
	})
	if err != nil {
		return nil, err
	}

	for _, cluster := range clusterConfigs {
		if cluster.GetType() != "dir" {
			klog.Warningf("expected remote content to be a directory, but was a %s instead", cluster.GetType())
			continue
		}
		rawKC, _, _, err := b.client.Repositories.GetContents(ctx, b.Org, b.Repo, path.Join("clusters", *cluster.Name, "kubeconfig.yaml"), &github.RepositoryContentGetOptions{
			Ref: b.branchName,
		})
		if err != nil {
			return nil, err
		}

		kc, err := rawKC.GetContent()
		if err != nil {
			return nil, err
		}

		kubeconfig := strings.NewReader(kc)

		rawKubeconfig, err := io.ReadAll(kubeconfig)
		if err != nil {
			return nil, err
		}

		config := &v1.Config{}
		err = k8syaml.Unmarshal(rawKubeconfig, config)
		if err != nil {
			return nil, err
		}

		b.clusters[*cluster.Name] = config
	}

	clusters := make([]types.ClusterInfo, len(b.clusters))
	for name, conf := range b.clusters {
		kubeconfig, err := capiYaml.Marshal(conf)
		if err != nil {
			return nil, err
		}
		list, err := utils.BuildNodeInfoList(ctx, kubeconfig)
		if err != nil {
			return nil, err
		}
		clusters = append(clusters, types.ClusterInfo{
			Name:  name,
			Nodes: list,
		})
	}

	return clusters, nil
}

func (b *Backend) WriteConfig(ctx context.Context, clusterName string, config *v1.Config) error {
	js, err := json.Marshal(config)
	if err != nil {
		return err
	}

	y, err := k8syaml.JSONToYAML(js)
	if err != nil {
		return err
	}
	filePath := path.Join("clusters", clusterName, "kubeconfig.yaml")
	_, err = b.uploadFile(ctx, string(y), filePath, clusterName)
	if err != nil {
		return fmt.Errorf("failed to write cluster %s config: %v", clusterName, err)
	}
	return nil
}

func (b *Backend) WriteFiles(ctx context.Context, clusterName string, cloudInitConfig *capiYaml.Config) ([]string, error) {
	downloadCmds := make([]string, len(cloudInitConfig.WriteFiles))
	newFiles := make([]capiYaml.InitFile, len(cloudInitConfig.WriteFiles))
	for i, file := range cloudInitConfig.WriteFiles {
		newCmd, newFile, err := b.writeFile(ctx, clusterName, file)
		if err != nil {
			return nil, err
		}
		downloadCmds[i] = newCmd
		newFiles[i] = *newFile
	}
	cloudInitConfig.WriteFiles = newFiles
	return downloadCmds, nil
}

func (b *Backend) writeFile(ctx context.Context, clusterName string, cloudInitFile capiYaml.InitFile) (string, *capiYaml.InitFile, error) {
	if cloudInitFile.Content == "" {
		return "", nil, errors.New("cloudInitFile content is empty")
	}
	remotePath := path.Join("clusters", clusterName, "files", cloudInitFile.Path)

	downloadURL, err := b.uploadFile(ctx, cloudInitFile.Content, remotePath, clusterName)
	if err != nil {
		return "", nil, fmt.Errorf("couldn't upload object: %v", err)
	}

	cloudInitFile.Content = ""
	klog.V(4).Infof("[github backend] updated existing state file %s for cluster %s in remote repo %s/%s", remotePath, clusterName, b.Org, b.Repo)

	downloadCmd := fmt.Sprintf("curl -sL -H 'Accept: application/vnd.github.raw+json' -H 'Authorization: Bearer %s' -H 'X-GitHub-Api-Version: 2022-11-28' '%s' | xargs -0 cloud-init query -f > %s", b.Token, downloadURL, cloudInitFile.Path)
	return downloadCmd, &cloudInitFile, nil
}

func (b *Backend) uploadFile(ctx context.Context, fileContent string, remotePath string, clusterName string) (string, error) {
	// need config for the SHA
	config, _, httpResp, err := b.client.Repositories.GetContents(ctx, b.Org, b.Repo, remotePath, &github.RepositoryContentGetOptions{
		Ref: b.branchName,
	})
	if err != nil {
		switch httpResp.StatusCode {
		case http.StatusNotFound, http.StatusOK, http.StatusFound, http.StatusNotModified:
			// expected
		case http.StatusForbidden:
			return "", fmt.Errorf("failed to upload state file %s due to permissions error: %w", remotePath, err)
		default:
			return "", err
		}
	}

	var contentResp *github.RepositoryContentResponse
	switch httpResp.StatusCode {
	// https://docs.github.com/en/rest/repos/contents?apiVersion=2022-11-28#get-repository-content--status-codes
	case http.StatusNotFound:
		// create
		contentResp, _, err = b.client.Repositories.CreateFile(ctx, b.Org, b.Repo, remotePath, &github.RepositoryContentFileOptions{
			Content: []byte(fileContent),
			Branch:  PointerTo(b.branchName),
			Committer: &github.CommitAuthor{
				Date:  &github.Timestamp{Time: time.Now()},
				Name:  b.user.Name,
				Email: b.user.Email,
				Login: b.user.Login,
			},
			Message: PointerTo(fmt.Sprintf("creating cluster %s state file", clusterName)),
		})
		if err != nil {
			return "", err
		}

	default:
		// update
		contentResp, _, err = b.client.Repositories.UpdateFile(ctx, b.Org, b.Repo, remotePath, &github.RepositoryContentFileOptions{
			Content: []byte(fileContent),
			Branch:  PointerTo(b.branchName),
			Committer: &github.CommitAuthor{
				Date:  &github.Timestamp{Time: time.Now()},
				Name:  b.user.Name,
				Email: b.user.Email,
				Login: b.user.Login,
			},
			SHA:     config.SHA,
			Message: PointerTo(fmt.Sprintf("updating cluster %s state file", clusterName)),
		})
		if err != nil {
			return "", err
		}
	}
	return contentResp.Content.GetDownloadURL(), nil
}

func authenticate(ctx context.Context, token, org, repo string) (*github.Client, *github.User, error) {
	client := github.NewClient(nil).WithAuthToken(token)

	// fetch the repo to allow easy access to owner info (name, email, login, etc.)
	user, _, err := client.Users.Get(ctx, org)
	if err != nil {
		return nil, nil, err
	}

	// validate we have access to the repository
	_, r, err := client.Repositories.Get(ctx, org, repo)
	if err != nil || r.StatusCode != http.StatusOK {
		return nil, nil, err
	}

	return client, user, nil
}

func PointerTo[T any](s T) *T {
	return &s
}
