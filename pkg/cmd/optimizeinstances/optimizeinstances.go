package optimizeinstances

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/mail"
	"strings"
	"sync"
	"time"

	"github.com/alecthomas/units"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer"
	"github.com/aws/aws-sdk-go-v2/service/dlm"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/pricing"
	"github.com/aws/aws-sdk-go-v2/service/servicequotas"

	"github.com/brevdev/brev-cli/pkg/collections" //nolint:typecheck
	"github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/ids"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/hashicorp/go-multierror"
	"github.com/spf13/cobra"
	"go.opentelemetry.io/otel"
)

var (
	short   = "Apply Brev cost optimizations to your instances. Enter the IDs of the instances you want to optimized."
	long    = "Apply Brev cost optimizations to your instances. Enter the IDs of the instances you want to optimized. We will apply everything from autostop to letting you scale between hardware configs"
	example = "brev optimize-instances i-1234567890abcdef0 i-1234567890abcdef1"
)

type optimizeInstancesStore interface{}

type AWSClient struct {
	location           string
	ec2Client          *ec2.Client
	quotaClient        *servicequotas.Client
	pricingClient      *pricing.Client
	costExplorerClient *costexplorer.Client
	dlmClient          *dlm.Client
	iamClient          *iam.Client
}
type LifecycleStatus string

type Status struct {
	LifecycleStatus LifecycleStatus
}

type Instance struct {
	ID                  ids.CloudProviderInstanceID
	Hostname            string
	ImageID             string
	InstanceType        string
	DiskSize            units.Base2Bytes
	PubKeyFingerprint   string
	Status              Status
	MetaEndpointEnabled bool
	MetaTagsEnabled     bool
	VPCID               string
	SubnetID            string
	Spot                bool
	Name                string
}

type KeyPair struct {
	KeyFingerprint string

	// The name of the key pair.
	KeyName string

	// The ID of the key pair.
	KeyPairID string
}

func NewCmdOptimizeInstances(t *terminal.Terminal, store optimizeInstancesStore) *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "optimize-instances",
		Aliases:               []string{"oi"},
		DisableFlagsInUseLine: true,
		Short:                 short,
		Long:                  long,
		Example:               example,
		RunE: func(cmd *cobra.Command, args []string) error {
			err := OptimizeInstances(t, args, store)
			if err != nil {
				return errors.WrapAndTrace(err)
			}
			return nil
		},
	}
	return cmd
}

func OptimizeInstances(t *terminal.Terminal, args []string, _ optimizeInstancesStore) error {
	userEmail, err := getUserEmail(t, "Please enter your email address: ")
	if err != nil {
		return errors.WrapAndTrace(err)
	}
	fmt.Print("This operation will modify your instances' user data. Do you want to continue? If you don't know what that is don't worry (y/n): ")
	confirmed, err := askForConfirmation()
	if err != nil {
		return errors.WrapAndTrace(err)
	}
	if !confirmed {
		fmt.Println("Aborting")
		return nil
	}
	config, err := GetLiveFileConfig(context.TODO())
	if err != nil {
		return errors.WrapAndTrace(err)
	}
	awsClient := GetAWSClient(config)

	var result error
	var wg sync.WaitGroup

	for _, arg := range args {
		wg.Add(1)

		arg := arg
		go func() {
			defer wg.Done()
			err := updateInstance(t, awsClient, arg, userEmail)
			if err != nil {
				fmt.Println(t.Red("Failed to update instance: "), arg, t.Red(err.Error()))
			} else {
				fmt.Println(t.Green("Successfully updated: "), arg)
			}
			result = multierror.Append(result, err)
		}()
	}
	wg.Wait()

	if merr, ok := result.(*multierror.Error); ok {
		if len(merr.Errors) < len(args) {
			return nil
		}
	}
	return errors.WrapAndTrace(result)
}

