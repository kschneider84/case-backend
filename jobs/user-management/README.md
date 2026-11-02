# User Management Job

This job handles scheduled user management operations including account cleanup, verification reminders, inactivity management, and profile ID lookup generation. It helps maintain data integrity and manages the user lifecycle across studies.

## Configuration

### Required Environment Variable

- `CONFIG_FILE_PATH` - Path to the YAML configuration file

### Optional Environment Variables (Secret Overrides)

The following environment variables can be used to override secrets from the configuration file:

#### Database Credentials

- `STUDY_DB_USERNAME` - Override study database username
- `STUDY_DB_PASSWORD` - Override study database password
- `PARTICIPANT_USER_DB_USERNAME` - Override participant user database username
- `PARTICIPANT_USER_DB_PASSWORD` - Override participant user database password
- `GLOBAL_INFOS_DB_USERNAME` - Override global infos database username
- `GLOBAL_INFOS_DB_PASSWORD` - Override global infos database password
- `MESSAGING_DB_USERNAME` - Override messaging database username
- `MESSAGING_DB_PASSWORD` - Override messaging database password

#### Messaging Configuration

- `SMTP_BRIDGE_API_KEY` - Override SMTP bridge API key for email sending

#### Study Configuration

- `STUDY_GLOBAL_SECRET` - Override the global secret used for study operations

#### External Service API Keys

For each external service that has a `name` defined, you can override the `api_key` using environment variables with the following naming pattern:

**Format**: `EXTERNAL_SERVICE_API_KEY_FOR_{NORMALIZED_NAME}`

**Name Normalization Rules**:

- Convert to uppercase
- Replace any non-alphanumeric characters with underscores
- Remove leading/trailing underscores

**Examples**:

- Service name: `"notification-api_v2"` → Environment variable: `EXTERNAL_SERVICE_API_KEY_FOR_NOTIFICATION_API_V2`
- Service name: `"webhook-handler"` → Environment variable: `EXTERNAL_SERVICE_API_KEY_FOR_WEBHOOK_HANDLER`

**Note**: Only services with a defined `name` field will have their API keys overridden. Services without names will be skipped.

## Configuration File Example

```yaml
# Logging configuration
logging:
  log_level: "info"
  include_src: true
  log_to_file: true
  filename: "user-management.log"
  max_size: 100
  max_age: 28
  max_backups: 3
  compress_old_logs: true
  include_build_info: "once" # one of: never, always, once

# Database configurations
db_configs:
  participant_user_db:
    connection_str: "<connection_str>"
    username: "<env var PARTICIPANT_USER_DB_USERNAME>"
    password: "<env var PARTICIPANT_USER_DB_PASSWORD>"
    connection_prefix: ""
    timeout: 30
    idle_conn_timeout: 45
    max_pool_size: 4
    use_no_cursor_timeout: false
    db_name_prefix: ""

  global_infos_db:
    connection_str: "<connection_str>"
    username: "<env var GLOBAL_INFOS_DB_USERNAME>"
    password: "<env var GLOBAL_INFOS_DB_PASSWORD>"
    connection_prefix: ""
    timeout: 30
    idle_conn_timeout: 45
    max_pool_size: 4
    use_no_cursor_timeout: false
    db_name_prefix: ""

  messaging_db:
    connection_str: "<connection_str>"
    username: "<env var MESSAGING_DB_USERNAME>"
    password: "<env var MESSAGING_DB_PASSWORD>"
    connection_prefix: ""
    timeout: 30
    idle_conn_timeout: 45
    max_pool_size: 4
    use_no_cursor_timeout: false
    db_name_prefix: ""

  study_db:
    connection_str: "<connection_str>"
    username: "<env var STUDY_DB_USERNAME>"
    password: "<env var STUDY_DB_PASSWORD>"
    connection_prefix: ""
    timeout: 30
    idle_conn_timeout: 45
    max_pool_size: 4
    use_no_cursor_timeout: false
    db_name_prefix: ""

# List of instance IDs to process
instance_ids:
  - "default"
  - "research_instance"
  - "pilot_study"

# User management configuration
user_management_config:
  delete_unverified_users_after: "168h"
  send_reminder_to_confirm_account_after: "48h"
  email_contact_verification_token_ttl: "48h"
  notify_after_inactive_for: "8760h"
  mark_for_deletion_after_inactivity_notification: "336h"

# Messaging configuration
messaging_configs:
  smtp_bridge_config:
    url: "https://smtp-bridge.example.com"
    api_key: "default_smtp_bridge_key"
    timeout: 30

  global_email_template_constants:
    "app_name": "Research Platform"
    "support_email": "support@example.com"
    "website_url": "https://example.com"

# Study module configuration
study_configs:
  global_secret: "your_global_secret_here"

  # External services for user management actions
  external_services:
    - name: "email-service"
      url: "https://api.email-provider.com"
      api_key: "default_email_api_key"
      timeout: 30
    - name: "audit-service"
      url: "https://api.audit-service.com"
      api_key: "default_audit_key"
      timeout: 15

# Task execution configuration
run_tasks:
  clean_up_unverified_users: true
  send_reminder_to_confirm_accounts: true
  handle_inactive_users: true
  generate_profile_id_lookup: true
```

