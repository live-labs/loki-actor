# Loki-actor

Loki-actor is a tool designed to consume logs from Loki and trigger events based on predefined conditions.

## Features
- Real-time Log Consumption: Seamlessly integrates with Loki to consume logs in real-time.
- Event Triggering: Automatically triggers events based on customizable conditions and rules.
- Customizable Actions: Define your own actions to be executed when specific log patterns are detected.

## Example config

```yaml
loki:
  host: "loki.example.com"
  port: 3100

# each flow is built around separate query (to get a stream of logs)
# in each flow, you can define multiple triggers, based on regex
# in each trigger, you can define an action to execute
# if multiple triggers fire for the same line, only the first one will be run

actions:
  base_action:
    type: 'cmd'
    cmd_run: [ 'echo', '!!!!!', 'error', '${labels.host}', '${labels.container_name}', '${values.message}' ]
  example:
    extends: base_action

flows:
  - name: 'Example flow'
    # LogQL query to get filtered logs from Loki. Be sure to adjust the query to match your log structure and make it
    # more specific if needed.
    query: '{compose_project="example", container_name =~ "example.*"}'
    triggers:
      - name: 'error'
        # Regex to match error messages in logs. Is applied only to the message part of the log.
        regex: 'ERR|ERROR'
        ignore_regex: 'Authentication failed'
        # List of actions to perform, currently only 'run' is supported.
        # 'run' executes a command with the specified arguments.
        # substitutions are available for `${labels.*}` and `${values.ts} and ${values.message}`
        # Navigate your logs in grafana to see the available labels for your project
        action: example
```

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
