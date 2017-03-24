package validation

import (
	"testing"

	roles "github.com/hpcloud/fissile/model"
	"github.com/stretchr/testify/assert"
)

func TestValidateRoleRun(t *testing.T) {
	newRun := func(memory int, cpu int, name string, externalPort string, internalPort string, protocol string) *roles.RoleRun {
		return &roles.RoleRun{
			VirtualCPUs: cpu,
			Memory:      memory,
			ExposedPorts: []*roles.RoleRunExposedPort{{Name: name, External: externalPort,
				Internal: internalPort, Protocol: protocol,
			}},
		}
	}

	var (
		validRun      = newRun(10, 2, "test", "1", "2", "UDP")
		wrongProtocol = newRun(10, 2, "test", "1", "2", "AA")
		wrongPorts    = newRun(10, 2, "test", "0", "-1", "UDP")
		wrongParse    = newRun(10, 2, "test", "0", "qq", "UDP")
		negativeField = newRun(-10, 2, "test", "1", "2", "UDP")
	)

	tests := map[string]struct {
		run            *roles.RoleRun
		isValid        bool
		expectedErrors string
	}{
		"nil":            {nil, false, "run: Required value"},
		"valid":          {validRun, true, ``},
		"wrong protocol": {wrongProtocol, false, "run.exposed-ports[test].protocol: Unsupported value: \"AA\": supported values: TCP, UDP"},
		"wrong ports": {wrongPorts, false, "run.exposed-ports[test].external: Invalid value: 0: must be between 1 and 65535, inclusive\n" +
			"run.exposed-ports[test].internal: Invalid value: -1: must be between 1 and 65535, inclusive"},
		"wrong parse": {wrongParse, false, "run.exposed-ports[test].external: Invalid value: 0: must be between 1 and 65535, inclusive\n" +
			"run.exposed-ports[test].internal: Invalid value: \"qq\": invalid syntax"},
		"negative field": {negativeField, false, `run.memory: Invalid value: -10: must be greater than or equal to 0`},
	}

	for name, tc := range tests {
		errs := ValidateRoleRun(tc.run)
		if tc.isValid && len(errs) > 0 {
			t.Errorf("%v: unexpected error: %v", name, errs)
		}
		if !tc.isValid && len(errs) == 0 {
			t.Errorf("%v: unexpected non-error", name)
		}
		if !tc.isValid && len(errs) > 0 {
			assert.Equal(t, tc.expectedErrors, errs.Errors())
		}
	}
}
