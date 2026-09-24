# The account runs on AWS Free plan credits; when they run out the account closes, so this
# measures gross spend before credits. There is never a bill to catch.
resource "aws_budgets_budget" "monthly" {
  name         = "hausy-monthly-gross"
  budget_type  = "COST"
  limit_amount = "20"
  limit_unit   = "USD"
  time_unit    = "MONTHLY"

  cost_types {
    include_credit = false
  }

  notification {
    notification_type          = "ACTUAL"
    comparison_operator        = "GREATER_THAN"
    threshold                  = 80
    threshold_type             = "PERCENTAGE"
    subscriber_email_addresses = ["mendeznicolas2611@gmail.com"]
  }

  notification {
    notification_type          = "FORECASTED"
    comparison_operator        = "GREATER_THAN"
    threshold                  = 100
    threshold_type             = "PERCENTAGE"
    subscriber_email_addresses = ["mendeznicolas2611@gmail.com"]
  }
}
