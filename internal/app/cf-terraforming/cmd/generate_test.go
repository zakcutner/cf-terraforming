package cmd

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/MakeNowJust/heredoc/v2"

	cfv0 "github.com/cloudflare/cloudflare-go"
	"github.com/cloudflare/cloudflare-go/v4"
	"github.com/cloudflare/cloudflare-go/v4/option"

	"github.com/dnaeon/go-vcr/cassette"
	"github.com/dnaeon/go-vcr/recorder"

	"github.com/hashicorp/hcl/v2/hclwrite"

	"github.com/spf13/viper"

	"github.com/stretchr/testify/assert"
)

var (
	// listOfString is an example representation of a key where the value is a
	// list of string values.
	//
	//   resource "example" "example" {
	//     attr = [ "b", "c", "d"]
	//   }
	listOfString = []interface{}{"b", "c", "d"}

	// configBlockOfStrings is an example of where a key is a "block" assignment
	// in HCL.
	//
	//   resource "example" "example" {
	//     attr = {
	//       c = "d"
	//       e = "f"
	//     }
	//   }
	configBlockOfStrings = map[string]interface{}{
		"c": "d",
		"e": "f",
	}

	cloudflareTestZoneID    = "0da42c8d2132a9ddaf714f9e7c920711"
	cloudflareTestAccountID = "f037e56e89293a057740de681ac9abbe"
)

func TestGenerate_writeAttrLineV4(t *testing.T) {
	multilineListOfStrings := heredoc.Doc(`
		a = ["b", "c", "d"]
	`)
	multilineBlock := heredoc.Doc(`
		a = {
		  c = "d"
		  e = "f"
		}
	`)
	tests := map[string]struct {
		key   string
		value interface{}
		want  string
	}{
		"value is string":           {key: "a", value: "b", want: fmt.Sprintf("a = %q\n", "b")},
		"value is int":              {key: "a", value: 1, want: "a = 1\n"},
		"value is float":            {key: "a", value: 1.0, want: "a = 1\n"},
		"value is bool":             {key: "a", value: true, want: "a = true\n"},
		"value is list of strings":  {key: "a", value: listOfString, want: multilineListOfStrings},
		"value is block of strings": {key: "a", value: configBlockOfStrings, want: multilineBlock},
		"value is nil":              {key: "a", value: nil, want: ""},
	}

	for name, tc := range tests {
		f := hclwrite.NewEmptyFile()
		t.Run(name, func(t *testing.T) {
			writeAttrLine(tc.key, tc.value, "", f.Body())
			assert.Equal(t, tc.want, string(f.Bytes()))
		})
	}
}

func TestGenerate_ResourceNotSupportedV4(t *testing.T) {
	output, err := executeCommandC(rootCmd, "generate", "--resource-type", "notreal")
	assert.Nil(t, err)
	assert.Equal(t, `"notreal" is not yet supported for automatic generation`, output)
}

