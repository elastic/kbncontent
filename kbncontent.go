// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

// Package kbncontent implements routines for analyzing Kibana content.
//
// It provides information about Kibana assets and a single source of truth for what things are legacy and should no longer be used.
package kbncontent

import (
	"errors"
	"fmt"
	"strings"

	"github.com/mitchellh/mapstructure"
	"github.com/stretchr/objx"
)

// VisualizationDescriptor describes a visualization.
// In this case, "visualization" means anything which can be embedded in a dashboard.
type VisualizationDescriptor struct {
	Doc             map[string]interface{}
	SavedObjectType string
	Link            string
}

func (v VisualizationDescriptor) findDocumentPathsAsString(paths []string) string {
	m := objx.Map(v.Doc)

	for _, path := range paths {
		if m.Get(path).IsStr() {
			return m.Get(path).Str()
		}
	}

	return ""
}

// Type returns the root-level visualization type
// currently empty for Lens
func (v VisualizationDescriptor) Type() string {
	if v.SavedObjectType != "visualization" {
		return ""
	}

	return v.findDocumentPathsAsString([]string{
		"attributes.type",
		"attributes.visState.type",
		"embeddableConfig.savedVis.type", // by-value dashboard panel
	})
}

// Editor returns the name of the visualization editor
func (v VisualizationDescriptor) Editor() string {
	switch v.SavedObjectType {
	case "lens":
		return "Lens"
	case "map":
		return "Maps"
	case "search":
		return "Discover"
	case "visualization":
		switch v.Type() {
		case "metrics":
			return "TSVB"
		case "vega":
			return "Vega"
		case "timelion":
			return "Timelion"
		default:
			return "Aggs-based"
		}
	}
	return "Unknown"
}

// IsLegacy returns whether the visualization is considered legacy
// legacy visualizations should not be used and will be
// removed from Kibana in the future
func (v VisualizationDescriptor) IsLegacy() bool {
	if v.SavedObjectType != "visualization" {
		return false
	}

	if v.isTSVB() {
		// TSVB markdown is not marked as legacy because
		// we don't yet have a good replacement
		return v.TSVBType() != "markdown"
	}

	switch v.Type() {
	case "markdown", "vega":
		return false
	default:
		return true
	}
}

func (v VisualizationDescriptor) isTSVB() bool {
	return v.Type() == "metrics"
}

// SemanticType is meant to be a visualization-editor-agnostic name for what
// kind of visualization this actually is (pie, bar, etc)
// Note: does not yet support Lens
func (v VisualizationDescriptor) SemanticType() string {
	if v.isTSVB() {
		return v.TSVBType()
	} else {
		return v.Type()
	}
}

// TSVBType returns the TSVB sub type (gauge, markdown, etc)
// TSVB visualizations are always Type "metrics"
func (v VisualizationDescriptor) TSVBType() string {
	if !v.isTSVB() {
		return ""
	}

	return v.findDocumentPathsAsString([]string{
		"attributes.visState.params.type",
		"embeddableConfig.savedVis.params.type", // by-value dashboard panel
	})
}

// Title returns the visualization title
func (v VisualizationDescriptor) Title() string {
	if v.SavedObjectType != "visualization" {
		return ""
	}

	return v.findDocumentPathsAsString([]string{
		"attributes.title",
		"title",
		"embeddableConfig.savedVis.title", // by-value dashboard panel
	})
}

func deserializeSubPaths(doc objx.Map) error {
	jsonPaths := []string{
		"attributes.uiStateJSON",
		"attributes.visState",
		"attributes.kibanaSavedObjectMeta.searchSourceJSON",
	}
	for _, fieldName := range jsonPaths {
		field := doc.Get(fieldName)
		if !field.IsStr() {
			continue
		}
		parsed, err := objx.FromJSON(field.Str())
		if err != nil {
			return fmt.Errorf("failed to decode embedded json in %q: %w", fieldName, err)
		}
		doc.Set(fieldName, parsed)
	}

	/* these transformations from the original script facilitate the vis_tsvb_aggs and other TSVB-related runtime fields
		TODO - implement these or convert the
		vis_tsvb_aggs and any other necessary runtime fields to Go
	if (
	    doc?.attributes?.visState?.params?.filter &&
	    typeof doc.attributes.visState.params.filter !== "string"
	  ) {
		  console.log("ENCOUNTERED STRINGIFIED visState.params.filter STATE", path)
	    doc.attributes.visState.params.filter = JSON.stringify(
	      doc.attributes.visState.params.filter
	    );
	  }
	  if (
	    doc?.attributes?.visState?.params?.series &&
	    Array.isArray(doc.attributes.visState.params.series)
	  ) {
		  console.log("ENCOUNTERED STRINGIFIED visState.params.series STATE", path)
	    doc.attributes.visState.params.series =
	      doc.attributes.visState.params.series.map((s) => ({
	        ...s,
	        filter: JSON.stringify(s.filter),
	      }));
	  }
	*/

	return nil
}