func getUserEmail(t *terminal.Terminal, promptText string) (string, error) {
	fmt.Print(t.Yellow(promptText))
	var userEmail string
	_, err := fmt.Scanln(&userEmail)
	if err != nil {
		return "", errors.WrapAndTrace(err)
	}
	if validEmail(userEmail) {
		return userEmail, nil
	}
	return getUserEmail(t, "Please enter a valid email address: ")
}

func validEmail(email string) bool {
	_, err := mail.ParseAddress(email)
	return err == nil
}

func findAWSClientWithCorrectRegion(ctx context.Context, instanceID string) (AWSClient, error) {
	regions := []string{"us-east-1", "us-east-2", "us-west-1", "us-west-2", "ap-east-1", "ap-south-1", "ap-northeast-3", "ap-northeast-2", "ap-southeast-1", "ap-southeast-2", "ap-northeast-1", "ca-central-1", "eu-central-1", "eu-west-1", "eu-west-2", "eu-west-3", "eu-north-1", "me-south-1", "sa-east-1", "af-south-1", "ap-southeast-3", "eu-south-1", "me-central-1", "us-gov-east-1", "us-gov-west-1"}

	config, err := GetLiveFileConfig(context.TODO())
	if err != nil {
		return AWSClient{}, errors.WrapAndTrace(err)
	}
	for _, region := range regions {
		config.Region = region
		awsClient := GetAWSClient(config)

		_, err := awsClient.GetInstance(ctx, ids.CloudProviderInstanceID(instanceID))
		if err == nil {
			return awsClient, nil
		}
	}
	return AWSClient{}, errors.WrapAndTrace(errors.New("Could not find instance in any region"))
}

func updateInstance(t *terminal.Terminal, awsClient AWSClient, instanceID string, userEmail string) error {
	awsClient, err := findAWSClientWithCorrectRegion(context.TODO(), instanceID)
	if err != nil {
		return errors.WrapAndTrace(err)
	}
	fmt.Println(t.Red("Stopping instance: "), instanceID)
	err = awsClient.StopInstance(context.Background(), ids.CloudProviderInstanceID(instanceID))
	if err != nil {
		return errors.WrapAndTrace(err)
	}
	// TODO: append the actual current data
	err = waitForEc2State(context.Background(), awsClient.ec2Client, ids.CloudProviderInstanceID(instanceID), ec2types.InstanceStateNameStopped, 300)
	if err != nil {
		return errors.WrapAndTrace(err)
	}
	userDataArgs := UserDataArgs{
		OnBootScript: `#!/bin/bash
		bash -c "$(curl -fsSL https://raw.githubusercontent.com/brevdev/brev-cli/main/bin/install-latest.sh)"
		brev postinstall ` + userEmail,
		OnInstanceScript: ``,
		OnOnceScript:     ``,
	}
	newUserData := makeUserData(userDataArgs)
	fmt.Println(t.Yellow("Updating instance: "), instanceID)
	err = awsClient.UpdateInstanceUserData(context.TODO(), ids.CloudProviderInstanceID(instanceID), newUserData)
	if err != nil {
		return errors.WrapAndTrace(err)
	}
	time.Sleep(5 * time.Second)
	err = awsClient.StartInstance(context.Background(), ids.CloudProviderInstanceID(instanceID))
	if err != nil {
		return errors.WrapAndTrace(err)
	}
	return nil
}

func GetLiveFileConfig(ctx context.Context) (aws.Config, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return aws.Config{}, errors.WrapAndTrace(err)
	}

	return cfg, nil
}

func GetAWSClient(cfg aws.Config) AWSClient {
	ec2Client := ec2.NewFromConfig(cfg)
	pricingConfig := cfg.Copy()
	pricingClient := pricing.NewFromConfig(pricingConfig)
	quotaClient := servicequotas.NewFromConfig(cfg)
	costExplorerCLient := costexplorer.NewFromConfig(cfg)
	dlmClient := dlm.NewFromConfig(cfg)
	iamClient := iam.NewFromConfig(cfg)
	return NewAWSClient(ec2Client, pricingClient, quotaClient, costExplorerCLient, dlmClient, iamClient, pricingConfig.Region)
}

