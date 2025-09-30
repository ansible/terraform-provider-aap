package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
)

func TestIsContextActive(t *testing.T) {
	var testTable = []struct {
		testName            string
		shouldCancelContext bool
		diagsShouldHaveErr  bool
		expectedReturnValue bool
	}{
		{
			testName:            "context active",
			shouldCancelContext: false,
			diagsShouldHaveErr:  false,
			expectedReturnValue: true,
		},
		{
			testName:            "context complete",
			shouldCancelContext: true,
			diagsShouldHaveErr:  true,
			expectedReturnValue: false,
		},
	}

	for _, test := range testTable {
		t.Run("test_"+test.testName, func(t *testing.T) {
			var diags = diag.Diagnostics{}
			ctx, cancel := context.WithCancel(context.TODO())
			defer cancel()

			if test.shouldCancelContext {
				cancel()
			}

			returnValue := IsContextActive(test.testName, ctx, &diags)

			if returnValue != test.expectedReturnValue {
				t.Errorf("Got an unexpected return value. got: %t, expected: %t", returnValue, test.expectedReturnValue)
				return
			}

			if test.diagsShouldHaveErr {
				if !diags.HasError() {
					t.Errorf("Expected error but received none.")
				}
			}
		})
	}
}
