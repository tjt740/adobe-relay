//go:build unit

package admin

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAdobeAlertRuleValidation(t *testing.T) {
	for _, tc := range []struct {
		name, filters string
		valid         bool
	}{
		{"recipient", `{"email_recipient":"owner@example.com","platform":"adobe"}`, true},
		{"global", `{}`, true},
		{"invalid recipient", `{"email_recipient":"not-an-email"}`, false},
		{"wrong type", `{"email_recipient":["owner@example.com"]}`, false},
		{"multiple recipients", `{"email_recipient":"a@example.com,b@example.com"}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var raw map[string]json.RawMessage
			require.NoError(t, json.Unmarshal([]byte(`{"name":"Adobe errors","metric_type":"adobe_401_408_accounts","operator":">=","threshold":5,"window_minutes":10,"filters":`+tc.filters+`}`), &raw))
			v, err := validateOpsAlertRulePayload(raw)
			if !tc.valid {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, 10, v.WindowMinutes)
			require.EqualValues(t, 5, v.Threshold)
		})
	}
}
