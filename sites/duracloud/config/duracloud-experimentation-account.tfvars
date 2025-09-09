stacks = {
  duracloud-test = {
    alert_email_address        = "admin@duracloud.org"
    checksum_exporter_schedule = "cron(0 6 * * ? *)"
    lambda_architecture        = "arm64"
    report_generator_schedule  = "cron(0 8 * * ? *)"
  }
}
