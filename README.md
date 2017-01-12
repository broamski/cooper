# cooper
Broker access all of your AWS accounts with ease

----

Consider the scenario where you need access to a number of AWS accounts. You may **own** or **manage** _(have administrator access or root credential access)_ some or all of these accounts. Regardless, the management cost associated with accessing these accounts via individual IAM Users becomes linear as the number of accounts increase. Thankfully, AWS has lessened the burden by offering the ability to obtain access to any number of accounts through the Security Token Service (STS). **`cooper`** utilizes the `AssumeRole` and `GetFederationToken` STS API actions to provide you a simple way to access all of your accounts.

### Implementation Details

Option 1: Run cooper on EC2, leveraging IAM Roles for Amazon EC2, eliminating the need to store authortative credentials in code or config.

Option 2: Run cooper on compute resources outside of AWS, such as your datacenter or another provider. This is least preferred as you fork your security framework and are required to use IAM user access keys.

Create an AWS account exclusively for cooper operations. This can reduce the scope and increase the security posture of cooper.

--

#### Build Specifications

##### Technologies
 - DynamoDB for persistent storage of cooper data
 - KMS -- A KMS Key is required for storing sensitive data in the DynamDB table (TOTP tokens) 

#### Authentication - Extendable and flexible, to include any number of these:
 - Local authentication sceheme (local meaning handled on the backend)
 - SAML Integration - Use your typical SSO: Okta, OneLogin, etc.
 - Google Authentication - Google for work is popular, so this is helpful 

#### Authorization

Administrators
 - Universal Scope
 - AWS Account Scope
 
Assumption Targets
 - IAM Roles
 - Federated Users
 
**High Security Areas** - If an Admin user is operating within a sensitive area, require the users setup a local TOTP that they must provide in order to perform the particular operation.
