resource "aws_db_subnet_group" "this" {
  name       = "${var.project}-db"
  subnet_ids = aws_subnet.public[*].id
  tags       = { Name = "${var.project}-db-subnet-group" }
}

# Only the EKS nodes/pods may reach the database. The cluster's primary security
# group is attached to managed nodes, so allowing it covers the pods.
resource "aws_security_group" "rds" {
  name        = "${var.project}-rds"
  description = "Allow Postgres from the EKS cluster"
  vpc_id      = aws_vpc.this.id

  ingress {
    description     = "PostgreSQL from EKS"
    from_port       = 5432
    to_port         = 5432
    protocol        = "tcp"
    security_groups = [aws_eks_cluster.this.vpc_config[0].cluster_security_group_id]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = { Name = "${var.project}-rds" }
}

resource "aws_db_instance" "this" {
  identifier     = "${var.project}-postgres"
  engine         = "postgres"
  engine_version = var.db_engine_version
  instance_class = var.db_instance_class

  allocated_storage = var.db_allocated_storage
  storage_type      = "gp3"
  storage_encrypted = true

  db_name  = var.db_name
  username = var.db_username
  password = var.db_password

  db_subnet_group_name   = aws_db_subnet_group.this.name
  vpc_security_group_ids = [aws_security_group.rds.id]
  publicly_accessible    = false
  multi_az               = false

  # Demo-friendly settings; harden for real use.
  skip_final_snapshot = true
  deletion_protection = false
  apply_immediately   = true
}
