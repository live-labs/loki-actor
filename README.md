# Loki-actor

Loki-actor is a tool designed to consume logs from Loki and trigger events based on predefined conditions.

## Features
- Real-time Log Consumption: Seamlessly integrates with Loki to consume logs in real-time.
- Event Triggering: Automatically triggers events based on customizable conditions and rules.
- Customizable Actions: Define your own actions to be executed when specific log patterns are detected.

## Configuration Guide

### Table of Contents
1. [Basic Configuration Structure](#basic-configuration-structure)
2. [Loki Connection Settings](#loki-connection-settings)
3. [Actions Configuration](#actions-configuration)
    - [Action Types](#action-types)
    - [Action Inheritance](#action-inheritance)
4. [Flows Configuration](#flows-configuration)
    - [Flow Structure](#flow-structure)
    - [Triggers](#triggers)
5. [Variable Substitution](#variable-substitution)
6. [Complete Configuration Example](#complete-configuration-example)

### Basic Configuration Structure

The configuration file uses YAML format and consists of three main sections:
```yaml
loki:
  # Loki connection settings
actions:
  # Action definitions
flows:
  # Flow definitions
```

### Loki Connection Settings

Configure the connection to your Loki instance:
```yaml
loki:
  host: "loki.example.com"  # Loki server hostname
  port: 3100                # Loki server port
```

### Actions Configuration

### Variable Substitution

The following variables are available in actions:
- `${labels.*}`: Access to any Loki label (e.g., `${labels.host}`, `${labels.container_name}`)
- `${values.ts}`: Timestamp of the log entry
- `${values.message}`: The log message content


#### Action Types

Loki-actor supports two types of actions:

1. **Slack Actions**:
```yaml
actions:
  my_slack_action:
    type: 'slack'
    slack_webhook_url: 'https://hooks.slack.com/services/YOUR/WEBHOOK/URL'
    slack_timeout_sec: 5
    slack_message_template: |
      *Message from ${labels.container_name}*
      ```
      ${values.message}
      ```
    slack_concat: 0              # Optional: number of messages to concatenate
    slack_concat_prefix: ""      # Optional: prefix for concatenated messages
    slack_concat_suffix: ""      # Optional: suffix for concatenated messages
```

2. **Command Actions**:
```yaml
actions:
  my_cmd_action:
    type: 'cmd'
    cmd_run: ['echo', 'Error in ${labels.container_name}:', '${values.message}']
```

#### Action Inheritance

Actions can inherit properties from other actions using the `extends` field:
```yaml
actions:
  base_action:
    abstract: true    # Mark as abstract to prevent direct usage
    type: 'slack'
    slack_webhook_url: 'https://hooks.slack.com/services/YOUR/WEBHOOK/URL'
  
  derived_action:
    extends: base_action
    slack_message_template: "Custom message: ${values.message}"
```

### Flows Configuration

#### Flow Structure

Flows define what logs to monitor and how to respond to them:
```yaml
flows:
  my_flow:
    query: '{compose_project="example", container_name=~"app.*"}'
    triggers:
      # Trigger definitions
```

Flows can also inherit from other flows:
```yaml
flows:
  base_flow:
    abstract: true
    triggers:
      # Base triggers
  
  specific_flow:
    extends: base_flow
    query: '{compose_project="myproject"}'
```

#### Triggers

Triggers define patterns to match in logs and actions to take:
```yaml
triggers:
  - name: "error_trigger"
    regex: "ERR|ERROR"              # Pattern to match
    ignore_regex: "ignored_pattern" # Optional pattern to ignore
    lines: 30                       # Optional: capture additional lines
    action: "main_action"           # Action for matched line
    next_lines_action: "follow_up"  # Action for additional captured lines
```

### Complete Configuration Example

```yaml
loki:
  host: "loki.example.com"
  port: 3100

actions:
  base_slack:
    abstract: true
    type: 'slack'
    slack_webhook_url: 'https://hooks.slack.com/services/YOUR/WEBHOOK/URL'
    slack_timeout_sec: 5

  error_notification:
    extends: base_slack
    slack_message_template: |
      *Error in ${labels.container_name}*
      ```
      ${values.message}
      ```

  stack_trace_first:
    type: 'cmd'
    cmd_run: ['echo', 'Exception detected:', '${values.message}']

  stack_trace_next:
    type: 'cmd'
    cmd_run: ['echo', '${values.message}']

flows:
  error_monitoring:
    abstract: true
    triggers:
      - name: "error_detection"
        regex: "ERROR|Exception:"
        ignore_regex: "Caused by:"
        lines: 30
        action: "error_notification"
      
      - name: "stack_trace"
        regex: "Exception:"
        lines: 30
        action: "stack_trace_first"
        next_lines_action: "stack_trace_next"

  project1:
    extends: error_monitoring
    query: '{compose_project="myapp"}'

  project2:
    extends: error_monitoring
    query: '{compose_project="anotherapp"}'
```

This example demonstrates both Slack and command actions, inheritance, and multiline trigger handling.

See [example-config.yml](example-config.yml) for more examples.

## How to run

`loki-actor -config <path_to_config.yml>`

## Docker compose

```yaml
services:
  loki-actor:
    image: ghcr.io/live-labs/loki-actor:latest
    container_name: loki-actor
    restart: unless-stopped
    volumes:
      - ./config/example-config.yml:/etc/loki-actor/config.yml
```