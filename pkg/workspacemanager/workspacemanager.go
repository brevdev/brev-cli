package workspacemanager

import (
	"errors"
	"fmt"
	"strings"

	"github.com/brevdev/brev-cli/pkg/collections" //nolint:typecheck // uses generic code
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/store"
)

type WorkspaceManager struct{}

func NewWorkspaceManager() *WorkspaceManager {
	// NOT YET IMPLEMENTED
	return &WorkspaceManager{}
}

func (w WorkspaceManager) Start(workspaceID string) error {
	// somewhere we need to check if they have docker installed and all the tools needed before running this
	fmt.Println(workspaceID)
	// get a list of workspaces that have been run previously
	// if the workspaceID matches any of them, then run the (first, last, most recently run?) version
	previouslyRunWorkspaces := fetchPreviouslyRunWorkspaces()
	matchingWorkspaces := collections.Filter(collections.P2(nameMatches, workspaceID), previouslyRunWorkspaces)
	// it should either find a pre-existing container with this name
	if len(matchingWorkspaces) > 0 {
		// and start it if it is currently stopped
		workspaceToStart := collections.First(collections.SortBy(workspacePriorityFunc, matchingWorkspaces))
		return startLocalWorkspace(*workspaceToStart)
	}
	// or if that is not the case, then start a new one via the following steps:
	// given a workspace id, resolve it to get the full data object for the workspace
	workspace := resolveWorkspaceID(workspaceID)
	// turn the workspace object into setup parameters
	setupParams := workspaceToSetupParams(workspace)
	// we need the workspace base container
	baseImage := fetchBaseImage()
	// we need to retrieve the kubernetes token
	// we need to mix in the kubernetes token
	kubernetesServiceTokenDir := fetchKubernetesServiceTokenDir()
	mountToken := collections.P2(mixInKubernetesToken, kubernetesServiceTokenDir)
	// we need to mount the setup parameters to it, we need to map in the kubernetes service token
	mountParams := collections.P2(mountSetupParams, setupParams)
	// we need to mount  a volume for the workspace/ folder

	mountWSVolume := collections.P2(mountWorkspaceVolume, generateWorkspaceVolume(workspaceID))
	// we need to make sure we map in everything in meta
	mapMeta := collections.P2(mapValuesThroughMeta, generateMetaValues(workspace))
	// we need to expose the ports correctly so it can speak with the outside world (and the other way as well)
	// think about how this can be running more than one workspace (clever port-mapping?)
	portMapping := generatePortMapping(workspace, previouslyRunWorkspaces)
	mapPorts := collections.P2(exposePorts, portMapping)
	// as long as each takes what the other provides, this function chain will work (evaluated right-to-left)
	preparedImage := collections.C5(mapPorts, mapMeta, mountWSVolume, mountParams, mountToken)(baseImage) // right-to-left application
	// we need to execute a docker run with the workspace volume pointed to the correct space
	// (this might need to be run in privileged mode)
	return dockerExecute(workspaceID, preparedImage)

	// ASIDES BUT IMPORTANT
	// need to build a secret manager config manager
	// right now there is a config file which gets put into the workspace
	// inside the workspace vault agent updates when the file changes
	// the way we change that file with a kubernetes thing -- but we will need to build that magic for this

	// maybe later we need to configure a health check to reboot if it doesn't path (but for now probably ok)

	// see simulate-workspace in the Makefile for some inspiration
	// e2e test folder setup_test.go to see docker exec being run
}

func (w WorkspaceManager) Stop(workspaceID string) error {
	runningWorkspaces := fetchRunningWorkspaces()
	matchingWorkspaces := collections.Filter(collections.P2(nameMatches, workspaceID), runningWorkspaces)
	if len(matchingWorkspaces) > 0 {
		workspaceToStop := collections.First(collections.SortBy(workspacePriorityFunc, matchingWorkspaces))
		return stopLocalWorkspace(*workspaceToStop)
	}
	return errors.New(strings.Join([]string{"No entity.WorkspaceID by name", workspaceID, "currently running."}, " "))
}

func (w WorkspaceManager) Reset(workspaceID string) error {
	err := w.Stop(workspaceID)
	if err != nil {
		return err // do we want this to return if it can't find one to stop,
		// or should it just start one if it can find one to start?
	}
	return w.Start(workspaceID)
}

type DockerContainer struct {
	CommandArgs     []string
	Name            string
	VolumeMap       map[string]string
	ShellPreference string `default:"zsh"`
	PortMap         map[string]string
	BaseImage       string `default:"brevdev/ubuntu-proxy:0.3.7"`
	Privileged      `default:true`
}