## Usage

1. Create a configuration file based on the example above
2. Set the `CONFIG_FILE_PATH` environment variable to point to your config file
3. Optionally set any secret override environment variables
4. Run the user management job

Example:

```bash
export CONFIG_FILE_PATH="/path/to/your/config.yaml"
export PARTICIPANT_USER_DB_PASSWORD="secure_user_db_password"
export MESSAGING_DB_PASSWORD="secure_messaging_db_password"
export SMTP_BRIDGE_API_KEY="secure_smtp_bridge_key"
export STUDY_GLOBAL_SECRET="secure_global_secret"
export EXTERNAL_SERVICE_API_KEY_FOR_EMAIL_SERVICE="secure_email_service_key"
./user-management
```

## Features

### Account Verification Management

- **Unverified User Cleanup**: Automatically removes users who haven't verified their accounts within the configured timeframe
- **Verification Reminders**: Sends reminder emails to users who need to confirm their accounts
- **Email Notifications**: Sends appropriate email notifications during account deletion

### Inactivity Management

- **Inactivity Detection**: Identifies users who haven't been active for a specified period
- **Warning Notifications**: Sends warning emails to inactive users before account deletion
- **Grace Period**: Provides a configurable grace period after warning before deletion
- **Automatic Cleanup**: Removes accounts marked for deletion after the grace period expires

### Profile ID Lookup Generation

- **Confidential ID Mapping**: Generates lookup tables mapping profile IDs to confidential study IDs
- **Multi-Study Support**: Creates mappings for all active studies across instances
- **Data Integrity**: Ensures consistent participant identification across studies
- **Automatic Updates**: Updates lookup tables for existing users and studies

### Task Configuration

All major operations can be individually enabled or disabled:

- **clean_up_unverified_users**: Remove unverified accounts
- **send_reminder_to_confirm_accounts**: Send verification reminders
- **handle_inactive_users**: Manage inactive user workflow
- **generate_profile_id_lookup**: Update profile ID mappings

## Configuration Details

### Time Durations

All duration fields accept Go duration format (e.g., "24h", "168h", "30m"):

- **delete_unverified_users_after**: How long to wait before deleting unverified accounts
- **send_reminder_to_confirm_account_after**: When to send verification reminders
- **email_contact_verification_token_ttl**: How long verification tokens remain valid
- **notify_after_inactive_for**: Inactivity threshold for warnings
- **mark_for_deletion_after_inactivity_notification**: Grace period after inactivity warning

### Email Templates

The job uses predefined email templates for:

- **REGISTRATION**: Account verification reminders
- **ACCOUNT_DELETED**: Notification when unverified account is deleted
- **ACCOUNT_INACTIVITY**: Warning about account inactivity
- **ACCOUNT_DELETED_AFTER_INACTIVITY**: Notification when inactive account is deleted
