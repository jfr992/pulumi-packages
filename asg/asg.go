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
	ASG struct {
		ASGName           string   `yaml:"name"`
		AMI_ID            string   `yaml:"ami-id"`
		InstanceType      string   `yaml:"instance-type"`
		MinSize           int      `yaml:"min-size"`
		MaxSize           int      `yaml:"max-size"`
		DesiredCapacity   int      `yaml:"desired-capacity"`
		AvailabilityZones []string `yaml:"azs"`
		Ports             []int    `yaml:"ports"`
	} `yaml:"asg"`
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

func stringOutputToString(input pulumi.StringOutput) pulumi.String {
	var result pulumi.String
	input.ApplyT(func(v string) string {
		result = pulumi.String(v)
		return v
	})
	return result
}

func CreateASG(ctx *pulumi.Context, configFile string, userdata string, vpcID pulumi.IDOutput, subnets pulumi.StringArrayOutput, targetGroupArn pulumi.StringOutput, sourceSecurityGroupId pulumi.IDOutput) error {

	targetGroupArnString := stringOutputToString(targetGroupArn)

	// loading config

	config, err := loadConfig(configFile)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %v", err)
	}
	userDataBytes, err := os.ReadFile(userdata)

	if err != nil {
		return fmt.Errorf("failed to read user data script: %v", err)
	}

	userData := base64.StdEncoding.EncodeToString(userDataBytes)

	instancesSecurityGroup, err := ec2.NewSecurityGroup(ctx, "instanceSecurityGroup", &ec2.SecurityGroupArgs{
		Description: pulumi.String("Security group for the instances"),
		VpcId:       vpcID,
	})
	if err != nil {
		return err
	}

	for _, port := range config.ASG.Ports {
		_, err := ec2.NewSecurityGroupRule(ctx, fmt.Sprintf("ingressRule-%d", port), &ec2.SecurityGroupRuleArgs{
			Type:                  pulumi.String("ingress"),
			SecurityGroupId:       instancesSecurityGroup.ID(),
			FromPort:              pulumi.Int(port),
			ToPort:                pulumi.Int(port),
			Protocol:              pulumi.String("tcp"),
			SourceSecurityGroupId: sourceSecurityGroupId,
		})
		if err != nil {
			return err
		}
	}
	lt, err := ec2.NewLaunchTemplate(ctx, "launchtemplate", &ec2.LaunchTemplateArgs{
		NamePrefix:          pulumi.String(config.ASG.ASGName),
		ImageId:             pulumi.String(config.ASG.AMI_ID),
		InstanceType:        pulumi.String(config.ASG.InstanceType),
		VpcSecurityGroupIds: pulumi.StringArray{instancesSecurityGroup.ID()},
		UserData:            pulumi.String(userData),
	}, pulumi.DependsOn([]pulumi.Resource{instancesSecurityGroup}))
	if err != nil {
		return err
	}

	_, err = autoscaling.NewGroup(ctx, "asg", &autoscaling.GroupArgs{
		VpcZoneIdentifiers: subnets,
		DesiredCapacity:    pulumi.Int(config.ASG.DesiredCapacity),
		MaxSize:            pulumi.Int(config.ASG.DesiredCapacity),
		MinSize:            pulumi.Int(config.ASG.DesiredCapacity),
		LaunchTemplate: &autoscaling.GroupLaunchTemplateArgs{
			Id:      lt.ID(),
			Version: pulumi.String("$Latest"),
		},
		TargetGroupArns: pulumi.StringArray{targetGroupArnString},
	}, pulumi.DependsOn([]pulumi.Resource{lt}))

	if err != nil {
		return err
	}

	return nil
}
