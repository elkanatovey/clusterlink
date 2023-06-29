package policytypes_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"github.ibm.com/mbg-agent/pkg/policyEngine/policytypes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	trivialLabel       = map[string]string{"key": "val"}
	trivialSelector    = metav1.LabelSelector{MatchLabels: trivialLabel}
	trivialWorkloadSet = policytypes.WorkloadSetOrSelector{WorkloadSelector: &trivialSelector}
)

func TestMatching(t *testing.T) {
	emptyConnPol := policytypes.ConnectivityPolicy{} // should match nothing
	matches, err := emptyConnPol.Matches(map[string]string{}, map[string]string{})
	require.Nil(t, err)
	require.False(t, matches)

	trivialConnPol := policytypes.ConnectivityPolicy{From: []policytypes.WorkloadSetOrSelector{trivialWorkloadSet}}
	matches, err = trivialConnPol.Matches(trivialLabel, trivialLabel)
	require.Nil(t, err)
	require.False(t, matches) // no To field - should still match nothing

	trivialConnPol.To = []policytypes.WorkloadSetOrSelector{trivialWorkloadSet}
	matches, err = trivialConnPol.Matches(trivialLabel, trivialLabel)
	require.Nil(t, err)
	require.True(t, matches) // From and To and set - there is a match now
}

func TestMarshall(t *testing.T) {
	trivialConnPol := policytypes.ConnectivityPolicy{
		Action: policytypes.PolicyActionAllow,
		From:   []policytypes.WorkloadSetOrSelector{trivialWorkloadSet},
		To:     []policytypes.WorkloadSetOrSelector{trivialWorkloadSet},
	}
	b, err := json.Marshal(trivialConnPol)
	require.Nil(t, err)
	expected := `{` +
		`"name":"",` +
		`"privileged":false,` +
		`"action":"allow",` +
		`"from":[{"workloadSelector":{"matchLabels":{"key":"val"}}}],` +
		`"to":[{"workloadSelector":{"matchLabels":{"key":"val"}}}]}`
	require.Equal(t, expected, string(b))

	trivialConnPol.Name = "trivialPol"
	b, err = json.Marshal(trivialConnPol)
	require.Nil(t, err)
	expected = `{` +
		`"name":"trivialPol",` +
		`"privileged":false,` +
		`"action":"allow",` +
		`"from":[{"workloadSelector":{"matchLabels":{"key":"val"}}}],` +
		`"to":[{"workloadSelector":{"matchLabels":{"key":"val"}}}]}`
	require.Equal(t, expected, string(b))
}

func TestBadSelector(t *testing.T) {
	badLabel := map[string]string{"bad key": "bad value!@#$%^&*()"}
	badSelector := metav1.LabelSelector{MatchLabels: badLabel}
	badWorkloadSet := policytypes.WorkloadSetOrSelector{WorkloadSelector: &badSelector}
	badPolicy := policytypes.ConnectivityPolicy{
		Action: policytypes.PolicyActionDeny,
		From:   []policytypes.WorkloadSetOrSelector{badWorkloadSet},
		To:     []policytypes.WorkloadSetOrSelector{badWorkloadSet},
	}
	err := badPolicy.Validate()
	require.NotNil(t, err)
	_, err = badPolicy.Matches(nil, nil)
	require.NotNil(t, err)

	anotherBadPolicy := policytypes.ConnectivityPolicy{
		Action: policytypes.PolicyActionDeny,
		From:   []policytypes.WorkloadSetOrSelector{trivialWorkloadSet},
		To:     []policytypes.WorkloadSetOrSelector{badWorkloadSet},
	}
	err = anotherBadPolicy.Validate()
	require.NotNil(t, err)
	_, err = anotherBadPolicy.Matches(trivialLabel, nil)
	require.NotNil(t, err)

	emptySelector := policytypes.WorkloadSetOrSelector{}
	anotherBadPolicy.To = []policytypes.WorkloadSetOrSelector{emptySelector}
	err = anotherBadPolicy.Validate()
	require.NotNil(t, err)

	notYetSupportedSelector := policytypes.WorkloadSetOrSelector{WorkloadSets: []string{"a-set"}}
	notYetSupportedPolicy := policytypes.ConnectivityPolicy{
		Action: policytypes.PolicyActionDeny,
		From:   []policytypes.WorkloadSetOrSelector{trivialWorkloadSet},
		To:     []policytypes.WorkloadSetOrSelector{notYetSupportedSelector},
	}
	err = notYetSupportedPolicy.Validate()
	require.NotNil(t, err)
}

func TestValidation(t *testing.T) {
	badPolicy := policytypes.ConnectivityPolicy{}
	err := badPolicy.Validate()
	require.NotNil(t, err) // action is an empty string

	badPolicy.Action = "notAnAction"
	err = badPolicy.Validate()
	require.NotNil(t, err) // action is not a legal action

	badPolicy.Action = "deny"
	err = badPolicy.Validate()
	require.NotNil(t, err) // missing from

	badPolicy.From = []policytypes.WorkloadSetOrSelector{trivialWorkloadSet}
	err = badPolicy.Validate()
	require.NotNil(t, err) // missing to

	badPolicy.To = []policytypes.WorkloadSetOrSelector{trivialWorkloadSet}
	err = badPolicy.Validate()
	require.Nil(t, err)
}
