resource "cloudflare_workers_kv_namespace" "terraform_managed_resource" {
  account_id = "023e105f4ecef8ad9ca31a8372d0c353"
  title = "My Own Namespace"
}
