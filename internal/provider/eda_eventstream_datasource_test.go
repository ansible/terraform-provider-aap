package provider

import "testing"

func TestNewEDAEventStreamDataSource(t *testing.T) {
	testDataSource := NewEDAEventStreamDataSource()

	expectedMetadataEntitySlug := "eda_eventstream"
	expectedDescriptiveEntityName := "EDA Event Stream"
	expectedApiEntitySlug := "event-streams"

	switch v := testDataSource.(type) {
	case *EDAEventStreamDataSource:
		if v.ApiEntitySlug != expectedApiEntitySlug {
			t.Errorf("Incorrect ApiEntitySlug. Got: %s, wanted: %s", v.ApiEntitySlug, expectedApiEntitySlug)
		}
		if v.DescriptiveEntityName != expectedDescriptiveEntityName {
			t.Errorf("Incorrect DescriptiveEntityName. Got: %s, wanted: %s", v.DescriptiveEntityName, expectedDescriptiveEntityName)
		}
		if v.MetadataEntitySlug != expectedMetadataEntitySlug {
			t.Errorf("Incorrect MetadataEntitySlug. Got: %s, wanted: %s", v.MetadataEntitySlug, expectedMetadataEntitySlug)
		}
	default:
		t.Errorf("Incorrect datasource type returned. Got: %T, wanted: %T", v, testDataSource)
	}
}
