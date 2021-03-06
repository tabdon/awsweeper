package test

import (
	"fmt"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/cloudetc/awsweeper/command"
	res "github.com/cloudetc/awsweeper/resource"
	"github.com/hashicorp/terraform/helper/resource"
	"github.com/hashicorp/terraform/terraform"
	"github.com/spf13/afero"
)

func TestAccInstance_deleteByTags(t *testing.T) {
	var instance1, instance2 ec2.Instance

	resource.Test(t, resource.TestCase{
		PreCheck:  func() { testAccPreCheck(t) },
		Providers: testAccProviders,
		Steps: []resource.TestStep{
			{
				Config:             testAccInstanceConfig,
				ExpectNonEmptyPlan: true,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckInstanceExists("aws_instance.foo", &instance1),
					testAccCheckInstanceExists("aws_instance.bar", &instance2),
					testMainTags(argsDryRun, testAccInstanceAWSweeperTagsConfig),
					testInstanceExists(&instance1),
					testInstanceExists(&instance2),
					testMainTags(argsForceDelete, testAccInstanceAWSweeperTagsConfig),
					testInstanceDeleted(&instance1),
					testInstanceExists(&instance2),
				),
			},
		},
	})
}

func TestAccInstance_deleteByIds(t *testing.T) {
	var instance1, instance2 ec2.Instance

	resource.Test(t, resource.TestCase{
		PreCheck:  func() { testAccPreCheck(t) },
		Providers: testAccProviders,
		Steps: []resource.TestStep{
			{
				Config:             testAccInstanceConfig,
				ExpectNonEmptyPlan: true,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckInstanceExists("aws_instance.foo", &instance1),
					testAccCheckInstanceExists("aws_instance.bar", &instance2),
					testMainInstanceIds(argsDryRun, &instance1),
					testInstanceExists(&instance1),
					testInstanceExists(&instance2),
					testMainInstanceIds(argsForceDelete, &instance1),
					testInstanceDeleted(&instance1),
					testInstanceExists(&instance2),
				),
			},
		},
	})
}

func testAccCheckInstanceExists(n string, instance *ec2.Instance) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("No instance ID is set")
		}

		conn := client.EC2conn
		DescribeInstanceOpts := &ec2.DescribeInstancesInput{
			InstanceIds: []*string{aws.String(rs.Primary.ID)},
		}
		resp, err := conn.DescribeInstances(DescribeInstanceOpts)
		if err != nil {
			return err
		}
		if len(resp.Reservations) == 0 {
			return fmt.Errorf("Instance not found")
		}

		*instance = *resp.Reservations[0].Instances[0]

		return nil
	}
}

func testInstanceExists(instance *ec2.Instance) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		conn := client.EC2conn
		DescribeInstanceOpts := &ec2.DescribeInstancesInput{
			InstanceIds: []*string{instance.InstanceId},
		}
		resp, err := conn.DescribeInstances(DescribeInstanceOpts)
		if err != nil {
			return err
		}
		if len(resp.Reservations) == 0 {
			return fmt.Errorf("Instance has been deleted")
		}

		return nil
	}
}

func testInstanceDeleted(instance *ec2.Instance) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		conn := client.EC2conn
		DescribeInstanceOpts := &ec2.DescribeInstancesInput{
			InstanceIds: []*string{instance.InstanceId},
		}
		resp, err := conn.DescribeInstances(DescribeInstanceOpts)
		if err != nil {
			ec2err, ok := err.(awserr.Error)
			if !ok {
				return err
			}
			if ec2err.Code() == "InvalidInstanceID.NotFound" {
				return nil
			}

			return err
		}

		for _, r := range resp.Reservations {
			for _, i := range r.Instances {
				if i.State != nil && *i.State.Name != "terminated" {
					return fmt.Errorf("Found unterminated instance: %s", i)
				}
			}
		}

		return nil
	}
}

func testMainInstanceIds(args []string, instance *ec2.Instance) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		res.AppFs = afero.NewMemMapFs()
		afero.WriteFile(res.AppFs, "config.yml", []byte(testAccInstanceAWSweeperIdsConfig(instance)), 0644)
		os.Args = args

		command.WrappedMain()
		return nil
	}
}

const testAccInstanceConfig = `
resource "aws_instance" "foo" {
	ami = "${data.aws_ami.foo.id}"
	instance_type = "t2.micro"
	subnet_id = "${aws_subnet.foo.id}"

	tags {
		foo = "bar"
		Name = "awsweeper-testacc"
	}
}

resource "aws_instance" "bar" {
	ami = "${data.aws_ami.foo.id}"
	instance_type = "t2.micro"
	subnet_id = "${aws_subnet.foo.id}"

	tags {
		bar = "baz"
		Name = "awsweeper-testacc"
	}
}

resource "aws_vpc" "foo" {
	cidr_block = "10.1.0.0/16"

	tags {
		Name = "awsweeper-testacc"
	}
}

resource "aws_subnet" "foo" {
	vpc_id = "${aws_vpc.foo.id}"
	cidr_block = "10.1.0.1/24"

	tags {
		Name = "awsweeper-testacc"
	}
}

data "aws_ami" "foo" {
	most_recent = true
	owners = ["099720109477"]

	filter {
		name = "name"
		values = ["*ubuntu-trusty-14.04-amd64-server-*"]
	}

	filter {
		name = "state"
		values = ["available"]
	}

	filter {
		name = "virtualization-type"
		values = ["hvm"]
	}

	filter {
		name = "is-public"
		values = ["true"]
	}
}
`

const testAccInstanceAWSweeperTagsConfig = `
aws_instance:
  tags:
    foo: bar
`

func testAccInstanceAWSweeperIdsConfig(instance *ec2.Instance) string {
	id := instance.InstanceId

	return fmt.Sprintf(`
aws_instance:
  ids:
    - %s
`, *id)
}
