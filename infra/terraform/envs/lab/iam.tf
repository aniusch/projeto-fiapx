# Learner Lab forbids creating IAM roles, so we reuse the pre-provisioned role
# (LabRole) for both the EKS control plane and the worker nodes. That is also why
# IRSA is not used — pods rely on the node role instead.
data "aws_iam_role" "lab" {
  name = var.lab_role_name
}

data "aws_caller_identity" "current" {}
