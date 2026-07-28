resource "aws_db_subnet_group" "this" {
  name       = "${var.project}-db"
  subnet_ids = var.subnet_ids
  tags       = { Name = "${var.project}-db-subnet-group" }
}

resource "aws_security_group" "this" {
  name        = "${var.project}-rds"
  description = "Allow Postgres from the EKS cluster"
  vpc_id      = var.vpc_id

  ingress {
    description     = "PostgreSQL"
    from_port       = 5432
    to_port         = 5432
    protocol        = "tcp"
    security_groups = var.allowed_security_group_ids
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
  engine_version = var.engine_version
  instance_class = var.instance_class

  allocated_storage = var.allocated_storage
  storage_type      = "gp3"
  storage_encrypted = true

  db_name  = var.db_name
  username = var.username
  password = var.password

  db_subnet_group_name   = aws_db_subnet_group.this.name
  vpc_security_group_ids = [aws_security_group.this.id]
  publicly_accessible    = false
  multi_az               = false

  # Demo-friendly; harden for real use.
  skip_final_snapshot = true
  deletion_protection = false
  apply_immediately   = true
}
