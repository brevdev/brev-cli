package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

const (
	instancesDirectory = "./instances"

	instanceType = ec2types.InstanceTypeG4dnXlarge // g4dn.xlarge = 1xT4
	imageId      = "ami-0f5fcdfbd140e4ab7"         // ami-0f5fcdfbd140e4ab7 = Ubuntu Server 24.04 LTS
	region       = "us-east-2"
)

func main() {
	ctx := context.Background()

	awsConfig, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		panic(err)
	}
	ec2Client := ec2.NewFromConfig(awsConfig)

	fmt.Printf("EC2 client configured for region: %s\n", region)

	err = run(ctx, ec2Client)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}

	os.Exit(0)
}

func run(ctx context.Context, ec2Client *ec2.Client) error {
	if len(os.Args) < 2 {
		return fmt.Errorf("usage: %s <migrations_directory>", os.Args[0])
	}

	switch os.Args[1] {
	case "create":
		fmt.Println("Creating EC2 instance...")
		return create(ctx, ec2Client)
	case "ssh":
		if len(os.Args) < 3 {
			return fmt.Errorf("usage: %s ssh <instance_id>", os.Args[0])
		}
		fmt.Printf("SSHing into EC2 instance: %s\n", os.Args[2])
		return ssh(ctx, ec2Client, os.Args[2])
	case "scp":
		if len(os.Args) < 4 {
			return fmt.Errorf("usage: %s scp <instance_id> <local_file_path>", os.Args[0])
		}
		fmt.Printf("SCPing from %s on EC2 instance: %s\n", os.Args[2], os.Args[3])
		return scp(ctx, ec2Client, os.Args[2], os.Args[3])
	case "list":
		fmt.Println("Listing EC2 instances...")
		return list(ctx, ec2Client)
	case "delete":
		if len(os.Args) < 3 {
			return fmt.Errorf("usage: %s delete <instance_id>", os.Args[0])
		}
		fmt.Printf("Deleting EC2 instance: %s\n", os.Args[2])
		return delete(ctx, ec2Client, os.Args[2])
	}

	return nil
}

