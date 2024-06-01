package network

import (
	"fmt"
	"os"

	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/ec2"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"gopkg.in/yaml.v3"
)

type VpcConfig struct {
	Name      string `yaml:"name"`
	CidrBlock string `yaml:"cidr_block"`
}

type SubnetConfig struct {
	Name      string `yaml:"name"`
	CidrBlock string `yaml:"cidr_block"`
	Az        string `yaml:"az"`
	Public    bool   `yaml:"public"`
}

type NetworkConfig struct {
	Vpc     VpcConfig      `yaml:"vpc"`
	Subnets []SubnetConfig `yaml:"subnets"`
}

func loadConfig(filename string) (*NetworkConfig, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var config NetworkConfig
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

func CreateNetwork(ctx *pulumi.Context, configFile string) error {
	// Load configuration from YAML file
	config, err := loadConfig(configFile)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %v", err)
	}

	//  VPC
	vpc, err := ec2.NewVpc(ctx, config.Vpc.Name, &ec2.VpcArgs{
		CidrBlock:          pulumi.String(config.Vpc.CidrBlock),
		EnableDnsSupport:   pulumi.Bool(true),
		EnableDnsHostnames: pulumi.Bool(true),
	})
	if err != nil {
		return err
	}

	//  Subnets
	for _, subnetConfig := range config.Subnets {
		_, err := ec2.NewSubnet(ctx, subnetConfig.Name, &ec2.SubnetArgs{
			CidrBlock:           pulumi.String(subnetConfig.CidrBlock),
			VpcId:               vpc.ID(),
			AvailabilityZone:    pulumi.String(subnetConfig.Az),
			MapPublicIpOnLaunch: pulumi.Bool(subnetConfig.Public),
		})
		if err != nil {
			return err
		}
	}

	return nil
}
