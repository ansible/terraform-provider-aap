# Due to https://github.com/hashicorp/terraform-plugin-framework/issues/853

go mod init terraform-provider-aap
go mod edit -require github.com/hashicorp/terraform-plugin-framework@v1.3.5
go mod edit -require github.com/hashicorp/terraform-plugin-go@v0.18.0
go mod tidy 
