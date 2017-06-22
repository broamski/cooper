# cooper
Broker access to all of your AWS accounts

----

Consider the scenario where you need access to a number of AWS accounts. You may **own** or **manage** _(have administrator access or root credential access)_ some or all of these accounts. Regardless, the management cost associated with accessing these accounts via individual IAM Users becomes linear as the number of accounts increase. Thankfully, AWS has lessened the burden by offering the ability to obtain access to any number of accounts through the Security Token Service (STS). **`cooper`** utilizes the `AssumeRole` and `GetFederationToken` STS API actions to provide you a simple way to access all of your accounts.

### Implementation Details

Option 1: Run cooper on EC2, leveraging IAM Roles for Amazon EC2, eliminating the need to store authortative credentials in code or config.

Option 2: Run cooper on compute resources outside of AWS, such as your datacenter or another provider. This is least preferred as you fork your security framework and are required to use IAM user access keys.

Create an AWS account exclusively for cooper operations. This can reduce the scope and increase the security posture of cooper.

----

### Concepts

#### **Target** - A `Target` is any IAM Role or Federated User that you could potentially become.

#### **Target Assoication** - A user is **associated** to a `Target`, meaning that user is allowed to pursue becoming that target.

----

#### Build Specifications

##### Technologies
 - DynamoDB for persistent storage of cooper data
 - KMS -- A KMS Key is required for referencing sensitive data

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


----

#### Todo:
- [ ] CSRF Protection
- [ ] SAML Authentication (pick lib, implement)

---

#### Execution Examples

`cooper -setup` - Creates the necessary DynamoDB tables when first installing the app. This does not start the application

`cooper -encrypt -encrypt-payload "AKIAIOSFODNN7EXAMPLE|wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY" --kmskey 0bd3695d-96a9-4642-8477-0a95e3b968fd -region us-east-1` - This will output a base64 encoded string you can use to supply the "Federation Keys" field when creating a new target

`cooper -region us-east-1` - Starts the application
