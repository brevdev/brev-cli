package savemoney

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/costexplorer"
	"github.com/aws/aws-sdk-go-v2/service/dlm"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/pricing"
	"github.com/aws/aws-sdk-go-v2/service/servicequotas"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/brevdev/brev-cli/pkg/errors"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/brevdev/dev-plane/internal/ids"
	"github.com/spf13/cobra"
)

var (
	short   = "TODO"
	long    = "TODO"
	example = "TODO"
)

type saveMoneyStore interface{}

type AWSClient struct {
	location           string
	ec2Client          *ec2.Client
	quotaClient        *servicequotas.Client
	pricingClient      *pricing.Client
	costExplorerClient *costexplorer.Client
	dlmClient          *dlm.Client
	iamClient          *iam.Client
}

func NewCmdSaveMoney(t *terminal.Terminal, store saveMoneyStore) *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "save-money",
		DisableFlagsInUseLine: true,
		Short:                 short,
		Long:                  long,
		Example:               example,
		RunE: func(cmd *cobra.Command, args []string) error {
			err := SaveMoney(t, args, store)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
	}
	return cmd
}

func SaveMoney(t *terminal.Terminal, args []string, store saveMoneyStore) error {
	fmt.Println("Save monay")
	fmt.Println(args)
	// so these args are then the instance ids, first thing we want to do is

	// first, we want to create an aws client
	config, err := GetLiveFileConfig(context.Background()) // TODO: make sure the context is good
	if err != nil {
		return errors.WrapAndTrace(err)
	}
	awsClient := GetAWSClient(config)

	return nil
}

func GetLiveFileConfig(ctx context.Context) (aws.Config, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return aws.Config{}, errors.WrapAndTrace(err)
	}
	cfg.Region = "us-west-2"
	return cfg, nil
}

func GetAWSClient(cfg aws.Config) AWSClient {
	ec2Client := ec2.NewFromConfig(cfg)
	pricingConfig := cfg.Copy()
	pricingConfig.Region = pricingRegion
	pricingClient := pricing.NewFromConfig(pricingConfig)
	quotaClient := servicequotas.NewFromConfig(cfg)
	costExplorerCLient := costexplorer.NewFromConfig(cfg)
	dlmClient := dlm.NewFromConfig(cfg)
	iamClient := iam.NewFromConfig(cfg)
	return NewAWSClient(ec2Client, pricingClient, quotaClient, costExplorerCLient, dlmClient, iamClient, pricingRegion)
}

// type AWSCredential struct {
// 	AccessKeyID     string `json:"access_key_id"`
// 	SecretAccessKey string `json:"secret_access_key"`
// 	RoleARN         string `json:"role_arn"`
// 	ExternalID      string `json:"external_id"`
// }

// func (a AWSCredential) Validate() error {
// 	return validation.ValidateStruct(&a,
// 		validation.Field(&a.SecretAccessKey, validation.Required),
// 		validation.Field(&a.AccessKeyID, validation.Required),
// 		validation.Field(&a.RoleARN, validation.When(a.ExternalID != "", validation.Required.Error("rolARN is required to set ExternalID"))),
// 		validation.Field(&a.ExternalID),
// 	)
// }

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
