package metrics

import (
	"strings"
	"testing"
)

func TestNamespaceMatcher(t *testing.T) {
	for name, tc := range map[string]struct {
		include, exclude []string
		want             string
	}{
		"None":    {nil, nil, ""},
		"Include": {[]string{"a", "b"}, nil, `namespace=~"a|b"`},
		"Exclude": {nil, []string{"kube-system"}, `namespace!~"kube-system"`},
		"Quoted":  {[]string{"a.b"}, nil, `namespace=~"a\\.b"`},
	} {
		t.Run(name, func(t *testing.T) {
			if got := NamespaceMatcher(tc.include, tc.exclude); got != tc.want {
				t.Errorf("NamespaceMatcher() = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestPodQueries(t *testing.T) {
	pms := &PromMetricSync{namespaceMatcher: `namespace=~"a"`}

	for _, q := range pms.podQueries() {
		if strings.Contains(q.query, namespaceMatcherPlaceholder) {
			t.Errorf("%s: placeholder not replaced: %s", q.metricCategory, q.query)
		}

		if !strings.Contains(q.query, `namespace=~"a"`) {
			t.Errorf("%s: not restricted to namespace: %s", q.metricCategory, q.query)
		}
	}

	if !strings.Contains(promQueriesPod[0].query, namespaceMatcherPlaceholder) {
		t.Error("podQueries() modified promQueriesPod")
	}
}