// DescribeVisualizationSavedObject reports information about a visualization saved object (unmarshalled JSON)
// Supports maps, saved searches, Lens, Vega, and legacy visualizations
func DescribeVisualizationSavedObject(doc map[string]interface{}) (VisualizationDescriptor, error) {
	doc = objx.New(doc)
	err := deserializeSubPaths(doc)

	if err != nil {
		return VisualizationDescriptor{}, fmt.Errorf("failed to deserialize embedded JSON objects: %w", err)
	}

	soType, ok := doc["type"].(string)
	if !ok {
		return VisualizationDescriptor{}, errors.New("`type` in visualization is not present or is not a string")
	}

	desc := VisualizationDescriptor{
		Doc:             doc,
		SavedObjectType: soType,
		Link:            "by_reference",
	}

	return desc, nil
}

// DescribeByValueDashboardPanels reports information about the by-value panels given
// dashboard state (unmarshalled JSON)
func DescribeByValueDashboardPanels(dashboard interface{}) (visDescriptions []VisualizationDescriptor, err error) {
	var panelsValue *objx.Value
	if dashboardMap, ok := dashboard.(map[string]interface{}); ok {
		panelsValue = objx.Map(dashboardMap).Get("attributes.panelsJSON")
	} else {
		return nil, fmt.Errorf("dashboard of unexpected type %T", dashboard)
	}

	var panels []objx.Map
	if panelsValue.IsStr() {
		result, err := objx.FromJSONSlice(panelsValue.Str())
		if err != nil {
			return nil, fmt.Errorf("failed to parse panels JSON: %w", err)
		}

		panels = result
	} else if panelsValue.IsObjxMapSlice() {
		panels = panelsValue.ObjxMapSlice()
	} else {
		return nil, fmt.Errorf("panelsJSON is of unexpected type %T. Expected string or map[string]interface{}", err)
	}

	for _, panel := range panels {
		panelTypeValue := panel.Get("type")
		if !panelTypeValue.IsStr() {
			// no type, so by-reference
			continue
		}

		panelType := panelTypeValue.Str()

		// TODO - I ported these checks from JS, but I do not understand why they are necessary
		// also, note that this logic needs to change to support saved searches whenever they become by-value
		filterOut := true

		if panel.Has("embeddableConfig.savedVis") && panelType == "visualization" {
			filterOut = false
		}

		if panel.Has("embeddableConfig.attributes") && (panelType == "lens" || panelType == "map") {
			filterOut = false
		}

		if !filterOut {
			desc := VisualizationDescriptor{
				Doc:             panel,
				SavedObjectType: panelType,
				Link:            "by_value",
			}
			visDescriptions = append(visDescriptions, desc)
		}
	}
	return visDescriptions, nil
}

func GetDashboardTitle(dashboard interface{}) (string, error) {
	dashboardMap, ok := dashboard.(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("dashboard of unexpected type: %T. Expected map[string]interface{}", dashboard)
	}
	m := objx.Map(dashboardMap)
	return m.Get("attributes.title").Str(), nil
}

// A dashboard reference
type Reference struct {
	ID, Type, Name string
}

func toReferenceSlice(val interface{}) ([]Reference, error) {
	vals, ok := val.([]interface{})
	if !ok {
		return nil, errors.New("conversion error to array")
	}
	var refs []Reference
	for _, v := range vals {
		var ref Reference
		err := mapstructure.Decode(v, &ref)
		if err != nil {
			return nil, fmt.Errorf("conversion errror to reference element: %w", err)
		}

		refs = append(refs, ref)
	}
	return refs, nil
}

// GetByReferencePanelIDs returns IDs of saved objects that compose the by-ref panels of the dashboard
func GetByReferencePanelIDs(dashboard interface{}) ([]string, error) {
	allReferences, err := toReferenceSlice(dashboard.(map[string]interface{})["references"])

	if err != nil {
		return nil, err
	}

	var panelIds []string
	for _, ref := range allReferences {
		if strings.Contains(ref.Name, "panel_") {
			panelIds = append(panelIds, ref.ID)
		}
	}
	return panelIds, nil
}
