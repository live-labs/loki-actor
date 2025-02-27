package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name        string
		fileContent string
		expectedErr bool
	}{
		{
			name: "ValidConfig",
			fileContent: `
flows:
  - triggers:
      - name: "Trigger1"
        regex: ".*test.*"
        ignore_regex: "^ignore$"
`,
			expectedErr: false,
		},
		{
			name: "ValidConfig2",
			fileContent: `
flows:
  - triggers:
      - name: "Trigger2"
        regex: ".*test.*"
        ignore_regex: "^ignore$"
        duration_ms: 1000
        continuation_action:
          run: [ 'echo', '!!!!!', 'exception', '${labels.host}', '${labels.container_name}', '${values.message}' ]
        actions:
          - run: [ 'echo', '!!!!!', 'exception', '${labels.host}', '${labels.container_name}', '${values.message}' ]
          - run: [ 'curl', '-X', 'POST', '-H', 'Content-Type: application/json', '-d',
                   '{ "text": ":boom: ${labels.host} - ${labels.container_name} - ${values.message}" }',
                   'https://hooks.slack.com/services/TXXXXXXXXXX/BXXXXXXXXXX/JXXXXXXXXXXXXXXXXXXXXXXX' ]

`,
			expectedErr: false,
		},
		{
			name: "InvalidYAML",
			fileContent: `flows:
  - triggers
    - name: "Trigger1"`,
			expectedErr: true,
		},
		{
			name: "InvalidRegex",
			fileContent: `
flows:
  - triggers:
      - name: "Trigger1"
        regex: "["
`,
			expectedErr: true,
		},
		{
			name: "InvalidIgnoreRegex",
			fileContent: `
flows:
  - triggers:
      - name: "Trigger1"
        regex: ".*test.*"
        ignore_regex: "["
`,
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			filePath := filepath.Join(tempDir, "config.yaml")

			err := os.WriteFile(filePath, []byte(tt.fileContent), 0644)
			if err != nil {
				t.Fatalf("failed to create temp config file: %v", err)
			}

			_, err = Load(filePath)
			if tt.expectedErr && err == nil {
				t.Errorf("expected an error but got nil")
			}
			if !tt.expectedErr && err != nil {
				t.Errorf("did not expect an error but got: %v", err)
			}
		})
	}
}