func create(ctx context.Context, ec2Client *ec2.Client) error {
	id := fmt.Sprintf("brev-cli-test-%d", time.Now().Unix())

	// Create a local directory for the below resources
	baseDir := instancesDirectory
	instanceDir := filepath.Join(baseDir, id)
	err := os.MkdirAll(instanceDir, 0o755)
	if err != nil {
		return err
	}

	// Create a security group that allows SSH access
	fmt.Println("Creating security group...")
	createSecurityGroupOutput, err := ec2Client.CreateSecurityGroup(ctx, &ec2.CreateSecurityGroupInput{
		GroupName:   aws.String(id),
		Description: aws.String("Allow SSH access for Brev CLI Test"),
		TagSpecifications: []ec2types.TagSpecification{
			{
				ResourceType: ec2types.ResourceTypeSecurityGroup,
				Tags: []ec2types.Tag{
					{
						Key:   aws.String("Name"),
						Value: aws.String(id),
					},
					{
						Key:   aws.String("Owner"),
						Value: aws.String("Brev CLI Test"),
					},
				},
			},
		},
	})
	if err != nil {
		return err
	}
	securityGroupId := *createSecurityGroupOutput.GroupId

	// Store the security group ID in the local directory
	err = os.WriteFile(filepath.Join(instanceDir, "security_group_id.txt"), []byte(securityGroupId), 0o644)
	if err != nil {
		return err
	}

	// Add SSH inbound rule to the security group
	fmt.Println("Adding SSH inbound rule to security group...")
	_, err = ec2Client.AuthorizeSecurityGroupIngress(ctx, &ec2.AuthorizeSecurityGroupIngressInput{
		GroupId: aws.String(securityGroupId),
		IpPermissions: []ec2types.IpPermission{
			{
				IpProtocol: aws.String("tcp"),
				FromPort:   aws.Int32(22),
				ToPort:     aws.Int32(22),
				IpRanges: []ec2types.IpRange{
					{
						CidrIp:      aws.String("0.0.0.0/0"),
						Description: aws.String("Allow SSH from anywhere"),
					},
				},
			},
		},
	})
	if err != nil {
		return err
	}

	// Create unique keypair for the instance
	fmt.Println("Creating keypair...")
	createKeyPairOutput, err := ec2Client.CreateKeyPair(ctx, &ec2.CreateKeyPairInput{
		KeyName: aws.String(id),
		KeyType: ec2types.KeyTypeRsa,
		TagSpecifications: []ec2types.TagSpecification{
			{
				ResourceType: ec2types.ResourceTypeKeyPair,
				Tags: []ec2types.Tag{
					{
						Key:   aws.String("Name"),
						Value: aws.String(id),
					},
					{
						Key:   aws.String("Owner"),
						Value: aws.String("Brev CLI Test"),
					},
				},
			},
		},
	})
	if err != nil {
		return err
	}
	keyName := *createKeyPairOutput.KeyName

	// Store the key name in the local directory
	err = os.WriteFile(filepath.Join(instanceDir, "key_name.txt"), []byte(keyName), 0o644)
	if err != nil {
		return err
	}

	// Store the key .pem file in the local directory
	err = os.WriteFile(filepath.Join(instanceDir, "key.pem"), []byte(*createKeyPairOutput.KeyMaterial), 0o400)
	if err != nil {
		return err
	}

	// Create EC2 instance for use in testing with BYON
	fmt.Println("Creating EC2 instance with configuration:")
	fmt.Printf("\tInstance type: %s\n", instanceType)
	fmt.Printf("\tImage ID: %s\n", imageId)
	fmt.Printf("\tKey name: %s\n", keyName)
	fmt.Printf("\tSecurity group ID: %s\n", securityGroupId)
	fmt.Printf("\tMin count: 1\n")
	fmt.Printf("\tMax count: 1\n")
	runInstancesOutput, err := ec2Client.RunInstances(ctx, &ec2.RunInstancesInput{
		InstanceType:     instanceType,
		ImageId:          aws.String(imageId),
		KeyName:          aws.String(keyName),
		SecurityGroupIds: []string{securityGroupId},
		MinCount:         aws.Int32(1),
		MaxCount:         aws.Int32(1),
		TagSpecifications: []ec2types.TagSpecification{
			{
				ResourceType: ec2types.ResourceTypeInstance,
				Tags: []ec2types.Tag{
					{
						Key:   aws.String("Name"),
						Value: aws.String(id),
					},
					{
						Key:   aws.String("Owner"),
						Value: aws.String("Brev CLI Test"),
					},
				},
			},
		},
	})
	if err != nil {
		return err
	}
	instanceId := *runInstancesOutput.Instances[0].InstanceId

	// Store the instance ID in the local directory
	err = os.WriteFile(filepath.Join(instanceDir, "instance_id.txt"), []byte(instanceId), 0o644)
	if err != nil {
		return err
	}

	return nil
}

func ssh(ctx context.Context, ec2Client *ec2.Client, instanceId string) error {
	// Read the instance ID from the local directory
	instanceIdBytes, err := os.ReadFile(filepath.Join(instancesDirectory, instanceId, "instance_id.txt"))
	if err != nil {
		return err
	}
	ec2InstanceId := string(instanceIdBytes)

	// Get the instance details
	describeInstancesOutput, err := ec2Client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: []string{ec2InstanceId},
	})
	if err != nil {
		return err
	}
	ec2Instance := describeInstancesOutput.Reservations[0].Instances[0]

	// Build the path to the .pem file
	pemFilePath := filepath.Join(instancesDirectory, instanceId, "key.pem")

	// SSH into the instance
	fmt.Printf("Connecting to %s...\n", *ec2Instance.PublicIpAddress)
	cmd := exec.Command("ssh", "-i", pemFilePath, fmt.Sprintf("ubuntu@%s", *ec2Instance.PublicIpAddress))
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("SSH connection failed: %w", err)
	}

	return nil
}

func scp(ctx context.Context, ec2Client *ec2.Client, instanceId string, localFilePath string) error {
	// Read the instance ID from the local directory
	instanceIdBytes, err := os.ReadFile(filepath.Join(instancesDirectory, instanceId, "instance_id.txt"))
	if err != nil {
		return err
	}
	ec2InstanceId := string(instanceIdBytes)

	// Get the instance details
	describeInstancesOutput, err := ec2Client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: []string{ec2InstanceId},
	})
	if err != nil {
		return err
	}
	ec2Instance := describeInstancesOutput.Reservations[0].Instances[0]

	// Build the path to the .pem file
	pemFilePath := filepath.Join(instancesDirectory, instanceId, "key.pem")

	// SCP the file to the instance
	cmd := exec.Command("scp", "-i", pemFilePath, localFilePath, fmt.Sprintf("ubuntu@%s:%s", *ec2Instance.PublicIpAddress, "/home/ubuntu"))
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("SCP failed: %w", err)
	}

	return nil
}

