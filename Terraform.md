# Terraform Module Verification Prompt

## Instructions for LLM Verification

You are tasked with verifying that the Terraform module located in `terraform/modules/duracloud/` contains all the necessary resources and configurations to replicate the infrastructure defined in the CloudFormation template `template.yaml` (which serves as the source of truth).

### Verification Process

1. **Read and Analyze the CloudFormation Template**
   - Examine `template.yaml` in the project root
   - Identify ALL resources defined in the `Resources:` section
   - Note all parameters, conditions, and their usage
   - Document all resource properties, dependencies, and configurations

2. **Inventory CloudFormation Resources**
   Create a comprehensive list of all resources by type:
   - AWS::Serverless::Function (Lambda functions)
   - AWS::DynamoDB::Table
   - AWS::S3::Bucket and AWS::S3::BucketPolicy
   - AWS::Events::Rule (EventBridge rules)
   - AWS::SQS::Queue
   - AWS::SNS::Topic and AWS::SNS::Subscription
   - AWS::IAM::Role, AWS::IAM::Policy, AWS::IAM::Group, AWS::IAM::User, AWS::IAM::AccessKey
   - AWS::Lambda::Permission and AWS::Lambda::EventSourceMapping
   - AWS::CloudWatch::Alarm
   - AWS::Logs::LogGroup
   - AWS::SSM::Parameter
   - Any other resource types present

3. **Examine the Terraform Module**
   - Review all `.tf` files in `terraform/modules/duracloud/`
   - Map each CloudFormation resource to its Terraform equivalent
   - Verify resource configurations match exactly

4. **Detailed Verification Checklist**

   For each CloudFormation resource, verify:
   
   **Lambda Functions:**
   - All 8 functions are present with correct names
   - Environment variables match exactly
   - Memory, timeout, and architecture settings are identical
   - IAM policies and permissions are equivalent
   - Docker image configurations are preserved
   - Log group associations are correct

   **DynamoDB Tables:**
   - Table names, billing modes, and key schemas match
   - Stream specifications are identical
   - Point-in-time recovery settings match
   - TTL configurations are preserved

   **S3 Buckets:**
   - All bucket names and configurations match
   - Lifecycle rules are identical
   - Bucket policies are equivalent
   - Notification configurations match
   - Dependencies are preserved

   **EventBridge Rules:**
   - Schedule expressions are identical
   - Event patterns match exactly
   - Target configurations are equivalent
   - Rule states (enabled/disabled) match

   **SQS Queues:**
   - Queue names and properties match
   - Dead letter queue configurations are identical
   - Visibility timeouts and retention periods match
   - Redrive policies are equivalent

   **IAM Resources:**
   - All roles, policies, groups, and users are present
   - Policy documents are functionally equivalent
   - Trust relationships match
   - Group memberships are preserved
   - Access key configurations match

   **CloudWatch Alarms:**
   - All alarm configurations are identical
   - Metric names, thresholds, and evaluation periods match
   - SNS topic associations are preserved
   - Alarm actions and OK actions match

   **Event Source Mappings:**
   - DynamoDB stream mappings are identical
   - SQS queue mappings match
   - Batch sizes and filtering criteria are preserved
   - Starting positions match

   **Lambda Permissions:**
   - All invoke permissions are present
   - Principal and source ARN configurations match
   - Function associations are correct

   **SSM Parameters:**
   - Parameter names, types, and values match
   - Descriptions are preserved

5. **Configuration Accuracy**
   - Verify all resource properties match exactly
   - Check that all references and dependencies are maintained
   - Ensure conditional logic is properly implemented
   - Validate that all parameter substitutions work correctly
   - Confirm that resource naming conventions are preserved

6. **Missing Resources Report**
   If any CloudFormation resources are missing from the Terraform module:
   - List each missing resource with its CloudFormation logical ID
   - Specify the resource type and key properties
   - Indicate which Terraform file should contain the resource
   - Provide the equivalent Terraform resource type

7. **Configuration Discrepancies**
   For resources that exist but have different configurations:
   - Identify the specific property differences
   - Show the CloudFormation value vs. Terraform value
   - Explain the impact of the discrepancy
   - Recommend the correct Terraform configuration

8. **Validation Summary**
   Provide a final summary including:
   - Total number of CloudFormation resources
   - Number of resources correctly implemented in Terraform
   - Number of missing resources
   - Number of resources with configuration discrepancies
   - Overall completeness percentage
   - Priority recommendations for fixes

### Expected Outcome

The verification should result in a comprehensive report that either:
- Confirms 100% parity between CloudFormation and Terraform implementations
- Provides a detailed action plan to achieve complete parity

### Important Notes

- The CloudFormation template is the authoritative source of truth
- Every resource, property, and configuration must be replicated exactly
- Pay special attention to resource dependencies and references
- Conditional resources in CloudFormation must be handled appropriately in Terraform
- Parameter substitutions and dynamic values must be preserved
- Resource naming patterns must be maintained consistently

This verification ensures that the Terraform module can deploy identical infrastructure to what the CloudFormation template would create.