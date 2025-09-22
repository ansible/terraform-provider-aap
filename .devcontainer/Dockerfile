FROM hashicorp/terraform:1.14.0-beta as terraform
FROM mcr.microsoft.com/devcontainers/go:1.24.5
COPY --from=terraform /bin/terraform /bin/terraform