type LocalWorkspace struct {
	name string
}

func workspacePriorityFunc(left LocalWorkspace, right LocalWorkspace) bool {
	// NOT YET IMPLEMENTED
	// this should tell us what counts are 'more' or 'less'. example options are
	// doing the most recently run, or the most recently created, or the most recently added...
	return false
}

func fetchPreviouslyRunWorkspaces() []LocalWorkspace {
	// NOT YET IMPLEMENTED
	return []LocalWorkspace{}
}

func fetchRunningWorkspaces() []LocalWorkspace {
	// NOT YET IMPLEMENTED
	// take output of docker ps and use it somehow?
	return []LocalWorkspace{}
}

func nameMatches(name string, workspace LocalWorkspace) bool {
	// NOT YET IMPLEMENTED
	return false
}

func startLocalWorkspace(workspace LocalWorkspace) error {
	// NOT YET IMPLEMENTED
	return errors.New("Start entity.Workspace Not Yet Implemented")
}

func stopLocalWorkspace(workspace LocalWorkspace) error {
	// NOT YET IMPLEMENTED
	return errors.New("Stop entity.Workspace Not Yet Implemented")
}

func resolveWorkspaceID(workspaceID string) entity.Workspace {
	// NOT YET IMPLEMENTED
	return entity.Workspace{}
}

func workspaceToSetupParams(workspace entity.Workspace) store.SetupParamsV0 {
	// NOT YET IMPLEMENTED
	return store.SetupParamsV0{}
}

func fetchBaseImage() DockerContainer {
	// NOT YET IMPLEMENTED
	return DockerContainer{}
}

func fetchKubernetesServiceTokenDir() string {
	// NOT YET IMPLEMENTED
	return "/var/run/secrets/kubernetes.io/serviceaccount/" // not yet implemented
}

func mixInKubernetesToken(tokenDirectory string, container DockerContainer) DockerContainer {
	// NOT YET IMPLEMENTED
	return DockerContainer{}
}

func mountSetupParams(setupParams store.SetupParamsV0, container DockerContainer) DockerContainer {
	// NOT YET IMPLEMENTED
	return DockerContainer{}
}

func generateWorkspaceVolume(workspaceID string) string {
	// NOT YET IMPLEMENTED
	return workspaceID
}

func mountWorkspaceVolume(path string, container DockerContainer) DockerContainer {
	// NOT YET IMPLEMENTED
	return DockerContainer{}
}

func generateMetaValues(workspace entity.Workspace) map[string]string {
	// NOT YET IMPLEMENTED
	return map[string]string{}
}

func mapValuesThroughMeta(valuesMap map[string]string, container DockerContainer) DockerContainer {
	// NOT YET IMPLEMENTED
	return container
}

func generatePortMapping(workspace entity.Workspace, previouslyRunWorkspaces []LocalWorkspace) map[string]string {
	// NOT YET IMPLEMENTED
	return map[string]string{}
}

func exposePorts(portMap map[string]string, container DockerContainer) DockerContainer {
	// NOT YET IMPLEMENTED
	return DockerContainer{}
}

func portMapToString(portMap map[string]string) string {
	return strings.Join(collections.Fmap(func (key string) string {
		val := portMap[key]
		return strings.Join([]string{"-p", strings.Join([]string{key, val}, ":")}, " ")
	}, collections.Keys(volumeMap)), " ")
}

func volumeMapToString(volumeMap map[string]string) string {
	return strings.Join(collections.Fmap(func (key string) string {
		val := volumeMap[key]
		return strings.Join([]string{"-v", strings.Join([]string{key, val}, ":")}, " ")
	}, collections.Keys(volumeMap)), " ")
}

func containerToString(container DockerContainer) string {
	privilegeString := ""
	if container.Privileged {
		privilegeString = "--privileged=true"
	}
	strings.Join([]string{
		"docker run", "-d", // detached
		privilegeString, // name
		"--name", workspaceID,
		 "-it", // i attaches to stdin, t to terminal
		portMapToString(container.PortMap), 
		volumeMapToString(container.VolumeMap), 
		container.ShellPreference}, " ")
}

func dockerExecute(workspaceID string, container DockerContainer) error {
	// NOT YET IMPLEMENTED
	command := containerToString(workspaceID, container)
	fmt.Println("final command is ")
	fmt.Println(command)
	return command
	// return errors.New("Docker Execute Not Yet Implemented")
}
