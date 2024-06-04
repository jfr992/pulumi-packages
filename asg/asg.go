package asg

import (
	"encoding/base64"
	"fmt"
	"os"

	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/autoscaling"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/ec2"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"gopkg.in/yaml.v3"
)

type asgConfig struct {
	Name              string   `yaml:"name"`
	Ami_Id            string   `yaml:"ami-id"`
	Instance_Type     string   `yaml:"instance-type"`
	MinSize           int      `yaml:"min-size"`
	MaxSize           int      `yaml:"max-size"`
	DesiredCapacity   int      `yaml:"desired-capacity"`
	AvailabilityZones []string `yaml:"azs"`
	Ports             []int    `yaml:"ports"`
}

func loadConfig(filename string) (*asgConfig, error) {

	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var config asgConfig
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

func createASG(ctx *pulumi.Context, configFile string, userdata string, vpcID string, targetGroupArn []string, sourceSecurityGroupId string) error {
	userDataBytes, err := os.ReadFile(userdata)

	if err != nil {
		return fmt.Errorf("failed to read user data script: %v", err)
	}

	userData := base64.StdEncoding.EncodeToString(userDataBytes)

	// loading config

	config, err := loadConfig(configFile)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %v", err)
	}

	pulumi.Run(func(ctx *pulumi.Context) error {

		instancesSecurityGroup, err := ec2.NewSecurityGroup(ctx, "instanceSecurityGroup", &ec2.SecurityGroupArgs{
			Description: pulumi.String("Security group for the instances"),
			VpcId:       pulumi.String(vpcID),
		})
		if err != nil {
			return err
		}

		for _, port := range config.Ports {
			_, err := ec2.NewSecurityGroupRule(ctx, fmt.Sprintf("ingressRule-%d", port), &ec2.SecurityGroupRuleArgs{
				Type:                  pulumi.String("ingress"),
				SecurityGroupId:       instancesSecurityGroup.ID(),
				FromPort:              pulumi.Int(port),
				ToPort:                pulumi.Int(port),
				Protocol:              pulumi.String("tcp"),
				SourceSecurityGroupId: pulumi.String(sourceSecurityGroupId),
			})
			if err != nil {
				return err
			}
		}
		lt, err := ec2.NewLaunchTemplate(ctx, "launchtemplate", &ec2.LaunchTemplateArgs{
			NamePrefix:          pulumi.String(config.Name),
			ImageId:             pulumi.String(config.Ami_Id),
			InstanceType:        pulumi.String(config.Instance_Type),
			VpcSecurityGroupIds: pulumi.StringArray{instancesSecurityGroup.ID()},
			UserData:            pulumi.String(userData),
			IamInstanceProfile: &ec2.LaunchTemplateIamInstanceProfileArgs{
				Arn:  pulumi.String("string"),
				Name: pulumi.String("string"),
			},
		}, pulumi.DependsOn([]pulumi.Resource{instancesSecurityGroup}))
		if err != nil {
			return err
		}

		_, err = autoscaling.NewGroup(ctx, "asg", &autoscaling.GroupArgs{
			AvailabilityZones: pulumi.ToStringArray(config.AvailabilityZones),
			DesiredCapacity:   pulumi.Int(config.DesiredCapacity),
			MaxSize:           pulumi.Int(config.MaxSize),
			MinSize:           pulumi.Int(config.MinSize),
			LaunchTemplate: &autoscaling.GroupLaunchTemplateArgs{
				Id:      lt.ID(),
				Version: pulumi.String("$Latest"),
			},
			TargetGroupArns: pulumi.ToStringArray(targetGroupArn),
		}, pulumi.DependsOn([]pulumi.Resource{lt}))

		if err != nil {
			return err
		}

		return nil
	})
	return nil
}