func TestResourceGenerationV4(t *testing.T) {
	tests := map[string]struct {
		identiferType    string
		resourceType     string
		testdataFilename string
	}{
		"cloudflare access application simple (account)":     {identiferType: "account", resourceType: "cloudflare_access_application", testdataFilename: "cloudflare_access_application_simple_account"},
		"cloudflare access application with CORS (account)":  {identiferType: "account", resourceType: "cloudflare_access_application", testdataFilename: "cloudflare_access_application_with_cors_account"},
		"cloudflare access IdP OAuth (account)":              {identiferType: "account", resourceType: "cloudflare_access_identity_provider", testdataFilename: "cloudflare_access_identity_provider_oauth_account"},
		"cloudflare access IdP OAuth (zone)":                 {identiferType: "zone", resourceType: "cloudflare_access_identity_provider", testdataFilename: "cloudflare_access_identity_provider_oauth_zone"},
		"cloudflare access IdP OTP (account)":                {identiferType: "account", resourceType: "cloudflare_access_identity_provider", testdataFilename: "cloudflare_access_identity_provider_otp_account"},
		"cloudflare access IdP OTP (zone)":                   {identiferType: "zone", resourceType: "cloudflare_access_identity_provider", testdataFilename: "cloudflare_access_identity_provider_otp_zone"},
		"cloudflare access rule (account)":                   {identiferType: "account", resourceType: "cloudflare_access_rule", testdataFilename: "cloudflare_access_rule_account"},
		"cloudflare access rule (zone)":                      {identiferType: "zone", resourceType: "cloudflare_access_rule", testdataFilename: "cloudflare_access_rule_zone"},
		"cloudflare account member":                          {identiferType: "account", resourceType: "cloudflare_account_member", testdataFilename: "cloudflare_account_member"},
		"cloudflare api shield":                              {identiferType: "zone", resourceType: "cloudflare_api_shield", testdataFilename: "cloudflare_api_shield"},
		"cloudflare argo":                                    {identiferType: "zone", resourceType: "cloudflare_argo", testdataFilename: "cloudflare_argo"},
		"cloudflare bot management":                          {identiferType: "zone", resourceType: "cloudflare_bot_management", testdataFilename: "cloudflare_bot_management"},
		"cloudflare BYO IP prefix":                           {identiferType: "account", resourceType: "cloudflare_byo_ip_prefix", testdataFilename: "cloudflare_byo_ip_prefix"},
		"cloudflare certificate pack":                        {identiferType: "zone", resourceType: "cloudflare_certificate_pack", testdataFilename: "cloudflare_certificate_pack_acm"},
		"cloudflare custom hostname fallback origin":         {identiferType: "zone", resourceType: "cloudflare_custom_hostname_fallback_origin", testdataFilename: "cloudflare_custom_hostname_fallback_origin"},
		"cloudflare custom hostname":                         {identiferType: "zone", resourceType: "cloudflare_custom_hostname", testdataFilename: "cloudflare_custom_hostname"},
		"cloudflare custom pages (account)":                  {identiferType: "account", resourceType: "cloudflare_custom_pages", testdataFilename: "cloudflare_custom_pages_account"},
		"cloudflare custom pages (zone)":                     {identiferType: "zone", resourceType: "cloudflare_custom_pages", testdataFilename: "cloudflare_custom_pages_zone"},
		"cloudflare filter":                                  {identiferType: "zone", resourceType: "cloudflare_filter", testdataFilename: "cloudflare_filter"},
		"cloudflare firewall rule":                           {identiferType: "zone", resourceType: "cloudflare_firewall_rule", testdataFilename: "cloudflare_firewall_rule"},
		"cloudflare health check":                            {identiferType: "zone", resourceType: "cloudflare_healthcheck", testdataFilename: "cloudflare_healthcheck"},
		"cloudflare list (asn)":                              {identiferType: "account", resourceType: "cloudflare_list", testdataFilename: "cloudflare_list_asn"},
		"cloudflare list (hostname)":                         {identiferType: "account", resourceType: "cloudflare_list", testdataFilename: "cloudflare_list_hostname"},
		"cloudflare list (ip)":                               {identiferType: "account", resourceType: "cloudflare_list", testdataFilename: "cloudflare_list_ip"},
		"cloudflare list (redirect)":                         {identiferType: "account", resourceType: "cloudflare_list", testdataFilename: "cloudflare_list_redirect"},
		"cloudflare load balancer monitor":                   {identiferType: "account", resourceType: "cloudflare_load_balancer_monitor", testdataFilename: "cloudflare_load_balancer_monitor"},
		"cloudflare load balancer pool":                      {identiferType: "account", resourceType: "cloudflare_load_balancer_pool", testdataFilename: "cloudflare_load_balancer_pool"},
		"cloudflare load balancer":                           {identiferType: "zone", resourceType: "cloudflare_load_balancer", testdataFilename: "cloudflare_load_balancer"},
		"cloudflare logpush jobs with filter":                {identiferType: "zone", resourceType: "cloudflare_logpush_job", testdataFilename: "cloudflare_logpush_job_with_filter"},
		"cloudflare logpush jobs":                            {identiferType: "zone", resourceType: "cloudflare_logpush_job", testdataFilename: "cloudflare_logpush_job"},
		"cloudflare managed headers":                         {identiferType: "zone", resourceType: "cloudflare_managed_headers", testdataFilename: "cloudflare_managed_headers"},
		"cloudflare origin CA certificate":                   {identiferType: "zone", resourceType: "cloudflare_origin_ca_certificate", testdataFilename: "cloudflare_origin_ca_certificate"},
		"cloudflare page rule":                               {identiferType: "zone", resourceType: "cloudflare_page_rule", testdataFilename: "cloudflare_page_rule"},
		"cloudflare rate limit":                              {identiferType: "zone", resourceType: "cloudflare_rate_limit", testdataFilename: "cloudflare_rate_limit"},
		"cloudflare record CAA":                              {identiferType: "zone", resourceType: "cloudflare_record", testdataFilename: "cloudflare_record_caa"},
		"cloudflare record PTR":                              {identiferType: "zone", resourceType: "cloudflare_record", testdataFilename: "cloudflare_record_ptr"},
		"cloudflare record simple":                           {identiferType: "zone", resourceType: "cloudflare_record", testdataFilename: "cloudflare_record"},
		"cloudflare record subdomain":                        {identiferType: "zone", resourceType: "cloudflare_record", testdataFilename: "cloudflare_record_subdomain"},
		"cloudflare record TXT SPF":                          {identiferType: "zone", resourceType: "cloudflare_record", testdataFilename: "cloudflare_record_txt_spf"},
		"cloudflare ruleset (ddos_l7)":                       {identiferType: "zone", resourceType: "cloudflare_ruleset", testdataFilename: "cloudflare_ruleset_zone_ddos_l7"},
		"cloudflare ruleset (http_log_custom_fields)":        {identiferType: "zone", resourceType: "cloudflare_ruleset", testdataFilename: "cloudflare_ruleset_zone_http_log_custom_fields"},
		"cloudflare ruleset (http_ratelimit)":                {identiferType: "zone", resourceType: "cloudflare_ruleset", testdataFilename: "cloudflare_ruleset_zone_http_ratelimit"},
		"cloudflare ruleset (http_request_cache_settings)":   {identiferType: "zone", resourceType: "cloudflare_ruleset", testdataFilename: "cloudflare_ruleset_http_request_cache_settings"},
		"cloudflare ruleset (http_request_firewall_custom)":  {identiferType: "zone", resourceType: "cloudflare_ruleset", testdataFilename: "cloudflare_ruleset_zone_http_request_firewall_custom"},
		"cloudflare ruleset (http_request_firewall_managed)": {identiferType: "zone", resourceType: "cloudflare_ruleset", testdataFilename: "cloudflare_ruleset_zone_http_request_firewall_managed"},
		"cloudflare ruleset (http_request_late_transform)":   {identiferType: "zone", resourceType: "cloudflare_ruleset", testdataFilename: "cloudflare_ruleset_zone_http_request_late_transform"},
		"cloudflare ruleset (http_request_sanitize)":         {identiferType: "zone", resourceType: "cloudflare_ruleset", testdataFilename: "cloudflare_ruleset_zone_http_request_sanitize"},
		"cloudflare ruleset (no configuration)":              {identiferType: "zone", resourceType: "cloudflare_ruleset", testdataFilename: "cloudflare_ruleset_zone_no_configuration"},
		"cloudflare ruleset (override remapping = disabled)": {identiferType: "zone", resourceType: "cloudflare_ruleset", testdataFilename: "cloudflare_ruleset_override_remapping_disabled"},
		"cloudflare ruleset (override remapping = enabled)":  {identiferType: "zone", resourceType: "cloudflare_ruleset", testdataFilename: "cloudflare_ruleset_override_remapping_enabled"},
		"cloudflare ruleset (rewrite to empty query string)": {identiferType: "zone", resourceType: "cloudflare_ruleset", testdataFilename: "cloudflare_ruleset_zone_rewrite_to_empty_query_parameter"},
		"cloudflare ruleset":                                 {identiferType: "zone", resourceType: "cloudflare_ruleset", testdataFilename: "cloudflare_ruleset_zone"},
		"cloudflare spectrum application":                    {identiferType: "zone", resourceType: "cloudflare_spectrum_application", testdataFilename: "cloudflare_spectrum_application"},
		"cloudflare teams list":                              {identiferType: "account", resourceType: "cloudflare_teams_list", testdataFilename: "cloudflare_teams_list"},
		"cloudflare teams location":                          {identiferType: "account", resourceType: "cloudflare_teams_location", testdataFilename: "cloudflare_teams_location"},
		"cloudflare teams proxy endpoint":                    {identiferType: "account", resourceType: "cloudflare_teams_proxy_endpoint", testdataFilename: "cloudflare_teams_proxy_endpoint"},
		"cloudflare teams rule":                              {identiferType: "account", resourceType: "cloudflare_teams_rule", testdataFilename: "cloudflare_teams_rule"},
		"cloudflare tunnel":                                  {identiferType: "account", resourceType: "cloudflare_tunnel", testdataFilename: "cloudflare_tunnel"},
		"cloudflare turnstile_widget":                        {identiferType: "account", resourceType: "cloudflare_turnstile_widget", testdataFilename: "cloudflare_turnstile_widget"},
		"cloudflare turnstile_widget_no_domains":             {identiferType: "account", resourceType: "cloudflare_turnstile_widget", testdataFilename: "cloudflare_turnstile_widget_no_domains"},
		"cloudflare url normalization settings":              {identiferType: "zone", resourceType: "cloudflare_url_normalization_settings", testdataFilename: "cloudflare_url_normalization_settings"},
		"cloudflare user agent blocking rule":                {identiferType: "zone", resourceType: "cloudflare_user_agent_blocking_rule", testdataFilename: "cloudflare_user_agent_blocking_rule"},
		"cloudflare waiting room":                            {identiferType: "zone", resourceType: "cloudflare_waiting_room", testdataFilename: "cloudflare_waiting_room"},
		"cloudflare waiting room event":                      {identiferType: "zone", resourceType: "cloudflare_waiting_room_event", testdataFilename: "cloudflare_waiting_room_event"},
		"cloudflare waiting room rules":                      {identiferType: "zone", resourceType: "cloudflare_waiting_room_rules", testdataFilename: "cloudflare_waiting_room_rules"},
		"cloudflare waiting room settings":                   {identiferType: "zone", resourceType: "cloudflare_waiting_room_settings", testdataFilename: "cloudflare_waiting_room_settings"},
		"cloudflare worker route":                            {identiferType: "zone", resourceType: "cloudflare_worker_route", testdataFilename: "cloudflare_worker_route"},
		"cloudflare workers kv namespace":                    {identiferType: "account", resourceType: "cloudflare_workers_kv_namespace", testdataFilename: "cloudflare_workers_kv_namespace"},
		"cloudflare zone lockdown":                           {identiferType: "zone", resourceType: "cloudflare_zone_lockdown", testdataFilename: "cloudflare_zone_lockdown"},
		"cloudflare zone settings override":                  {identiferType: "zone", resourceType: "cloudflare_zone_settings_override", testdataFilename: "cloudflare_zone_settings_override"},
		"cloudflare tiered cache":                            {identiferType: "zone", resourceType: "cloudflare_tiered_cache", testdataFilename: "cloudflare_tiered_cache"},

		// "cloudflare access group (account)": {identiferType: "account", resourceType: "cloudflare_access_group", testdataFilename: "cloudflare_access_group_account"},
		// "cloudflare access group (zone)":    {identiferType: "zone", resourceType: "cloudflare_access_group", testdataFilename: "cloudflare_access_group_zone"},
		// "cloudflare custom certificates":    {identiferType: "zone", resourceType: "cloudflare_custom_certificates", testdataFilename: "cloudflare_custom_certificates"},
		// "cloudflare custom SSL":             {identiferType: "zone", resourceType: "cloudflare_custom_ssl", testdataFilename: "cloudflare_custom_ssl"},
		// "cloudflare load balancer pool":     {identiferType: "account", resourceType: "cloudflare_load_balancer_pool", testdataFilename: "cloudflare_load_balancer_pool"},
		// "cloudflare worker cron trigger":    {identiferType: "zone", resourceType: "cloudflare_worker_cron_trigger", testdataFilename: "cloudflare_worker_cron_trigger"},
		// "cloudflare zone":                   {identiferType: "zone", resourceType: "cloudflare_zone", testdataFilename: "cloudflare_zone"},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Reset the environment variables used in test to ensure we don't
			// have both present at once.
			viper.Set("zone", "")
			viper.Set("account", "")

			var r *recorder.Recorder
			var err error
			if os.Getenv("OVERWRITE_VCR_CASSETTES") == "true" {
				r, err = recorder.NewAsMode("../../../../testdata/cloudflare/v4/"+tc.testdataFilename, recorder.ModeRecording, http.DefaultTransport)
			} else {
				r, err = recorder.New("../../../../testdata/cloudflare/v4/" + tc.testdataFilename)
			}

			if err != nil {
				log.Fatal(err)
			}
			defer func() {
				err := r.Stop()
				if err != nil {
					log.Fatal(err)
				}
			}()

			r.AddFilter(func(i *cassette.Interaction) error {
				// Sensitive HTTP headers
				delete(i.Request.Headers, "X-Auth-Email")
				delete(i.Request.Headers, "X-Auth-Key")
				delete(i.Request.Headers, "Authorization")

				// HTTP request headers that we don't need to assert against
				delete(i.Request.Headers, "User-Agent")

				// HTTP response headers that we don't need to assert against
				delete(i.Response.Headers, "Cf-Cache-Status")
				delete(i.Response.Headers, "Cf-Ray")
				delete(i.Response.Headers, "Date")
				delete(i.Response.Headers, "Server")
				delete(i.Response.Headers, "Set-Cookie")
				delete(i.Response.Headers, "X-Envoy-Upstream-Service-Time")

				if os.Getenv("CLOUDFLARE_DOMAIN") != "" {
					i.Response.Body = strings.ReplaceAll(i.Response.Body, os.Getenv("CLOUDFLARE_DOMAIN"), "example.com")
				}

				return nil
			})

			apiV0, _ = cfv0.New(viper.GetString("key"), viper.GetString("email"), cfv0.HTTPClient(
				&http.Client{
					Transport: r,
				},
			))
			output := ""
			if tc.identiferType == "account" {
				viper.Set("account", cloudflareTestAccountID)
				output, _ = executeCommandC(rootCmd, "generate", "--resource-type", tc.resourceType, "--account", cloudflareTestAccountID)
			} else {
				viper.Set("zone", cloudflareTestZoneID)
				output, _ = executeCommandC(rootCmd, "generate", "--resource-type", tc.resourceType, "--zone", cloudflareTestZoneID)
			}

			expected := testDataFile("v4", tc.testdataFilename)
			assert.Equal(t, strings.TrimRight(expected, "\n"), strings.TrimRight(output, "\n"))
		})
	}
}

