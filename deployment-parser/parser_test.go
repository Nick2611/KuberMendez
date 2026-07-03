package parser

import "testing"

func TestParserFields(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name: "valid manifest",
			input: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: NicoNginx
spec:
  selector:
    matchLabels:
      app: NicoNginx
  template:
    metadata:
      labels:
        app: NicoNginx
    spec:
      containers:
        - name: nginx
          image: nginx
`,
			wantErr: false,
		},
		{
			name: "missing metadata",
			input: `apiVersion: apps/v1
kind: Deployment
spec:
  selector:
    matchLabels:
      app: NicoNginx
  template:
    metadata:
      labels:
        app: NicoNginx
    spec:
      containers:
        - name: nginx
          image: nginx
`,
			wantErr: true,
		},
		{
			name: "missing apiVersion",
			input: `kind: Deployment
metadata:
  name: Nico
spec:
  selector:
    matchLabels:
      app: Nico
  template:
    metadata:
      labels:
        app: Nico
    spec:
      containers:
        - name: Nico
          image: nginx
`,
			wantErr: true,
		},
		{
			name: "missing spec",
			input: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: Nico
`,
			wantErr: true,
		},
		{
			name: "missing container image",
			input: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: Nico
spec:
  selector:
    matchLabels:
      app: Nico
  template:
    metadata:
      labels:
        app: Nico
    spec:
      containers:
        - name: Nico
`,
			wantErr: true,
		},
		{
			name: "bad indented file",
			input: `
				apiVersion: apps/v1
	kind: Deployment
	metadata:
	name: Nico
	spec:
	selector:
		matchLabels:
		app: Nico
	template:
		metadata:
		labels:
			app: Nico
		spec:
		containers:
			- name: Nico
			`,
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Parser([]byte(test.input))

			if test.wantErr && err == nil {
				t.Fatal("Parser() returned nil error, want error")
			}

			if !test.wantErr && err != nil {
				t.Fatalf("Parser() returned error, want nil: %v", err)
			}
		})
	}
}

func TestValidator(t *testing.T){
	tests := []struct{
		name string
		input string
		wantErr bool
	} {
		{
		name: "bad container name",
		input: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: NicoNginx
spec:
  selector:
    matchLabels:
      app: NicoNginx
  template:
    metadata:
      labels:
        app: NicoNginx
    spec:
      containers:
        - name: NGINX
          image: nginx
`,
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := Validation([]byte(test.input))

			if test.wantErr && err == nil {
				t.Fatal("Validator() returned nil error, want error")
			}

			if !test.wantErr && err != nil {
				t.Fatalf("Validator() returned error, want nil: %v", err)
			}
		})
	}
}