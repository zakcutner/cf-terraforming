resource "cloudflare_snippet_rules" "terraform_managed_resource" {
  zone_id = "023e105f4ecef8ad9ca31a8372d0c353"
  rules = [{
    description = "Rule description"
    enabled = true
    expression = "http.cookie eq \"a=b\""
    snippet_name = "snippet_name_01"
  }]
}