func TestResourceGenerationV5(t *testing.T) {
	tests := map[string]struct {
		identiferType    string
		resourceType     string
		testdataFilename string
		cliFlags         string
	}{
		// "cloudflare access application simple (account)":     {identiferType: "account", resourceType: "cloudflare_access_application", testdataFilename: "cloudflare_access_application_simple_account"},
		// "cloudflare access application with CORS (account)":  {identiferType: "account", resourceType: "cloudflare_access_application", testdataFilename: "cloudflare_access_application_with_cors_account"},
		// "cloudflare access IdP OAuth (account)":              {identiferType: "account", resourceType: "cloudflare_access_identity_provider", testdataFilename: "cloudflare_access_identity_provider_oauth_account"},
		// "cloudflare access IdP OAuth (zone)":                 {identiferType: "zone", resourceType: "cloudflare_access_identity_provider", testdataFilename: "cloudflare_access_identity_provider_oauth_zone"},
		// "cloudflare access IdP OTP (account)":                {identiferType: "account", resourceType: "cloudflare_access_identity_provider", testdataFilename: "cloudflare_access_identity_provider_otp_account"},
		// "cloudflare access IdP OTP (zone)":                   {identiferType: "zone", resourceType: "cloudflare_access_identity_provider", testdataFilename: "cloudflare_access_identity_provider_otp_zone"},
		// "cloudflare access rule (account)":                   {identiferType: "account", resourceType: "cloudflare_access_rule", testdataFilename: "cloudflare_access_rule_account"},
		"cloudflare account": {identiferType: "account", resourceType: "cloudflare_account", testdataFilename: "cloudflare_account"},
		// "cloudflare access rule (zone)":                      {identiferType: "zone", resourceType: "cloudflare_access_rule", testdataFilename: "cloudflare_access_rule_zone"},
		"cloudflare account subscription":                            {identiferType: "account", resourceType: "cloudflare_account_subscription", testdataFilename: "cloudflare_account_subscription"},
		"cloudflare address map":                                     {identiferType: "account", resourceType: "cloudflare_address_map", testdataFilename: "cloudflare_address_map"},
		"cloudflare account member":                                  {identiferType: "account", resourceType: "cloudflare_account_member", testdataFilename: "cloudflare_account_member"},
		"cloudflare api shield schema":                               {identiferType: "zone", resourceType: "cloudflare_api_shield_schema", testdataFilename: "cloudflare_api_shield_schema"},
		"cloudflare api shield discovery operation":                  {identiferType: "zone", resourceType: "cloudflare_api_shield_discovery_operation", testdataFilename: "cloudflare_api_shield_discovery_operation"},
		"cloudflare api shield operation":                            {identiferType: "zone", resourceType: "cloudflare_api_shield_operation", testdataFilename: "cloudflare_api_shield_operation"},
		"cloudflare api shield schema validation settings":           {identiferType: "zone", resourceType: "cloudflare_api_shield_schema_validation_settings", testdataFilename: "cloudflare_api_shield_schema_validation_settings"},
		"cloudflare api shield operation schema validation settings": {identiferType: "zone", resourceType: "cloudflare_api_shield_operation_schema_validation_settings", testdataFilename: "cloudflare_api_shield_operation_schema_validation_settings", cliFlags: "cloudflare_api_shield_operation_schema_validation_settings=8255d5da-5a46-4928-ad00-01de7d48c1e7"},
		"cloudflare argo tiered caching":                             {identiferType: "zone", resourceType: "cloudflare_argo_tiered_caching", testdataFilename: "cloudflare_argo_tiered_caching"},
		"cloudflare argo smart routing":                              {identiferType: "zone", resourceType: "cloudflare_argo_smart_routing", testdataFilename: "cloudflare_argo_smart_routing"},
		"cloudflare authenticated origin_pulls":                      {identiferType: "zone", resourceType: "cloudflare_authenticated_origin_pulls", testdataFilename: "cloudflare_authenticated_origin_pulls", cliFlags: "cloudflare_authenticated_origin_pulls=jotsqcjaho.terraform.cfapi.net"},
		"cloudflare authenticated origin pulls certificate":          {identiferType: "zone", resourceType: "cloudflare_authenticated_origin_pulls_certificate", testdataFilename: "cloudflare_authenticated_origin_pulls_certificate"},
		"cloudflare bot management":                                  {identiferType: "zone", resourceType: "cloudflare_bot_management", testdataFilename: "cloudflare_bot_management"},
		"cloudflare calls sfu app":                                   {identiferType: "account", resourceType: "cloudflare_calls_sfu_app", testdataFilename: "cloudflare_calls_sfu_app"},
		"cloudflare calls turn_app":                                  {identiferType: "account", resourceType: "cloudflare_calls_turn_app", testdataFilename: "cloudflare_calls_turn_app"},
		// "cloudflare argo":                                    {identiferType: "zone", resourceType: "cloudflare_argo", testdataFilename: "cloudflare_argo"},
		// "cloudflare BYO IP prefix":                           {identiferType: "account", resourceType: "cloudflare_byo_ip_prefix", testdataFilename: "cloudflare_byo_ip_prefix"},
		"cloudflare certificate pack":                {identiferType: "zone", resourceType: "cloudflare_certificate_pack", testdataFilename: "cloudflare_certificate_pack"},
		"cloudflare content scanning expression":     {identiferType: "zone", resourceType: "cloudflare_content_scanning_expression", testdataFilename: "cloudflare_content_scanning_expression"},
		"cloudflare custom hostname fallback origin": {identiferType: "zone", resourceType: "cloudflare_custom_hostname_fallback_origin", testdataFilename: "cloudflare_custom_hostname_fallback_origin"},
		"cloudflare custom hostname":                 {identiferType: "zone", resourceType: "cloudflare_custom_hostname", testdataFilename: "cloudflare_custom_hostname"},
		// "cloudflare custom pages (account)":                  {identiferType: "account", resourceType: "cloudflare_custom_pages", testdataFilename: "cloudflare_custom_pages_account"},
		// "cloudflare custom pages (zone)":                     {identiferType: "zone", resourceType: "cloudflare_custom_pages", testdataFilename: "cloudflare_custom_pages_zone"},
		"cloudflare email routing address":                 {identiferType: "account", resourceType: "cloudflare_email_routing_address", testdataFilename: "cloudflare_email_routing_address"},
		"cloudflare email routing catch all":               {identiferType: "zone", resourceType: "cloudflare_email_routing_catch_all", testdataFilename: "cloudflare_email_routing_catch_all"},
		"cloudflare email routing dns":                     {identiferType: "zone", resourceType: "cloudflare_email_routing_dns", testdataFilename: "cloudflare_email_routing_dns"},
		"cloudflare email routing rule":                    {identiferType: "zone", resourceType: "cloudflare_email_routing_rule", testdataFilename: "cloudflare_email_routing_rule"},
		"cloudflare email routing settings":                {identiferType: "zone", resourceType: "cloudflare_email_routing_settings", testdataFilename: "cloudflare_email_routing_settings"},
		"cloudflare email security block sender":           {identiferType: "account", resourceType: "cloudflare_email_security_block_sender", testdataFilename: "cloudflare_email_security_block_sender"},
		"cloudflare email security trusted domains":        {identiferType: "account", resourceType: "cloudflare_email_security_trusted_domains", testdataFilename: "cloudflare_email_security_trusted_domains"},
		"cloudflare email security impersonation registry": {identiferType: "account", resourceType: "cloudflare_email_security_impersonation_registry", testdataFilename: "cloudflare_email_security_impersonation_registry"},
		"cloudflare filter":                                {identiferType: "zone", resourceType: "cloudflare_filter", testdataFilename: "cloudflare_filter"},
		// "cloudflare firewall rule":                           {identiferType: "zone", resourceType: "cloudflare_firewall_rule", testdataFilename: "cloudflare_firewall_rule"},
		"cloudflare health check":          {identiferType: "zone", resourceType: "cloudflare_healthcheck", testdataFilename: "cloudflare_healthcheck"},
		"cloudflare hostname tls setting":  {identiferType: "zone", resourceType: "cloudflare_hostname_tls_setting", testdataFilename: "cloudflare_hostname_tls_setting", cliFlags: "cloudflare_hostname_tls_setting=ciphers,min_tls_version"},
		"cloudflare keyless certificate":   {identiferType: "zone", resourceType: "cloudflare_keyless_certificate", testdataFilename: "cloudflare_keyless_certificate"},
		"cloudflare mtls certificate":      {identiferType: "account", resourceType: "cloudflare_mtls_certificate", testdataFilename: "cloudflare_mtls_certificate"},
		"cloudflare load balancer":         {identiferType: "zone", resourceType: "cloudflare_load_balancer", testdataFilename: "cloudflare_load_balancer"},
		"cloudflare load balancer monitor": {identiferType: "account", resourceType: "cloudflare_load_balancer_monitor", testdataFilename: "cloudflare_load_balancer_monitor"},
		"cloudflare load balancer pool":    {identiferType: "account", resourceType: "cloudflare_load_balancer_pool", testdataFilename: "cloudflare_load_balancer_pool"},
		// "cloudflare logpush jobs with filter":                {identiferType: "zone", resourceType: "cloudflare_logpush_job", testdataFilename: "cloudflare_logpush_job_with_filter"},
		"cloudflare managed transforms":    {identiferType: "zone", resourceType: "cloudflare_managed_transforms", testdataFilename: "cloudflare_managed_transforms"},
		"cloudflare origin ca certificate": {identiferType: "zone", resourceType: "cloudflare_origin_ca_certificate", testdataFilename: "cloudflare_origin_ca_certificate"},
		"cloudflare d1 database":           {identiferType: "account", resourceType: "cloudflare_d1_database", testdataFilename: "cloudflare_d1_database"},
		"cloudflare dns firewall":          {identiferType: "account", resourceType: "cloudflare_dns_firewall", testdataFilename: "cloudflare_dns_firewall"},
		// "cloudflare dns record CAA":                          {identiferType: "zone", resourceType: "cloudflare_dns_record", testdataFilename: "cloudflare_dns_record_caa"},
		// "cloudflare dns record PTR":                          {identiferType: "zone", resourceType: "cloudflare_dns_record", testdataFilename: "cloudflare_dns_record_ptr"},
		"cloudflare dns record simple": {identiferType: "zone", resourceType: "cloudflare_dns_record", testdataFilename: "cloudflare_dns_record"},
		// "cloudflare dns record subdomain":                    {identiferType: "zone", resourceType: "cloudflare_dns_record", testdataFilename: "cloudflare_dns_record_subdomain"},
		// "cloudflare dns record TXT SPF":                      {identiferType: "zone", resourceType: "cloudflare_dns_record", testdataFilename: "cloudflare_dns_record_txt_spf"},
		"cloudflare dns zone transfers acl":                  {identiferType: "account", resourceType: "cloudflare_dns_zone_transfers_acl", testdataFilename: "cloudflare_dns_zone_transfers_acl"},
		"cloudflare dns zone transfers incoming":             {identiferType: "zone", resourceType: "cloudflare_dns_zone_transfers_incoming", testdataFilename: "cloudflare_dns_zone_transfers_incoming"},
		"cloudflare dns zone transfers outgoing":             {identiferType: "zone", resourceType: "cloudflare_dns_zone_transfers_outgoing", testdataFilename: "cloudflare_dns_zone_transfers_outgoing"},
		"cloudflare dns zone transfers peer":                 {identiferType: "account", resourceType: "cloudflare_dns_zone_transfers_peer", testdataFilename: "cloudflare_dns_zone_transfers_peer"},
		"cloudflare dns zone transfers tsig":                 {identiferType: "account", resourceType: "cloudflare_dns_zone_transfers_tsig", testdataFilename: "cloudflare_dns_zone_transfers_tsig"},
		"cloudflare leaked credential check":                 {identiferType: "zone", resourceType: "cloudflare_leaked_credential_check", testdataFilename: "cloudflare_leaked_credential_check"},
		"cloudflare leaked credential check rule":            {identiferType: "zone", resourceType: "cloudflare_leaked_credential_check_rule", testdataFilename: "cloudflare_leaked_credential_check_rule"},
		"cloudflare list":                                    {identiferType: "account", resourceType: "cloudflare_list", testdataFilename: "cloudflare_list"},
		"cloudflare list item":                               {identiferType: "account", resourceType: "cloudflare_list_item", testdataFilename: "cloudflare_list_item", cliFlags: "cloudflare_list_item=2a4b8b2017aa4b3cb9e1151b52c81d22"},
		"cloudflare logpush job":                             {identiferType: "account", resourceType: "cloudflare_logpush_job", testdataFilename: "cloudflare_logpush_job"},
		"cloudflare logpull retention":                       {identiferType: "zone", resourceType: "cloudflare_logpull_retention", testdataFilename: "cloudflare_logpull_retention"},
		"cloudflare magic wan static route":                  {identiferType: "account", resourceType: "cloudflare_magic_wan_static_route", testdataFilename: "cloudflare_magic_wan_static_route"},
		"cloudflare notification policy":                     {identiferType: "account", resourceType: "cloudflare_notification_policy", testdataFilename: "cloudflare_notification_policy"},
		"cloudflare notification policy webhooks":            {identiferType: "account", resourceType: "cloudflare_notification_policy_webhooks", testdataFilename: "cloudflare_notification_policy_webhooks"},
		"cloudflare observatory scheduled test":              {identiferType: "zone", resourceType: "cloudflare_observatory_scheduled_test", testdataFilename: "cloudflare_observatory_scheduled_test", cliFlags: "cloudflare_observatory_scheduled_test=terraform.cfapi.net/thyygxveip"},
		"cloudflare pages domain":                            {identiferType: "account", resourceType: "cloudflare_pages_domain", testdataFilename: "cloudflare_pages_domain", cliFlags: "cloudflare_pages_domain=ykfjmcgpfs"},
		"cloudflare pages project":                           {identiferType: "account", resourceType: "cloudflare_pages_project", testdataFilename: "cloudflare_pages_project"},
		"cloudflare page shield policy":                      {identiferType: "zone", resourceType: "cloudflare_page_shield_policy", testdataFilename: "cloudflare_page_shield_policy"},
		"cloudflare registrar domain":                        {identiferType: "account", resourceType: "cloudflare_registrar_domain", testdataFilename: "cloudflare_registrar_domain"},
		"cloudflare rate limit":                              {identiferType: "zone", resourceType: "cloudflare_rate_limit", testdataFilename: "cloudflare_rate_limit"},
		"cloudflare r2 bucket":                               {identiferType: "account", resourceType: "cloudflare_r2_bucket", testdataFilename: "cloudflare_r2_bucket"},
		"cloudflare r2 managed domain":                       {identiferType: "account", resourceType: "cloudflare_r2_managed_domain", testdataFilename: "cloudflare_r2_managed_domain", cliFlags: "cloudflare_r2_managed_domain=jb-test-bucket,bnfywlzwpt"},
		"cloudflare r2 custom domain":                        {identiferType: "account", resourceType: "cloudflare_r2_custom_domain", testdataFilename: "cloudflare_r2_custom_domain", cliFlags: "cloudflare_r2_custom_domain=jb-test-bucket,bnfywlzwpt"},
		"cloudflare page rule":                               {identiferType: "zone", resourceType: "cloudflare_page_rule", testdataFilename: "cloudflare_page_rule"},
		"cloudflare ruleset (ddos_l7)":                       {identiferType: "zone", resourceType: "cloudflare_ruleset", testdataFilename: "cloudflare_ruleset_zone_ddos_l7"},
		"cloudflare ruleset (http_log_custom_fields)":        {identiferType: "zone", resourceType: "cloudflare_ruleset", testdataFilename: "cloudflare_ruleset_zone_http_log_custom_fields"},
		"cloudflare ruleset (http_ratelimit)":                {identiferType: "zone", resourceType: "cloudflare_ruleset", testdataFilename: "cloudflare_ruleset_zone_http_ratelimit"},
		"cloudflare ruleset (http_request_cache_settings)":   {identiferType: "zone", resourceType: "cloudflare_ruleset", testdataFilename: "cloudflare_ruleset_http_request_cache_settings"},
		"cloudflare ruleset (http_request_firewall_custom)":  {identiferType: "zone", resourceType: "cloudflare_ruleset", testdataFilename: "cloudflare_ruleset_zone_http_request_firewall_custom"},
		"cloudflare ruleset (http_request_firewall_managed)": {identiferType: "zone", resourceType: "cloudflare_ruleset", testdataFilename: "cloudflare_ruleset_zone_http_request_firewall_managed"},
		"cloudflare ruleset (http_request_late_transform)":   {identiferType: "zone", resourceType: "cloudflare_ruleset", testdataFilename: "cloudflare_ruleset_zone_http_request_late_transform"},
		"cloudflare ruleset (http_request_sanitize)":         {identiferType: "zone", resourceType: "cloudflare_ruleset", testdataFilename: "cloudflare_ruleset_zone_http_request_sanitize"},
		"cloudflare ruleset (no configuration)":              {identiferType: "zone", resourceType: "cloudflare_ruleset", testdataFilename: "cloudflare_ruleset_zone_no_configuration"},
		"cloudflare ruleset (override remapping = disabled)": {identiferType: "zone", resourceType: "cloudflare_ruleset", testdataFilename: "cloudflare_ruleset_override_remapping_disabled"},
		"cloudflare ruleset (override remapping = enabled)":  {identiferType: "zone", resourceType: "cloudflare_ruleset", testdataFilename: "cloudflare_ruleset_override_remapping_enabled"},
		"cloudflare ruleset (rewrite to empty query string)": {identiferType: "zone", resourceType: "cloudflare_ruleset", testdataFilename: "cloudflare_ruleset_zone_rewrite_to_empty_query_parameter"},
		"cloudflare ruleset":                                 {identiferType: "zone", resourceType: "cloudflare_ruleset", testdataFilename: "cloudflare_ruleset"},
		"cloudflare stream":                                  {identiferType: "account", resourceType: "cloudflare_stream", testdataFilename: "cloudflare_stream"},
		"cloudflare stream keys":                             {identiferType: "account", resourceType: "cloudflare_stream_key", testdataFilename: "cloudflare_stream_key"},
		"cloudflare stream live input":                       {identiferType: "account", resourceType: "cloudflare_stream_live_input", testdataFilename: "cloudflare_stream_live_input"},
		"cloudflare stream watermark":                        {identiferType: "account", resourceType: "cloudflare_stream_watermark", testdataFilename: "cloudflare_stream_watermark"},
		"cloudflare stream webhook":                          {identiferType: "account", resourceType: "cloudflare_stream_webhook", testdataFilename: "cloudflare_stream_webhook"},
		"cloudflare snippets":                                {identiferType: "zone", resourceType: "cloudflare_snippets", testdataFilename: "cloudflare_snippets"},
		"cloudflare snippet rules":                           {identiferType: "zone", resourceType: "cloudflare_snippet_rules", testdataFilename: "cloudflare_snippet_rules"},
		"cloudflare spectrum application":                    {identiferType: "zone", resourceType: "cloudflare_spectrum_application", testdataFilename: "cloudflare_spectrum_application"},
		"cloudflare tiered cache":                            {identiferType: "zone", resourceType: "cloudflare_tiered_cache", testdataFilename: "cloudflare_tiered_cache"},
		"cloudflare regional hostnames":                      {identiferType: "zone", resourceType: "cloudflare_regional_hostname", testdataFilename: "cloudflare_regional_hostname"},
		"cloudflare regional tiered cache":                   {identiferType: "zone", resourceType: "cloudflare_regional_tiered_cache", testdataFilename: "cloudflare_regional_tiered_cache"},
		// "cloudflare teams list":                              {identiferType: "account", resourceType: "cloudflare_teams_list", testdataFilename: "cloudflare_teams_list"},
		// "cloudflare teams location":                          {identiferType: "account", resourceType: "cloudflare_teams_location", testdataFilename: "cloudflare_teams_location"},
		// "cloudflare teams proxy endpoint":                    {identiferType: "account", resourceType: "cloudflare_teams_proxy_endpoint", testdataFilename: "cloudflare_teams_proxy_endpoint"},
		// "cloudflare teams rule":                              {identiferType: "account", resourceType: "cloudflare_teams_rule", testdataFilename: "cloudflare_teams_rule"},
		"cloudflare total tls": {identiferType: "zone", resourceType: "cloudflare_total_tls", testdataFilename: "cloudflare_total_tls"},
		// "cloudflare tunnel":                                  {identiferType: "account", resourceType: "cloudflare_tunnel", testdataFilename: "cloudflare_tunnel"},
		//"cloudflare turnstile_widget":            {identiferType: "account", resourceType: "cloudflare_turnstile_widget", testdataFilename: "cloudflare_turnstile_widget"},
		"cloudflare turnstile widget no domains": {identiferType: "account", resourceType: "cloudflare_turnstile_widget", testdataFilename: "cloudflare_turnstile_widget_no_domains"},
		// "cloudflare url normalization settings":              {identiferType: "zone", resourceType: "cloudflare_url_normalization_settings", testdataFilename: "cloudflare_url_normalization_settings"},
		// "cloudflare turnstile_widget":                        {identiferType: "account", resourceType: "cloudflare_turnstile_widget", testdataFilename: "cloudflare_turnstile_widget"},
		"cloudflare url normalization settings": {identiferType: "zone", resourceType: "cloudflare_url_normalization_settings", testdataFilename: "cloudflare_url_normalization_settings"},
		"cloudflare user":                       {identiferType: "account", resourceType: "cloudflare_user", testdataFilename: "cloudflare_user"},
		// "cloudflare user agent blocking rule":                {identiferType: "zone", resourceType: "cloudflare_user_agent_blocking_rule", testdataFilename: "cloudflare_user_agent_blocking_rule"},
		"cloudflare waiting room event": {identiferType: "zone", resourceType: "cloudflare_waiting_room_event", testdataFilename: "cloudflare_waiting_room_event", cliFlags: "cloudflare_waiting_room_event=e7f9e4c190ea8d6c66cab32ac110f39a"},
		"cloudflare waiting room rules": {identiferType: "zone", resourceType: "cloudflare_waiting_room_rules", testdataFilename: "cloudflare_waiting_room_rules", cliFlags: "cloudflare_waiting_room_rules=8bbd1b13450f6c63ab6ab4e08a63762d"},
		// "cloudflare waiting room settings":                   {identiferType: "zone", resourceType: "cloudflare_waiting_room_settings", testdataFilename: "cloudflare_waiting_room_settings"},
		"cloudflare web3 hostname": {identiferType: "zone", resourceType: "cloudflare_web3_hostname", testdataFilename: "cloudflare_web3_hostname"},
		// "cloudflare worker route":                            {identiferType: "zone", resourceType: "cloudflare_worker_route", testdataFilename: "cloudflare_worker_route"},
		// "cloudflare workers kv namespace":                    {identiferType: "account", resourceType: "cloudflare_workers_kv_namespace", testdataFilename: "cloudflare_workers_kv_namespace"},
		"cloudflare zone lockdown": {identiferType: "zone", resourceType: "cloudflare_zone_lockdown", testdataFilename: "cloudflare_zone_lockdown"},
		// "cloudflare tiered cache":                            {identiferType: "zone", resourceType: "cloudflare_tiered_cache", testdataFilename: "cloudflare_tiered_cache"},
		// "cloudflare access group (account)": {identiferType: "account", resourceType: "cloudflare_access_group", testdataFilename: "cloudflare_access_group_account"},
		// "cloudflare access group (zone)":    {identiferType: "zone", resourceType: "cloudflare_access_group", testdataFilename: "cloudflare_access_group_zone"},
		// "cloudflare custom certificates":    {identiferType: "zone", resourceType: "cloudflare_custom_certificates", testdataFilename: "cloudflare_custom_certificates"},
		// "cloudflare custom SSL": {identiferType: "zone", resourceType: "cloudflare_custom_ssl", testdataFilename: "cloudflare_custom_ssl"},
		"cloudflare queue":                                                   {identiferType: "account", resourceType: "cloudflare_queue", testdataFilename: "cloudflare_queue"},
		"cloudflare queue consumer":                                          {identiferType: "account", resourceType: "cloudflare_queue_consumer", testdataFilename: "cloudflare_queue_consumer", cliFlags: "cloudflare_queue_consumer=2dde6ac405cd457c9ce59dc4bda20c65"},
		"cloudflare web analytics site":                                      {identiferType: "account", resourceType: "cloudflare_web_analytics_site", testdataFilename: "cloudflare_web_analytics_site"},
		"cloudflare web analytics rule":                                      {identiferType: "account", resourceType: "cloudflare_web_analytics_rule", testdataFilename: "cloudflare_web_analytics_rule", cliFlags: "cloudflare_web_analytics_rule=2fa89d8f-35f7-49ef-87d3-f24e866a5d5e"},
		"cloudflare waiting room":                                            {identiferType: "zone", resourceType: "cloudflare_waiting_room", testdataFilename: "cloudflare_waiting_room"},
		"cloudflare waiting room settings":                                   {identiferType: "zone", resourceType: "cloudflare_waiting_room_settings", testdataFilename: "cloudflare_waiting_room_settings"},
		"cloudflare workers script subdomain":                                {identiferType: "account", resourceType: "cloudflare_workers_script_subdomain", testdataFilename: "cloudflare_workers_script_subdomain", cliFlags: "cloudflare_workers_script_subdomain=accounts"},
		"cloudflare workers deployment":                                      {identiferType: "account", resourceType: "cloudflare_workers_deployment", testdataFilename: "cloudflare_workers_deployment", cliFlags: "cloudflare_workers_deployment=script_2"},
		"cloudflare workers cron trigger":                                    {identiferType: "account", resourceType: "cloudflare_workers_cron_trigger", testdataFilename: "cloudflare_workers_cron_trigger", cliFlags: "cloudflare_workers_cron_trigger=script_2"},
		"cloudflare workers custom domain":                                   {identiferType: "account", resourceType: "cloudflare_workers_custom_domain", testdataFilename: "cloudflare_workers_custom_domain"},
		"cloudflare workers kv namespace":                                    {identiferType: "account", resourceType: "cloudflare_workers_kv_namespace", testdataFilename: "cloudflare_workers_kv_namespace"},
		"cloudflare workers for platforms dispatch namespace":                {identiferType: "account", resourceType: "cloudflare_workers_for_platforms_dispatch_namespace", testdataFilename: "cloudflare_workers_for_platforms_dispatch_namespace"},
		"cloudflare zero trust access application":                           {identiferType: "account", resourceType: "cloudflare_zero_trust_access_application", testdataFilename: "cloudflare_zero_trust_access_application"},
		"cloudflare zero trust access custom page":                           {identiferType: "account", resourceType: "cloudflare_zero_trust_access_custom_page", testdataFilename: "cloudflare_zero_trust_access_custom_page"},
		"cloudflare zero trust access group":                                 {identiferType: "account", resourceType: "cloudflare_zero_trust_access_group", testdataFilename: "cloudflare_zero_trust_access_group"},
		"cloudflare zero trust access identity provider":                     {identiferType: "zone", resourceType: "cloudflare_zero_trust_access_identity_provider", testdataFilename: "cloudflare_zero_trust_access_identity_provider"},
		"cloudflare zero trust access infrastructure target":                 {identiferType: "account", resourceType: "cloudflare_zero_trust_access_infrastructure_target", testdataFilename: "cloudflare_zero_trust_access_infrastructure_target"},
		"cloudflare zero trust access key configuration":                     {identiferType: "account", resourceType: "cloudflare_zero_trust_access_key_configuration", testdataFilename: "cloudflare_zero_trust_access_key_configuration"},
		"cloudflare zero trust access policy":                                {identiferType: "account", resourceType: "cloudflare_zero_trust_access_policy", testdataFilename: "cloudflare_zero_trust_access_policy"},
		"cloudflare zero trust access service token":                         {identiferType: "account", resourceType: "cloudflare_zero_trust_access_service_token", testdataFilename: "cloudflare_zero_trust_access_service_token"},
		"cloudflare zero trust access tag":                                   {identiferType: "account", resourceType: "cloudflare_zero_trust_access_tag", testdataFilename: "cloudflare_zero_trust_access_tag"},
		"cloudflare zero trust access short lived certificate":               {identiferType: "account", resourceType: "cloudflare_zero_trust_access_short_lived_certificate", testdataFilename: "cloudflare_zero_trust_access_short_lived_certificate"},
		"cloudflare zero trust risk scoring integration":                     {identiferType: "account", resourceType: "cloudflare_zero_trust_risk_scoring_integration", testdataFilename: "cloudflare_zero_trust_risk_scoring_integration"},
		"cloudflare zero trust dex test":                                     {identiferType: "account", resourceType: "cloudflare_zero_trust_dex_test", testdataFilename: "cloudflare_zero_trust_dex_test"},
		"cloudflare zero trust device custom profile":                        {identiferType: "account", resourceType: "cloudflare_zero_trust_device_custom_profile", testdataFilename: "cloudflare_zero_trust_device_custom_profile"},
		"cloudflare zero trust device posture rule":                          {identiferType: "account", resourceType: "cloudflare_zero_trust_device_posture_rule", testdataFilename: "cloudflare_zero_trust_device_posture_rule"},
		"cloudflare zero trust device posture integration":                   {identiferType: "account", resourceType: "cloudflare_zero_trust_device_posture_integration", testdataFilename: "cloudflare_zero_trust_device_posture_integration"},
		"cloudflare zero trust device managed networks":                      {identiferType: "account", resourceType: "cloudflare_zero_trust_device_managed_networks", testdataFilename: "cloudflare_zero_trust_device_managed_networks"},
		"cloudflare zero trust device default profile":                       {identiferType: "account", resourceType: "cloudflare_zero_trust_device_default_profile", testdataFilename: "cloudflare_zero_trust_device_default_profile"},
		"cloudflare zero trust device default profile local domain fallback": {identiferType: "account", resourceType: "cloudflare_zero_trust_device_default_profile_local_domain_fallback", testdataFilename: "cloudflare_zero_trust_device_default_profile_local_domain_fallback"},
		"cloudflare zero trust device default profile certificates":          {identiferType: "zone", resourceType: "cloudflare_zero_trust_device_default_profile_certificates", testdataFilename: "cloudflare_zero_trust_device_default_profile_certificates"},
		"cloudflare zero trust dlp dataset":                                  {identiferType: "account", resourceType: "cloudflare_zero_trust_dlp_dataset", testdataFilename: "cloudflare_zero_trust_dlp_dataset"},
		"cloudflare zero trust dlp predefined profile":                       {identiferType: "account", resourceType: "cloudflare_zero_trust_dlp_predefined_profile", testdataFilename: "cloudflare_zero_trust_dlp_predefined_profile", cliFlags: "cloudflare_zero_trust_dlp_predefined_profile=c8932cc4-3312-4152-8041-f3f257122dc4,56a8c060-01bb-4f89-ba1e-3ad42770a342"},
		"cloudflare zero trust dlp custom profile":                           {identiferType: "account", resourceType: "cloudflare_zero_trust_dlp_custom_profile", testdataFilename: "cloudflare_zero_trust_dlp_custom_profile", cliFlags: "cloudflare_zero_trust_dlp_custom_profile=38f45ad8-476e-4b56-ad16-42f364250802"},
		"cloudflare zero trust dns location":                                 {identiferType: "account", resourceType: "cloudflare_zero_trust_dns_location", testdataFilename: "cloudflare_zero_trust_dns_location"},
		"cloudflare zero trust gateway certificate":                          {identiferType: "account", resourceType: "cloudflare_zero_trust_gateway_certificate", testdataFilename: "cloudflare_zero_trust_gateway_certificate"},
		"cloudflare zero trust gateway policy":                               {identiferType: "account", resourceType: "cloudflare_zero_trust_gateway_policy", testdataFilename: "cloudflare_zero_trust_gateway_policy"},
		"cloudflare zero trust gateway proxy endpoint":                       {identiferType: "account", resourceType: "cloudflare_zero_trust_gateway_proxy_endpoint", testdataFilename: "cloudflare_zero_trust_gateway_proxy_endpoint"},
		"cloudflare zero trust list":                                         {identiferType: "account", resourceType: "cloudflare_zero_trust_list", testdataFilename: "cloudflare_zero_trust_list"},
		"cloudflare zero trust gateway settings":                             {identiferType: "account", resourceType: "cloudflare_zero_trust_gateway_settings", testdataFilename: "cloudflare_zero_trust_gateway_settings"},
		"cloudflare zero trust organization":                                 {identiferType: "account", resourceType: "cloudflare_zero_trust_organization", testdataFilename: "cloudflare_zero_trust_organization"},
		"cloudflare zero trust risk behavior":                                {identiferType: "account", resourceType: "cloudflare_zero_trust_risk_behavior", testdataFilename: "cloudflare_zero_trust_risk_behavior"},
		"cloudflare zero trust tunnel cloudflared":                           {identiferType: "account", resourceType: "cloudflare_zero_trust_tunnel_cloudflared", testdataFilename: "cloudflare_zero_trust_tunnel_cloudflared"},
		"cloudflare zero trust tunnel cloudflared route":                     {identiferType: "account", resourceType: "cloudflare_zero_trust_tunnel_cloudflared_route", testdataFilename: "cloudflare_zero_trust_tunnel_cloudflared_route"},
		"cloudflare zero trust tunnel cloudflared virtual network":           {identiferType: "account", resourceType: "cloudflare_zero_trust_tunnel_cloudflared_virtual_network", testdataFilename: "cloudflare_zero_trust_tunnel_cloudflared_virtual_network"},
		"cloudflare zero trust tunnel cloudflared config":                    {identiferType: "account", resourceType: "cloudflare_zero_trust_tunnel_cloudflared_config", testdataFilename: "cloudflare_zero_trust_tunnel_cloudflared_config", cliFlags: "cloudflare_zero_trust_tunnel_cloudflared_config=285f508d-d6ef-4ce4-9293-983d5bdc269e"},
		"cloudflare zero trust access mtls certificate":                      {identiferType: "account", resourceType: "cloudflare_zero_trust_access_mtls_certificate", testdataFilename: "cloudflare_zero_trust_access_mtls_certificate"},
		"cloudflare zero trust access mtls hostname settings":                {identiferType: "account", resourceType: "cloudflare_zero_trust_access_mtls_hostname_settings", testdataFilename: "cloudflare_zero_trust_access_mtls_hostname_settings"},
		"cloudflare zone":                                                    {identiferType: "zone", resourceType: "cloudflare_zone", testdataFilename: "cloudflare_zone"},
		"cloudflare zone dnssec":                                             {identiferType: "zone", resourceType: "cloudflare_zone_dnssec", testdataFilename: "cloudflare_zone_dnssec"},
		"cloudflare zone setting":                                            {identiferType: "zone", resourceType: "cloudflare_zone_setting", testdataFilename: "cloudflare_zone_setting", cliFlags: "cloudflare_zone_setting=always_online,cache_level"},
		"cloudflare zone cache variants":                                     {identiferType: "zone", resourceType: "cloudflare_zone_cache_variants", testdataFilename: "cloudflare_zone_cache_variants"},
		"cloudflare zone cache reserve":                                      {identiferType: "zone", resourceType: "cloudflare_zone_cache_reserve", testdataFilename: "cloudflare_zone_cache_reserve"},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Reset the environment variables used in test to ensure we don't
			// have both present at once.
			viper.Set("zone", "")
			viper.Set("account", "")

			var r *recorder.Recorder
			var err error
			if os.Getenv("OVERWRITE_VCR_CASSETTES") == "true" {
				r, err = recorder.NewAsMode("../../../../testdata/cloudflare/v5/"+tc.testdataFilename, recorder.ModeRecording, http.DefaultTransport)
			} else {
				r, err = recorder.New("../../../../testdata/cloudflare/v5/" + tc.testdataFilename)
			}

			if err != nil {
				log.Fatal(err)
			}
			defer func() {
				err := r.Stop()
				if err != nil {
					log.Fatal(err)
				}
			}()

			r.AddFilter(func(i *cassette.Interaction) error {
				// Sensitive HTTP headers
				delete(i.Request.Headers, "X-Auth-Email")
				delete(i.Request.Headers, "X-Auth-Key")
				delete(i.Request.Headers, "Authorization")

				// HTTP request headers that we don't need to assert against
				delete(i.Request.Headers, "User-Agent")

				// HTTP response headers that we don't need to assert against
				delete(i.Response.Headers, "Cf-Cache-Status")
				delete(i.Response.Headers, "Cf-Ray")
				delete(i.Response.Headers, "Date")
				delete(i.Response.Headers, "Server")
				delete(i.Response.Headers, "Set-Cookie")
				delete(i.Response.Headers, "X-Envoy-Upstream-Service-Time")

				if os.Getenv("CLOUDFLARE_DOMAIN") != "" {
					i.Response.Body = strings.ReplaceAll(i.Response.Body, os.Getenv("CLOUDFLARE_DOMAIN"), "example.com")
				}

				return nil
			})

			output := ""
			api = cloudflare.NewClient(option.WithHTTPClient(
				&http.Client{
					Transport: r,
				},
			))
			apiV0, _ = cfv0.New(viper.GetString("key"), viper.GetString("email"), cfv0.HTTPClient(
				&http.Client{
					Transport: r,
				},
			))
			if tc.identiferType == "account" {
				viper.Set("account", cloudflareTestAccountID)
				if tc.cliFlags != "" {
					output, _ = executeCommandC(rootCmd, "generate",
						"--resource-type", tc.resourceType,
						"--account", cloudflareTestAccountID,
						"--resource-id", tc.cliFlags,
					)
				} else {
					output, _ = executeCommandC(rootCmd, "generate", "--resource-type", tc.resourceType, "--account", cloudflareTestAccountID)
				}

			} else {
				viper.Set("zone", cloudflareTestZoneID)
				if tc.cliFlags != "" {
					output, _ = executeCommandC(rootCmd, "generate",
						"--resource-type", tc.resourceType,
						"--zone", cloudflareTestZoneID,
						"--resource-id", tc.cliFlags,
					)
				} else {
					output, _ = executeCommandC(rootCmd, "generate", "--resource-type", tc.resourceType, "--zone", cloudflareTestZoneID)
				}

			}
			expected := testDataFile("v5", tc.testdataFilename)
			assert.Equal(t, strings.TrimRight(expected, "\n"), strings.TrimRight(output, "\n"))
		})
	}
}