func NewAWSClient(ec2Client *ec2.Client, pricingClient *pricing.Client, quotaClient *servicequotas.Client, costExplorerClient *costexplorer.Client, dlmClient *dlm.Client, iamClient *iam.Client, location string) AWSClient {
	return AWSClient{
		ec2Client:          ec2Client,
		quotaClient:        quotaClient,
		pricingClient:      pricingClient,
		costExplorerClient: costExplorerClient,
		location:           location,
		dlmClient:          dlmClient,
		iamClient:          iamClient,
	}
}

func (a AWSClient) GetInstanceUserData(ctx context.Context, instanceID ids.CloudProviderInstanceID) (string, error) {
	result, err := a.ec2Client.DescribeInstanceAttribute(ctx, &ec2.DescribeInstanceAttributeInput{
		InstanceId: aws.String(string(instanceID)),
		Attribute:  ec2types.InstanceAttributeNameUserData,
	})
	if err != nil {
		return "", errors.WrapAndTrace(err)
	}
	if result.UserData == nil || result.UserData.Value == nil {
		return "", nil
	}
	return *result.UserData.Value, nil
}

func (a AWSClient) UpdateInstanceUserData(ctx context.Context, instanceID ids.CloudProviderInstanceID, userData string) error {
	_, err := a.ec2Client.ModifyInstanceAttribute(ctx, &ec2.ModifyInstanceAttributeInput{
		InstanceId: aws.String(string(instanceID)),
		UserData: &ec2types.BlobAttributeValue{
			Value: []byte(userData),
		},
	})
	if err != nil {
		return errors.WrapAndTrace(err)
	}
	return nil
}

func waitForEc2State(ctx context.Context, client *ec2.Client, instanceID ids.CloudProviderInstanceID, state ec2types.InstanceStateName, maxWait int) error {
	tracer := otel.GetTracerProvider().Tracer("")
	ctx, span := tracer.Start(ctx, "ec2.waitForState")
	defer span.End()
	// create vars here b/c they are used outside the loop and go is lexically scoped
	var res *ec2.DescribeInstancesOutput
	var err error
	for maxWait > 0 {
		res, err = client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
			InstanceIds: []string{string(instanceID)},
		})
		if err != nil {
			return errors.WrapAndTrace(err)
		}
		if len(res.Reservations) == 0 {
			return errors.New("no reservations found")
		}
		if len(res.Reservations[0].Instances) == 0 {
			return errors.New("no instances found")
		}
		if res.Reservations[0].Instances[0].State.Name == state {
			return nil
		}
		time.Sleep(time.Second)
		maxWait--
	}
	return fmt.Errorf("timeout waiting for state %s, current state %s", state, res.Reservations[0].Instances[0].State.Name)
}

func (a AWSClient) StopInstance(ctx context.Context, instanceID ids.CloudProviderInstanceID) error {
	_, err := a.ec2Client.StopInstances(ctx, &ec2.StopInstancesInput{
		InstanceIds: []string{string(instanceID)},
		Force:       aws.Bool(true),
	})
	return errors.WrapAndTrace(handleAWSError(err))
}

func (a AWSClient) StartInstance(ctx context.Context, instanceID ids.CloudProviderInstanceID) error {
	_, err := a.ec2Client.StartInstances(ctx, &ec2.StartInstancesInput{
		InstanceIds: []string{string(instanceID)},
	})
	return errors.WrapAndTrace(handleAWSError(err))
}

var ErrInstanceNotFound = fmt.Errorf("instance not found")

func handleAWSError(e error) error {
	if e == nil {
		return nil
	}
	if strings.Contains(e.Error(), "InvalidInstanceID.NotFound") {
		return ErrInstanceNotFound
	} else {
		return e
	}
}

