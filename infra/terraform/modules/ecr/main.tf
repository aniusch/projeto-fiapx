resource "aws_ecr_repository" "this" {
  for_each = toset(var.repositories)

  name                 = "${var.project}/${each.value}"
  image_tag_mutability = "MUTABLE"
  force_delete         = true # allow destroy even with images present

  image_scanning_configuration {
    scan_on_push = true
  }
}
