package v1

import (
	"github.com/icinga/icinga-go-library/types"
	icingav1 "github.com/icinga/icinga-kubernetes-testing/pkg/apis/icinga/v1"
	"github.com/icinga/icinga-kubernetes/pkg/database"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
)

type Test struct {
	Meta
	DeploymentName string
	ActualReplicas int32
	Tests          []TestTest  `db:"-"`
	Labels         []Label     `db:"-"`
	TestLabels     []TestLabel `db:"-"`
}

type TestTest struct {
	TestKind     string
	GoodReplicas int32
	BadReplicas  int32
	TestConfig   string
}

type TestLabel struct {
	TestUuid  types.UUID
	LabelUuid types.UUID
}

func NewTest() Resource {
	return &Test{}
}

func (t *Test) Obtain(k8s kmetav1.Object) {
	t.ObtainMeta(k8s)

	test := k8s.(*icingav1.Test)

	t.Name = test.Name
	t.DeploymentName = test.Spec.DeploymentName
	t.ActualReplicas = test.Status.AvailableReplicas

	for _, tst := range test.Spec.Tests {
		t.Tests = append(t.Tests, TestTest{
			TestKind:     tst.TestKind,
			GoodReplicas: *tst.GoodReplicas,
			BadReplicas:  *tst.BadReplicas,
			TestConfig:   tst.TestConfig,
		})
	}

	for labelName, labelValue := range test.Labels {
		labelUuid := NewUUID(t.Uuid, strings.ToLower(labelName+":"+labelValue))
		t.Labels = append(t.Labels, Label{
			Uuid:  labelUuid,
			Name:  labelName,
			Value: labelValue,
		})
		t.TestLabels = append(t.TestLabels, TestLabel{
			TestUuid:  t.Uuid,
			LabelUuid: labelUuid,
		})
	}
}

func (t *Test) Relations() []database.Relation {
	fk := database.WithForeignKey("test_uuid")

	return []database.Relation{
		database.HasMany(t.TestLabels, fk),
		database.HasMany(t.Labels, database.WithoutCascadeDelete()),
	}
}
