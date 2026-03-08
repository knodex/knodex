package metadata

import (
	"testing"
)

func TestValidateInstanceLabels(t *testing.T) {
	tests := []struct {
		name    string
		labels  map[string]string
		wantErr bool
	}{
		{
			name: "all required labels present",
			labels: map[string]string{
				ResourceGraphDefinitionNameLabel: "my-rgd",
				InstanceLabel:                    "my-instance",
				InstanceIDLabel:                  "abc-123",
			},
			wantErr: false,
		},
		{
			name:    "nil labels",
			labels:  nil,
			wantErr: true,
		},
		{
			name:    "empty labels",
			labels:  map[string]string{},
			wantErr: true,
		},
		{
			name: "missing RGD name label",
			labels: map[string]string{
				InstanceLabel:   "my-instance",
				InstanceIDLabel: "abc-123",
			},
			wantErr: true,
		},
		{
			name: "missing instance label",
			labels: map[string]string{
				ResourceGraphDefinitionNameLabel: "my-rgd",
				InstanceIDLabel:                  "abc-123",
			},
			wantErr: true,
		},
		{
			name: "missing instance ID label",
			labels: map[string]string{
				ResourceGraphDefinitionNameLabel: "my-rgd",
				InstanceLabel:                    "my-instance",
			},
			wantErr: true,
		},
		{
			name: "empty value treated as missing",
			labels: map[string]string{
				ResourceGraphDefinitionNameLabel: "",
				InstanceLabel:                    "my-instance",
				InstanceIDLabel:                  "abc-123",
			},
			wantErr: true,
		},
		{
			name: "extra labels ignored",
			labels: map[string]string{
				ResourceGraphDefinitionNameLabel: "my-rgd",
				InstanceLabel:                    "my-instance",
				InstanceIDLabel:                  "abc-123",
				"extra-label":                    "extra-value",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateInstanceLabels(tt.labels)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateInstanceLabels() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
