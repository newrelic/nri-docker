package aws

import "testing"

func TestProcessFargateLabels(t *testing.T) {
	labels := processFargateLabels(map[string]string{
		"com.amazonaws.ecs.cluster":                 "arn:aws:ecs:my-region:100000000000:cluster/the-cluster-name",
		"com.amazonaws.ecs.task-arn":                "arn:aws:ecs:my-region:100000000000:task/the-cluster-name/f05a5672397746638bc201a252c5bb75",
		"com.amazonaws.ecs.task-definition-family":  "the-task-definition-family",
		"com.amazonaws.ecs.task-definition-version": "the-task-definition-version",
	})

	expected := map[string]string{
		"com.amazonaws.ecs.cluster":           "the-cluster-name",
		"com.amazonaws.ecs.task-arn":          "arn:aws:ecs:my-region:100000000000:task/the-cluster-name/f05a5672397746638bc201a252c5bb75",
		"com.newrelic.nri-docker.cluster-arn": "arn:aws:ecs:my-region:100000000000:cluster/the-cluster-name",
		"com.newrelic.nri-docker.aws-region":  "my-region",
		"com.newrelic.nri-docker.launch-type": "fargate", // Set to fargate as 'com.amazonaws.ecs.cluster' is an arn
	}

	for eK, eV := range expected {
		if v := labels[eK]; v != eV {
			t.Fatalf("expected label %s to be '%s', found '%s' instead", eK, eV, v)
		}
	}
}
