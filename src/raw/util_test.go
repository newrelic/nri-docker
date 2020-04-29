package raw

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestDetectHostRoot(t *testing.T) {

	testCases := []struct {
		name          string
		hostRoot      string
		existingPaths []string
		expected      string
		expectedErr   error
	}{
		{
			name:     "Empty_HostRoot_OnHost",
			hostRoot: "",
			existingPaths: []string{
				"/proc",
			},
			expected:    "/",
			expectedErr: nil,
		},
		{
			name:     "Empty_HostRoot_OnContainer",
			hostRoot: "",
			existingPaths: []string{
				"/host/proc",
			},
			expected:    "/host",
			expectedErr: nil,
		},
		{
			name:     "Empty_HostRoot_OnContainer_Precedence",
			hostRoot: "",
			existingPaths: []string{
				"/host/proc",
				"/proc",
			},
			expected:    "/host",
			expectedErr: nil,
		},
		{
			name:     "Empty_HostRoot_Error",
			hostRoot: "",
			existingPaths: []string{
				"/host/test/proc",
			},
			expected:    "",
			expectedErr: errHostRootNotFound,
		},
		{
			name:     "Custom_HostRoot_OnContainer",
			hostRoot: "/custom",
			existingPaths: []string{
				"/host/proc",
				"/custom/proc",
				"/proc",
			},
			expected:    "/custom",
			expectedErr: nil,
		},
		{
			name:     "Custom_HostRoot_NotFound_OnHost",
			hostRoot: "/custom",
			existingPaths: []string{
				"/proc",
			},
			expected:    "/",
			expectedErr: nil,
		},
		{
			name:     "Custom_HostRoot_NotFound_OnHost",
			hostRoot: "/custom",
			existingPaths: []string{
				"/host/test/proc",
			},
			expected:    "",
			expectedErr: errHostRootNotFound,
		},
		{
			name:     "Custom_HostRoot_Root",
			hostRoot: "/",
			existingPaths: []string{
				"/proc",
			},
			expected:    "/",
			expectedErr: nil,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			actual, actualErr := DetectHostRoot(testCase.hostRoot, func(dir string) bool {
				for _, existingPath := range testCase.existingPaths {
					if existingPath == dir {
						return true
					}
				}
				return false
			})

			assert.Equal(t, testCase.expected, actual)
			assert.Equal(t, testCase.expectedErr, actualErr)
		})
	}
}