func (a AWSClient) GetInstance(ctx context.Context, instanceID ids.CloudProviderInstanceID) (*Instance, error) {
	res, err := a.ec2Client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: []string{string(instanceID)},
	})
	if err != nil {
		return nil, errors.WrapAndTrace(handleAWSError(err))
	}
	if len(res.Reservations) == 0 {
		return nil, errors.WrapAndTrace(ErrInstanceNotFound)
	}
	if len(res.Reservations[0].Instances) == 0 {
		return nil, errors.WrapAndTrace(ErrInstanceNotFound)
	}
	instance := res.Reservations[0].Instances[0]

	vols, err := a.GetInstanceVolumes(ctx, instanceID)
	if err != nil {
		return nil, errors.WrapAndTrace(err)
	}
	if len(vols) == 0 {
		return nil, errors.New("no volumes found")
	}
	diskSize := Int32GiBToUnit(*vols[0].Size)

	keyFingerprint := ""
	if instance.KeyName != nil {
		var keyPair *KeyPair
		keyPair, err = a.GetKeyPairByName(ctx, *instance.KeyName)
		if err != nil {
			return nil, errors.WrapAndTrace(err)
		}
		keyFingerprint = keyPair.KeyFingerprint
	}

	status, err := AWSInstanceStateToLifecyclState(*instance.State)
	if err != nil {
		return nil, errors.WrapAndTrace(err)
	}

	i := AWSInstanceToInstanceAttrs(instance, status, diskSize, keyFingerprint)
	if err != nil {
		return nil, errors.WrapAndTrace(err)
	}
	return &i, nil
}

func AWSInstanceToInstanceAttrs(instance ec2types.Instance, status LifecycleStatus, diskSize units.Base2Bytes, publicKeyFingerprint string) Instance {
	return Instance{
		ID:                  ids.CloudProviderInstanceID(*instance.InstanceId),
		Hostname:            collections.ValueOrZero(instance.PublicDnsName),
		ImageID:             collections.ValueOrZero(instance.ImageId),
		InstanceType:        string(instance.InstanceType),
		DiskSize:            diskSize,
		PubKeyFingerprint:   publicKeyFingerprint,
		Status:              Status{LifecycleStatus: status},
		MetaEndpointEnabled: collections.ValueOrZero(instance.MetadataOptions).HttpEndpoint == ec2types.InstanceMetadataEndpointStateEnabled,
		MetaTagsEnabled:     collections.ValueOrZero(instance.MetadataOptions).InstanceMetadataTags == ec2types.InstanceMetadataTagsStateEnabled,
		VPCID:               collections.ValueOrZero(instance.VpcId),
		SubnetID:            collections.ValueOrZero(instance.SubnetId),
		Spot:                collections.ValueOrZero(instance.SpotInstanceRequestId) != "",
		Name:                getNameTag(instance.Tags),
	}
}

func getNameTag(tags []ec2types.Tag) string {
	for _, tag := range tags {
		if *tag.Key == "Name" {
			return *tag.Value
		}
	}
	return ""
}

func Int32GiBToUnit(i int32) units.Base2Bytes {
	return units.Base2Bytes(i) * units.GiB
}

func (a AWSClient) GetInstanceVolumes(ctx context.Context, instanceID ids.CloudProviderInstanceID) ([]ec2types.Volume, error) {
	result, err := a.ec2Client.DescribeVolumes(ctx, &ec2.DescribeVolumesInput{
		Filters: []ec2types.Filter{
			{
				Name:   aws.String("attachment.instance-id"),
				Values: []string{string(instanceID)},
			},
		},
	})
	if err != nil {
		return nil, errors.WrapAndTrace(err)
	}
	return result.Volumes, nil
}

