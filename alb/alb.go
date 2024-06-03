package alb

import (
	"fmt"
	"os"

	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/ec2"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/lb"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"gopkg.in/yaml.v3"
)

type CommonConfig struct {
	VPCId   string   `yaml:"vpcid"`
	Subnets []string `yaml:"subnets"`
}

type albConfig struct {
	Port int `yaml:"port"`
	CommonConfig
	Input_CIDR []string `yaml:"allowed_cidrs"`
}

func loadConfig(filename string) (*albConfig, error) {

	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var config albConfig
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

func createALB(ctx *pulumi.Context, configFile string) error {

	config, err := loadConfig(configFile)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %v", err)
	}

	securityGroup, err := ec2.NewSecurityGroup(ctx, "albSecurityGroup", &ec2.SecurityGroupArgs{
		Name:        pulumi.String("ALB Security group"),
		Description: pulumi.String("Allow http inbound traffic"),
		VpcId:       pulumi.String(config.VPCId),
		Ingress: ec2.SecurityGroupIngressArray{
			&ec2.SecurityGroupIngressArgs{
				Description: pulumi.String("allow TCP"),
				FromPort:    pulumi.Int(config.Port),
				ToPort:      pulumi.Int(config.Port),
				Protocol:    pulumi.String("tcp"),
				CidrBlocks:  pulumi.ToStringArray(config.Input_CIDR),
			},
		},
		Egress: ec2.SecurityGroupEgressArray{
			&ec2.SecurityGroupEgressArgs{
				FromPort: pulumi.Int(0),
				ToPort:   pulumi.Int(0),
				Protocol: pulumi.String("-1"),
			},
		},
	})
	if err != nil {
		return err
	}

	alb, err := lb.NewLoadBalancer(ctx, "appLoadBalancer", &lb.LoadBalancerArgs{
		SecurityGroups: pulumi.StringArray{securityGroup.ID()},
		Subnets:        pulumi.ToStringArray(config.Subnets), // Replace with actual Subnet IDs
	})
	if err != nil {
		return err
	}

	targetGroup, err := lb.NewTargetGroup(ctx, "appTargetGroup", &lb.TargetGroupArgs{
		Port:       pulumi.Int(80),
		Protocol:   pulumi.String("HTTP"),
		VpcId:      pulumi.String(config.VPCId),
		TargetType: pulumi.String("instance"),
	})
	if err != nil {
		return err
	}

	_, err = lb.NewListener(ctx, "listener", &lb.ListenerArgs{
		LoadBalancerArn: alb.Arn,
		Port:            pulumi.Int(config.Port),
		DefaultActions: lb.ListenerDefaultActionArray{
			&lb.ListenerDefaultActionArgs{
				Type:           pulumi.String("forward"),
				TargetGroupArn: targetGroup.Arn,
			},
		},
	})
	if err != nil {
		return err
	}

	return nil
}
