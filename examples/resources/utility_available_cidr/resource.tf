# The following is a basic example showing how to use the
# resource to find an available CIDR given a "from" range
# and a list of already used CIDRs
resource "utility_available_cidr" "example" {
  from_cidrs = ["10.0.0.0/16"]
  used_cidrs = ["10.0.0.0/20", "10.0.16.0/24"]
  mask       = 24
}

# value will be "10.0.17.0/24"
output "cidr" {
  value = utility_available_cidr.example.result
}


# How to use in AWS to find available subnet space in a VPC

# Fetch the VPC to get the VPC CIDR
data "aws_vpc" "example" {
  id = var.vpc_id
}
# Fetch the list of subnets in the VPC
data "aws_subnets" "example" {
  filter {
    name   = "vpc-id"
    values = [var.vpc_id]
  }
}
# Lookup all the subnets to get the CIDRs
data "aws_subnet" "example" {
  for_each = toset(data.aws_subnets.example.ids)
  id       = each.value
}
# Find the next available range
resource "utility_available_cidr" "aws" {
  from_cidrs = [data.aws_vpc.example.cidr_block]
  used_cidrs = [for subnet in data.aws_subnet.example : subnet.cidr_block]
  mask       = 24
}


# How to use in Azure to find available subnet space in a VNet

# Fetch the VNet
data "azurerm_virtual_network" "example" {
  name                = var.vnet_name
  resource_group_name = var.resource_group
}
# Fetch all the subnets in the VNet
data "azurerm_subnet" "example" {
  for_each             = toset(data.azurerm_virtual_network.example.subnets)
  name                 = each.key
  virtual_network_name = var.vnet_name
  resource_group_name  = var.resource_group
}
# Find the next available range
resource "utility_available_cidr" "cidr" {
  from_cidrs = data.azurerm_virtual_network.example.address_space
  used_cidrs = flatten([for subnet in data.azurerm_subnet.example : subnet.address_prefixes])
  mask       = 24
}


# How to use in GCP to find available subnet space in a Global Network

# Fetch the Global Network
data "google_compute_network" "example" {
  name = var.network_name
}
# Fetch all the subnets in the Global Network
data "google_compute_subnetwork" "example" {
  for_each  = toset(data.google_compute_network.example.subnetworks_self_links)
  self_link = each.key
}
# Find the next available range
resource "utility_available_cidr" "cidr" {
  # Since global networks don't have defined ranges, we list the whole private space.
  # This should be adjusted to fit your desired network space
  from_cidrs = ["10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"]
  used_cidrs = flatten([for subnet in data.google_compute_subnetwork.example : subnet.ip_cidr_range])
  mask       = 24
}