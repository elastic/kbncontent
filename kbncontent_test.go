package kbncontent

import (
	"encoding/json"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDescribeByValueDashboardPanels(t *testing.T) {
	expected := []struct {
		title    string
		editor   string
		legacy   bool
		soType   string
		visType  string
		tsvbType string
	}{
		{title: "Legacy input control vis", editor: "Aggs-based", legacy: true, soType: "visualization", visType: "input_control_vis", tsvbType: ""},
		{title: "", editor: "Aggs-based", legacy: false, soType: "visualization", visType: "markdown", tsvbType: ""},
		{title: "", editor: "Lens", legacy: false, soType: "lens", visType: "", tsvbType: ""},
		{title: "Vega time series", editor: "Vega", legacy: false, soType: "visualization", visType: "vega", tsvbType: ""},
		{title: "", editor: "Maps", legacy: false, soType: "map", visType: "", tsvbType: ""},
		{title: "TSVB Markdown", editor: "TSVB", legacy: false, soType: "visualization", visType: "metrics", tsvbType: "markdown"},
		{title: "", editor: "Aggs-based", legacy: false, soType: "visualization", visType: "markdown", tsvbType: ""},
		{title: "", editor: "Aggs-based", legacy: false, soType: "visualization", visType: "markdown", tsvbType: ""},
		{title: "TSVB time series", editor: "TSVB", legacy: true, soType: "visualization", visType: "metrics", tsvbType: "timeseries"},
		{title: "TSVB gauge", editor: "TSVB", legacy: true, soType: "visualization", visType: "metrics", tsvbType: "gauge"},
		{title: "", editor: "Aggs-based", legacy: false, soType: "visualization", visType: "markdown", tsvbType: ""},
		{title: "Aggs-based table", editor: "Aggs-based", legacy: true, soType: "visualization", visType: "table", tsvbType: ""},
		{title: "Aggs-based tag cloud", editor: "Aggs-based", legacy: true, soType: "visualization", visType: "tagcloud", tsvbType: ""},
		{title: "", editor: "Aggs-based", legacy: true, soType: "visualization", visType: "heatmap", tsvbType: ""},
		{title: "Timelion time series", editor: "Timelion", legacy: true, soType: "visualization", visType: "timelion", tsvbType: ""},
	}

	content, err := ioutil.ReadFile("./testdata/dashboard.json")
	if err != nil {
		t.Fatalf("Couldn't open test data: %v", err)
	}

	var dashboard interface{}
	err = json.Unmarshal(content, &dashboard)
	if err != nil {
		t.Fatalf("Couldn't parse JSON: %v", err)
	}

	descriptions, err := DescribeByValueDashboardPanels(dashboard)

	if err != nil {
		t.Fatalf("Encountered error during subject execution: %v", err)
	}

	assert.Equal(t, len(descriptions), 15, "The number of panels should be correct")
	for i, desc := range descriptions {
		title := desc.Title()
		var editor string
		if result, err := desc.Editor(); err == nil {
			editor = result
		}

		// Properties
		assert.Equalf(t, desc.Link, "by_value", "Link should be \"by_value\" in \"%s\" (%s)", title, editor)
		assert.Equalf(t, desc.SavedObjectType, expected[i].soType, "SavedObjectType should match expected in \"%s\" (%s)", title, editor)

		// Methods
		assert.Equalf(t, title, expected[i].title, "Title() should match expected in \"%s\" (%s)")
		assert.Equalf(t, editor, expected[i].editor, "Editor() should match expected in \"%s\" (%s)")
		assert.Equalf(t, desc.IsLegacy(), expected[i].legacy, "IsLegacy() should match expected in \"%s\" (%s)", title, editor)
		assert.Equalf(t, desc.Type(), expected[i].visType, "Type() should match expected in \"%s\" (%s)", title, editor)
		assert.Equalf(t, desc.TSVBType(), expected[i].tsvbType, "TSVBType() should match expected in \"%s\" (%s)", title, editor)
	}
}
