package terraform

// Template is a terraform configuration template
const Template = `
terraform {
	backend "s3" {
		bucket = "<% .ConfigBucket %>"
		key    = "<% .TFStatePath %>"
		region = "eu-west-1"
	}
}

variable "rds_instance_class" {
  type = "string"
	default = "<% .RDSInstanceClass %>"
}

variable "rds_instance_username" {
  type = "string"
	default = "<% .RDSUsername %>"
}

variable "rds_instance_password" {
  type = "string"
	default = "<% .RDSPassword %>"
}

variable "source_access_ip" {
  type = "string"
	default = "<% .SourceAccessIP %>"
}

variable "region" {
  type = "string"
	default = "<% .Region %>"
}

variable "availability_zone" {
  type = "string"
	default = "<% .AvailabilityZone %>"
}

variable "deployment" {
  type = "string"
	default = "<% .Deployment %>"
}

variable "rds_default_database_name" {
  type = "string"
	default = "<% .RDSDefaultDatabaseName %>"
}

variable "public_key" {
  type = "string"
	default = "<% .PublicKey %>"
}

variable "project" {
  type = "string"
	default = "<% .Project %>"
}

variable "multi_az_rds" {
  type = "string"
  default = <%if .MultiAZRDS %>true<%else%>false<%end%>
}

<%if .HostedZoneID %>
variable "hosted_zone_id" {
  type = "string"
  default = "<% .HostedZoneID %>"
}

variable "hosted_zone_record_prefix" {
  type = "string"
  default = "<% .HostedZoneRecordPrefix %>"
}
<%end%>

provider "aws" {
	region = "<% .Region %>"
}

resource "aws_key_pair" "default" {
	key_name_prefix = "${var.deployment}"
	public_key      = "${var.public_key}"
}

resource "aws_s3_bucket" "blobstore" {
  bucket        = "${var.deployment}-blobstore"
  force_destroy = true
  region = "<% .Region %>"

  tags {
    Name = "${var.deployment}"
    concourse-up-project = "${var.project}"
    concourse-up-component = "bosh"
  }
}

resource "aws_iam_user" "blobstore" {
  name = "${var.deployment}-blobstore"
}

resource "aws_iam_access_key" "blobstore" {
  user = "${var.deployment}-blobstore"
  depends_on = ["aws_iam_user.blobstore"]
}

resource "aws_iam_user_policy" "blobstore" {
  name = "${var.deployment}-blobstore"
  user = "${aws_iam_user.blobstore.name}"

  policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": [
        "s3:*"
      ],
      "Effect": "Allow",
      "Resource": [
        "arn:aws:s3:::${aws_s3_bucket.blobstore.id}",
        "arn:aws:s3:::${aws_s3_bucket.blobstore.id}/*"
      ]
    }
  ]
}
EOF
}

resource "aws_iam_user" "bosh" {
  name = "${var.deployment}-bosh"
}

resource "aws_iam_access_key" "bosh" {
  user = "${var.deployment}-bosh"
  depends_on = ["aws_iam_user.bosh"]
}

resource "aws_iam_user_policy" "bosh" {
  name = "${var.deployment}-bosh"
  user = "${aws_iam_user.bosh.name}"

  policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": [
        "ec2:*",
        "elasticloadbalancing:*"
      ],
      "Effect": "Allow",
      "Resource": "*"
    }
  ]
}
EOF
}

resource "aws_vpc" "default" {
  cidr_block = "10.0.0.0/16"

  tags {
    Name = "${var.deployment}"
    concourse-up-project = "${var.project}"
    concourse-up-component = "bosh"
  }
}

resource "aws_internet_gateway" "default" {
  vpc_id = "${aws_vpc.default.id}"

  tags {
    Name = "${var.deployment}"
    concourse-up-project = "${var.project}"
    concourse-up-component = "bosh"
  }
}

resource "aws_route" "internet_access" {
  route_table_id         = "${aws_vpc.default.main_route_table_id}"
  destination_cidr_block = "0.0.0.0/0"
  gateway_id             = "${aws_internet_gateway.default.id}"
}

resource "aws_subnet" "default" {
  vpc_id                  = "${aws_vpc.default.id}"
  availability_zone       = "${var.availability_zone}"
  cidr_block              = "10.0.0.0/24"
  map_public_ip_on_launch = true

  tags {
    Name = "${var.deployment}"
    concourse-up-project = "${var.project}"
    concourse-up-component = "bosh"
  }
}

resource "aws_elb" "concourse" {
  name            = "${var.deployment}"
  subnets         = ["${aws_subnet.default.id}"]
  security_groups = ["${aws_security_group.elb.id}"]

  listener {
    instance_port      = 8080
    instance_protocol  = "tcp"
    lb_port            = 80
    lb_protocol        = "tcp"
  }

  listener {
    instance_port      = 4443
    instance_protocol  = "tcp"
    lb_port            = 443
    lb_protocol        = "tcp"
  }

  listener {
    instance_port     = 2222
    instance_protocol = "tcp"
    lb_port           = 2222
    lb_protocol       = "tcp"
  }

  health_check {
    healthy_threshold   = 2
    unhealthy_threshold = 2
    timeout             = 3
    target              = "TCP:8080"
    interval            = 30
  }

  tags {
    Name = "${var.deployment}"
    concourse-up-project = "${var.project}"
    concourse-up-component = "bosh"
  }
}

<%if .HostedZoneID %>
resource "aws_route53_record" "concourse" {
  zone_id = "${var.hosted_zone_id}"
  name    = "${var.hosted_zone_record_prefix}"
  type    = "A"

  alias {
    name                   = "${aws_elb.concourse.dns_name}"
    zone_id                = "${aws_elb.concourse.zone_id}"
    evaluate_target_health = true
  }
}
<%end%>

resource "aws_eip" "director" {
  vpc = true
}

resource "aws_security_group" "director" {
  name        = "${var.deployment}-director"
  description = "Concourse UP Default BOSH security group"
  vpc_id      = "${aws_vpc.default.id}"

  tags {
    Name = "${var.deployment}-director"
    concourse-up-project = "${var.project}"
    concourse-up-component = "bosh"
  }

  ingress {
    from_port   = 22
    to_port     = 22
    protocol    = "tcp"
    cidr_blocks = ["${var.source_access_ip}/32"]
  }

  ingress {
    from_port   = 6868
    to_port     = 6868
    protocol    = "tcp"
    cidr_blocks = ["${var.source_access_ip}/32"]
  }

  ingress {
    from_port   = 25555
    to_port     = 25555
    protocol    = "tcp"
    cidr_blocks = ["${var.source_access_ip}/32"]
  }

  ingress {
    from_port = 0
    to_port   = 65535
    protocol  = "tcp"
    self      = true
  }

  ingress {
    from_port = 0
    to_port   = 65535
    protocol  = "udp"
    self      = true
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_security_group" "vms" {
  name        = "${var.deployment}-vms"
  description = "Concourse UP VMs security group"
  vpc_id      = "${aws_vpc.default.id}"

  tags {
    Name = "${var.deployment}-vms"
    concourse-up-project = "${var.project}"
    concourse-up-component = "bosh"
  }

  ingress {
    from_port   = 22
    to_port     = 22
    protocol    = "tcp"
    cidr_blocks = ["${var.source_access_ip}/32"]
  }

  ingress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["10.0.0.0/16"]
  }

  ingress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["10.0.0.0/16"]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_security_group" "rds" {
  name        = "${var.deployment}-rds"
  description = "Concourse UP RDS security group"
  vpc_id      = "${aws_vpc.default.id}"

  tags {
    Name = "${var.deployment}-rds"
    concourse-up-project = "${var.project}"
    concourse-up-component = "rds"
  }

  ingress {
    from_port   = 5432
    to_port     = 5432
    protocol    = "tcp"
    cidr_blocks = ["10.0.0.0/16"]
  }

  ingress {
    from_port   = 5432
    to_port     = 5432
    protocol    = "tcp"
    cidr_blocks = ["10.0.0.0/16"]
  }
}

resource "aws_security_group" "elb" {
  name        = "${var.deployment}-elb"
  description = "Concourse UP ELB security group"
  vpc_id      = "${aws_vpc.default.id}"

  tags {
    Name = "${var.deployment}-elb"
    concourse-up-project = "${var.project}"
    concourse-up-component = "concourse"
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  ingress {
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  ingress {
    from_port   = 443
    to_port     = 443
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  ingress {
    from_port   = 2222
    to_port     = 2222
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_route_table" "rds" {
  vpc_id = "${aws_vpc.default.id}"

  tags {
    Name = "${var.deployment}-rds"
    concourse-up-project = "${var.project}"
    concourse-up-component = "concourse"
  }
}

resource "aws_route_table_association" "rds_a" {
  subnet_id      = "${aws_subnet.rds_a.id}"
  route_table_id = "${aws_route_table.rds.id}"
}

resource "aws_route_table_association" "rds_b" {
  subnet_id      = "${aws_subnet.rds_b.id}"
  route_table_id = "${aws_route_table.rds.id}"
}

resource "aws_subnet" "rds_a" {
  vpc_id            = "${aws_vpc.default.id}"
  availability_zone = "${var.region}a"
  cidr_block        = "10.0.4.0/24"

  tags {
    Name = "${var.deployment}-rds-a"
    concourse-up-project = "${var.project}"
    concourse-up-component = "rds"
  }
}

resource "aws_subnet" "rds_b" {
  vpc_id            = "${aws_vpc.default.id}"
  availability_zone = "${var.region}b"
  cidr_block        = "10.0.5.0/24"

  tags {
    Name = "${var.deployment}-rds-b"
    concourse-up-project = "${var.project}"
    concourse-up-component = "rds"
  }
}

resource "aws_db_subnet_group" "default" {
  name       = "${var.deployment}"
  subnet_ids = ["${aws_subnet.rds_a.id}", "${aws_subnet.rds_b.id}"]

  tags {
    Name = "${var.deployment}"
    concourse-up-project = "${var.project}"
    concourse-up-component = "rds"
  }
}

resource "aws_db_instance" "default" {
  allocated_storage      = 10
  port                   = 5432
  engine                 = "postgres"
  instance_class         = "${var.rds_instance_class}"
  engine_version         = "9.6.1"
  name                   = "${var.rds_default_database_name}"
  username               = "${var.rds_instance_username}"
  password               = "${var.rds_instance_password}"
  publicly_accessible    = false
  multi_az               = "${var.multi_az_rds}"
  vpc_security_group_ids = ["${aws_security_group.rds.id}"]
  db_subnet_group_name   = "${aws_db_subnet_group.default.name}"
  skip_final_snapshot    = true

  tags {
    Name = "${var.deployment}"
    concourse-up-project = "${var.project}"
    concourse-up-component = "rds"
  }
}

output "source_access_ip" {
  value = "${var.source_access_ip}"
}

output "director_key_pair" {
  value = "${aws_key_pair.default.key_name}"
}

output "director_public_ip" {
  value = "${aws_eip.director.public_ip}"
}

output "director_security_group_id" {
  value = "${aws_security_group.director.id}"
}

output "vms_security_group_id" {
  value = "${aws_security_group.vms.id}"
}

output "elb_security_group_id" {
  value = "${aws_security_group.elb.id}"
}

output "default_subnet_id" {
  value = "${aws_subnet.default.id}"
}

output "blobstore_bucket" {
  value = "${aws_s3_bucket.blobstore.id}"
}

output "blobstore_user_access_key_id" {
  value = "${aws_iam_access_key.blobstore.id}"
}

output "blobstore_user_secret_access_key" {
  value = "${aws_iam_access_key.blobstore.secret}"
}

output "bosh_user_access_key_id" {
  value = "${aws_iam_access_key.bosh.id}"
}

output "bosh_user_secret_access_key" {
  value = "${aws_iam_access_key.bosh.secret}"
}

output "bosh_db_port" {
  value = "${aws_db_instance.default.port}"
}

output "bosh_db_address" {
  value = "${aws_db_instance.default.address}"
}

output "elb_name" {
  value = "${aws_elb.concourse.name}"
}

output "elb_dns_name" {
  value = "${aws_elb.concourse.dns_name}"
}
`
