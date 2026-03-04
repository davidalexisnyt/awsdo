# Get AWS Credentials

One of our applications supports accessing S3 accounts through the AWS SDK, which requires authentication using the access key, secret, and token for the target AWS account. We need a feature in `awsdo` that would obtain and print out these credentials so that the user can use them in our other applications for authentication.
Determine the best or most appropriate way to obtain the credentials for the account tied to a specified AWS CLI profile - e.g. would this be doing via the SDK or via the AWS CLI?