func AWSInstanceStateToLifecyclState(status ec2types.InstanceState) (LifecycleStatus, error) {
	switch status.Name {
	case ec2types.InstanceStateNamePending:
		return LifecycleStatePending, nil
	case ec2types.InstanceStateNameRunning:
		return LifecycleStateRunning, nil
	case ec2types.InstanceStateNameStopping:
		return LifecycleStateStopping, nil
	case ec2types.InstanceStateNameStopped:
		return LifecycleStateStopped, nil
	case ec2types.InstanceStateNameShuttingDown:
		return LifecycleStateTerminating, nil
	case ec2types.InstanceStateNameTerminated:
		return LifecycleStateTerminated, nil
	}
	return "", fmt.Errorf("unknown instance state: %s", status.Name)
}

const (
	LifecycleStatePending     LifecycleStatus = "pending"
	LifecycleStateRunning     LifecycleStatus = "running"
	LifecycleStateStopping    LifecycleStatus = "stopping"
	LifecycleStateStopped     LifecycleStatus = "stopped"
	LifecycleStateSuspending  LifecycleStatus = "suspending"
	LifecycleStateSuspended   LifecycleStatus = "suspended"
	LifecycleStateTerminating LifecycleStatus = "terminating"
	LifecycleStateTerminated  LifecycleStatus = "terminated"
)

type UserDataArgs struct {
	OnBootScript     string
	OnInstanceScript string
	OnOnceScript     string
}

func (a AWSClient) GetKeyPairByName(ctx context.Context, name string) (*KeyPair, error) {
	result, err := a.ec2Client.DescribeKeyPairs(ctx, &ec2.DescribeKeyPairsInput{
		KeyNames: []string{name},
	})
	if err != nil {
		return nil, errors.WrapAndTrace(err)
	}
	if len(result.KeyPairs) == 0 {
		return nil, errors.New("no key pairs found")
	}
	keyPair := result.KeyPairs[0]
	return &KeyPair{
		KeyFingerprint: *keyPair.KeyFingerprint,
		KeyName:        *keyPair.KeyName,
		KeyPairID:      collections.ValueOrZero(keyPair.KeyPairId), // undo when local stack fixes
	}, nil
}

const devPlaneUserDataTemplate = `Content-Type: multipart/mixed; boundary="===============7279599212584821875=="
MIME-Version: 1.0

--===============7279599212584821875==
Content-Type: text/x-shellscript-per-boot; charset="utf-8"
MIME-Version: 1.0
Content-Transfer-Encoding: base64
Content-Disposition: attachment; filename="always.sh"


%s
--===============7279599212584821875==
Content-Type: text/x-shellscript-per-instance; charset="utf-8"
MIME-Version: 1.0
Content-Transfer-Encoding: base64
Content-Disposition: attachment; filename="instance.sh"

%s

--===============7279599212584821875==
Content-Type: text/x-shellscript-per-once; charset="utf-8"
MIME-Version: 1.0
Content-Transfer-Encoding: base64
Content-Disposition: attachment; filename="once.sh"

%s

--===============7279599212584821875==--`

func makeUserData(args UserDataArgs) string {
	userdata := fmt.Sprintf(devPlaneUserDataTemplate,
		base64.StdEncoding.EncodeToString([]byte(args.OnBootScript)),
		base64.StdEncoding.EncodeToString([]byte(args.OnInstanceScript)),
		base64.StdEncoding.EncodeToString([]byte(args.OnOnceScript)))

	return userdata // base64.StdEncoding.EncodeToString(
}

func askForConfirmation() (bool, error) {
	var response string
	_, err := fmt.Scanln(&response)
	if err != nil {
		return false, errors.WrapAndTrace(err)
	}
	okayResponses := []string{"y", "Y", "yes", "Yes", "YES"}
	nokayResponses := []string{"n", "N", "no", "No", "NO"}
	if containsString(okayResponses, response) {
		return true, nil
	} else if containsString(nokayResponses, response) {
		return false, nil
	}
	fmt.Println("Please type yes or no and then press enter:")
	return askForConfirmation()
}

func containsString(slice []string, element string) bool {
	return !(posString(slice, element) == -1)
}

func posString(slice []string, element string) int {
	for index, elem := range slice {
		if elem == element {
			return index
		}
	}
	return -1
}
