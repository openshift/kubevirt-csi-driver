package main

import (
	"errors"
	"testing"

	"k8s.io/apimachinery/pkg/types"
	kubevirtv1 "kubevirt.io/api/core/v1"
)

type fakeInfraVMGetter struct {
	vmi       *kubevirtv1.VirtualMachineInstance
	err       error
	calls     int
	namespace string
	name      string
}

func (f *fakeInfraVMGetter) GetVirtualMachine(namespace, name string) (*kubevirtv1.VirtualMachineInstance, error) {
	f.calls++
	f.namespace = namespace
	f.name = name
	return f.vmi, f.err
}

func TestResolveNodeID(t *testing.T) {
	firmwareUUID := "11111111-2222-3333-4444-555555555555"
	annotatedVM := &kubevirtv1.VirtualMachineInstance{
		Spec: kubevirtv1.VirtualMachineInstanceSpec{
			Domain: kubevirtv1.DomainSpec{
				Firmware: &kubevirtv1.Firmware{
					UUID: types.UID(firmwareUUID),
				},
			},
		},
	}
	completeAnnotations := map[string]string{
		"csi.kubevirt.io/infra-vm-name":      "annotated-vm",
		"csi.kubevirt.io/infra-vm-namespace": "annotated-namespace",
	}

	tests := []struct {
		name        string
		systemUUID  string
		annotations map[string]string
		vmi         *kubevirtv1.VirtualMachineInstance
		getErr      error
		wantNodeID  string
		wantErr     bool
		wantCalls   int
	}{
		{
			name:       "systemUUID is preferred",
			systemUUID: "system-uuid",
			annotations: map[string]string{
				"csi.kubevirt.io/infra-vm-name":      "ignored-vm",
				"csi.kubevirt.io/infra-vm-namespace": "ignored-namespace",
			},
			wantNodeID: "system-uuid",
			wantCalls:  0,
		},
		{
			name:        "annotations resolve the infra VM firmware UUID",
			annotations: completeAnnotations,
			vmi:         annotatedVM,
			wantNodeID:  firmwareUUID,
			wantCalls:   1,
		},
		{
			name: "missing annotation returns an error",
			annotations: map[string]string{
				"csi.kubevirt.io/infra-vm-namespace": "annotated-namespace",
			},
			wantErr:   true,
			wantCalls: 0,
		},
		{
			name:        "infra VM lookup failure returns an error",
			annotations: completeAnnotations,
			getErr:      errors.New("lookup failed"),
			wantErr:     true,
			wantCalls:   1,
		},
		{
			name:        "infra VM without firmware UUID returns an error",
			annotations: completeAnnotations,
			vmi:         &kubevirtv1.VirtualMachineInstance{},
			wantErr:     true,
			wantCalls:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getter := &fakeInfraVMGetter{vmi: tt.vmi, err: tt.getErr}

			gotNodeID, err := resolveNodeID("worker-0", tt.systemUUID, tt.annotations, getter)

			if (err != nil) != tt.wantErr {
				t.Fatalf("resolveNodeID() error = %v, wantErr %v", err, tt.wantErr)
			}
			if gotNodeID != tt.wantNodeID {
				t.Errorf("resolveNodeID() = %v, want %v", gotNodeID, tt.wantNodeID)
			}
			if getter.calls != tt.wantCalls {
				t.Errorf("infra VM getter calls = %d, want %d", getter.calls, tt.wantCalls)
			}
			if tt.wantCalls == 1 {
				if getter.namespace != "annotated-namespace" || getter.name != "annotated-vm" {
					t.Errorf("infra VM lookup = %s/%s, want annotated-namespace/annotated-vm", getter.namespace, getter.name)
				}
			}
		})
	}
}
