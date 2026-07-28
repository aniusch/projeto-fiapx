resource "aws_eks_node_group" "this" {
  cluster_name    = aws_eks_cluster.this.name
  node_group_name = "${var.project}-ng"
  node_role_arn   = data.aws_iam_role.lab.arn
  subnet_ids      = aws_subnet.public[*].id

  instance_types = [var.node_instance_type]
  capacity_type  = "ON_DEMAND" # Spot is often restricted in Learner Lab
  ami_type       = "AL2023_x86_64_STANDARD"
  disk_size      = var.node_disk_size

  scaling_config {
    desired_size = var.node_desired_size
    min_size     = var.node_min_size
    max_size     = var.node_max_size
  }

  update_config {
    max_unavailable = 1
  }

  # Let add-ons/cluster settle before creating nodes; ignore drift on desired
  # size so the cluster autoscaler / HPA-driven scaling isn't reverted by TF.
  lifecycle {
    ignore_changes = [scaling_config[0].desired_size]
  }
}