func list(ctx context.Context, ec2Client *ec2.Client) error {
	// Abort if the instances directory does not exist
	if _, err := os.Stat(instancesDirectory); os.IsNotExist(err) {
		return nil
	}

	entries, err := os.ReadDir(instancesDirectory)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		fmt.Println(entry.Name())

		// Read the instance ID from the local directory
		instanceIdBytes, err := os.ReadFile(filepath.Join(instancesDirectory, entry.Name(), "instance_id.txt"))
		if err != nil {
			return err
		}
		ec2InstanceId := string(instanceIdBytes)

		// Get the instance details
		describeInstancesOutput, err := ec2Client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
			InstanceIds: []string{ec2InstanceId},
		})
		if err != nil {
			return err
		}
		ec2Instance := describeInstancesOutput.Reservations[0].Instances[0]
		// Print as if JSON
		fmt.Printf("\tInstance ID: %s\n", *ec2Instance.InstanceId)
		fmt.Printf("\tInstance type: %s\n", ec2Instance.InstanceType)
		fmt.Printf("\tImage ID: %s\n", *ec2Instance.ImageId)
		fmt.Printf("\tKey name: %s\n", *ec2Instance.KeyName)
		fmt.Printf("\tPrivate IP address: %s\n", *ec2Instance.PrivateIpAddress)
		fmt.Printf("\tPublic IP address: %s\n", *ec2Instance.PublicIpAddress)
		fmt.Printf("\tState: %s\n", ec2Instance.State.Name)
	}
	return nil
}

func delete(ctx context.Context, ec2Client *ec2.Client, instanceId string) error {
	// Read the instance ID from the local directory
	instanceIdBytes, err := os.ReadFile(filepath.Join(instancesDirectory, instanceId, "instance_id.txt"))
	if err != nil {
		return err
	}
	ec2InstanceId := string(instanceIdBytes)

	// Read the key name from the local directory
	keyNameBytes, err := os.ReadFile(filepath.Join(instancesDirectory, instanceId, "key_name.txt"))
	if err != nil {
		return err
	}
	keyName := string(keyNameBytes)

	// Read the security group ID from the local directory
	securityGroupIdBytes, err := os.ReadFile(filepath.Join(instancesDirectory, instanceId, "security_group_id.txt"))
	if err != nil {
		return err
	}
	securityGroupId := string(securityGroupIdBytes)

	// Delete the instance
	fmt.Printf("Terminating instance: %s\n", ec2InstanceId)
	_, err = ec2Client.TerminateInstances(ctx, &ec2.TerminateInstancesInput{
		InstanceIds: []string{ec2InstanceId},
	})
	if err != nil {
		return err
	}

	// Wait for instance to terminate before deleting security group (note, this can easily take more than 5 minutes)
	fmt.Println("Waiting for instance to terminate...")
	waiter := ec2.NewInstanceTerminatedWaiter(ec2Client)
	err = waiter.Wait(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: []string{ec2InstanceId},
	}, 15*time.Minute)
	if err != nil {
		fmt.Printf("Warning: error waiting for instance termination: %v\n", err)
	}

	// Delete the security group
	fmt.Printf("Deleting security group: %s\n", securityGroupId)
	_, err = ec2Client.DeleteSecurityGroup(ctx, &ec2.DeleteSecurityGroupInput{
		GroupId: aws.String(securityGroupId),
	})
	if err != nil {
		return err
	}

	// Delete the key pair
	fmt.Printf("Deleting key pair: %s\n", keyName)
	_, err = ec2Client.DeleteKeyPair(ctx, &ec2.DeleteKeyPairInput{
		KeyName: aws.String(keyName),
	})
	if err != nil {
		return err
	}

	// Delete the local directory
	fmt.Printf("Deleting local directory: %s\n", filepath.Join(instancesDirectory, instanceId))
	err = os.RemoveAll(filepath.Join(instancesDirectory, instanceId))
	if err != nil {
		return err
	}

	return nil
}
