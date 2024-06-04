package alb

import (
	"fmt"
	"os"

	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/ec2"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/lb"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"gopkg.in/yaml.v3"
)

type ALBConfig struct {
	ALB struct {
		Port        int      `yaml:"port"`
		AllowedCIDR []string `yaml:"allowed_cidrs"`
	} `yaml:"alb"`
}

func loadConfig(filename string) (*ALBConfig, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var config ALBConfig
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

func CreateALB(ctx *pulumi.Context, configFile string, vpcID pulumi.IDOutput, subnets pulumi.StringArrayOutput) (pulumi.StringOutput, pulumi.IDOutput, error) {

	config, err := loadConfig(configFile)
	if err != nil {
		return pulumi.StringOutput{}, pulumi.IDOutput{}, fmt.Errorf("failed to load configuration: %v", err)
	}

	securityGroup, err := ec2.NewSecurityGroup(ctx, "albSecurityGroup", &ec2.SecurityGroupArgs{
		Name:        pulumi.String("ALB Security group"),
		Description: pulumi.String("Allow http inbound traffic"),
		VpcId:       vpcID,
		Ingress: ec2.SecurityGroupIngressArray{
			&ec2.SecurityGroupIngressArgs{
				Description: pulumi.String("allow TCP"),
				FromPort:    pulumi.Int(config.ALB.Port),
				ToPort:      pulumi.Int(config.ALB.Port),
				Protocol:    pulumi.String("tcp"),
				CidrBlocks:  pulumi.ToStringArray(config.ALB.AllowedCIDR),
			},
		},
	})
	if err != nil {
		return pulumi.StringOutput{}, pulumi.IDOutput{}, err
	}

	alb, err := lb.NewLoadBalancer(ctx, "appLoadBalancer", &lb.LoadBalancerArgs{
		SecurityGroups: pulumi.StringArray{securityGroup.ID()},
		Subnets:        subnets,
	}, pulumi.DependsOn([]pulumi.Resource{securityGroup}))
	if err != nil {
		return pulumi.StringOutput{}, pulumi.IDOutput{}, err
	}

	targetGroup, err := lb.NewTargetGroup(ctx, "appTargetGroup", &lb.TargetGroupArgs{
		Port:       pulumi.Int(config.ALB.Port),
		Protocol:   pulumi.String("HTTP"),
		VpcId:      vpcID,
		TargetType: pulumi.String("instance"),
	})
	if err != nil {
		return pulumi.StringOutput{}, pulumi.IDOutput{}, err
	}

	_, err = lb.NewListener(ctx, "listener", &lb.ListenerArgs{
		LoadBalancerArn: alb.Arn,
		Port:            pulumi.Int(config.ALB.Port),
		DefaultActions: lb.ListenerDefaultActionArray{
			&lb.ListenerDefaultActionArgs{
				Type:           pulumi.String("forward"),
				TargetGroupArn: targetGroup.Arn,
			},
		},
	}, pulumi.DependsOn([]pulumi.Resource{targetGroup, alb}))
	if err != nil {
		return pulumi.StringOutput{}, pulumi.IDOutput{}, err
	}

	return targetGroup.Arn, securityGroup.ID(), nil
}
