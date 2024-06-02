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

	config, err := loadConfig(configFile)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %v", err)
	}

	// vpc creation

	vpc, err := ec2.NewVpc(ctx, config.Vpc.Name, &ec2.VpcArgs{
		CidrBlock:          pulumi.String(config.Vpc.CidrBlock),
		EnableDnsSupport:   pulumi.Bool(true),
		EnableDnsHostnames: pulumi.Bool(true),
	})
	if err != nil {
		return err
	}
	// igw creation

	igw, err := ec2.NewInternetGateway(ctx, "internet-gateway", &ec2.InternetGatewayArgs{
		VpcId: vpc.ID(),
	})
	if err != nil {
		return err
	}

	// eip for natgateway

	eip, err := ec2.NewEip(ctx, "elasticip", &ec2.EipArgs{})
	if err != nil {
		return err
	}

	// subnet creation

	for i, subnetConfig := range config.Subnets {

		var subnetPrefix string

		if pulumi.Bool(subnetConfig.Public) {
			subnetPrefix = "subnet-public"
		} else {
			subnetPrefix = "subnet-private"
		}
		subnet, err := ec2.NewSubnet(ctx, fmt.Sprintf("%s-%d", subnetPrefix, i), &ec2.SubnetArgs{

			CidrBlock:           pulumi.String(subnetConfig.CidrBlock),
			VpcId:               vpc.ID(),
			AvailabilityZone:    pulumi.String(subnetConfig.Az),
			MapPublicIpOnLaunch: pulumi.Bool(subnetConfig.Public),
		})

		if err != nil {
			return err
		}
		ctx.Export(fmt.Sprintf("%s-%d", subnetPrefix, i), subnet.ID())

		if pulumi.Bool(subnetConfig.Public) {

			// nat gateway creation
			natGateway, err := ec2.NewNatGateway(ctx, "natgateway", &ec2.NatGatewayArgs{
				AllocationId: eip.ID(),
				SubnetId:     subnet.ID(),
			})
			if err != nil {
				return err
			}

			ctx.Export("NatGatewayID", natGateway.ID())

			publicRouteTable, err := ec2.NewRouteTable(ctx, fmt.Sprintf("%s-rt-%d", subnetPrefix, i), &ec2.RouteTableArgs{
				VpcId: vpc.ID(),
				Routes: ec2.RouteTableRouteArray{
					&ec2.RouteTableRouteArgs{
						CidrBlock: pulumi.String("0.0.0.0/0"),
						GatewayId: igw.ID(),
					},
				},
			})
			if err != nil {
				return err
			}

			_, err = ec2.NewRouteTableAssociation(ctx, fmt.Sprintf("%s-association-%d", subnetPrefix, i), &ec2.RouteTableAssociationArgs{
				SubnetId:     subnet.ID(),
				RouteTableId: publicRouteTable.ID(),
			})

			if err != nil {
				ctx.Log.Error("fatal error", nil)
				return err
			}

			ctx.Export("natGatewayID", natGateway.ID())

		} else {

			privateRouteTable, err := ec2.NewRouteTable(ctx, fmt.Sprintf("%s-rt-%d", subnetPrefix, i), &ec2.RouteTableArgs{
				VpcId: vpc.ID(),
				Routes: ec2.RouteTableRouteArray{
					&ec2.RouteTableRouteArgs{
						CidrBlock: pulumi.String("0.0.0.0/0"),
						//NatGatewayId: natGateway.ID(),
					},
				},
			})
			if err != nil {
				return err
			}

			_, err = ec2.NewRouteTableAssociation(ctx, fmt.Sprintf("%s-association-%d", subnetPrefix, i), &ec2.RouteTableAssociationArgs{
				SubnetId:     subnet.ID(),
				RouteTableId: privateRouteTable.ID(),
			})
			if err != nil {
				return err
			}
		}

	}
	return nil
}
