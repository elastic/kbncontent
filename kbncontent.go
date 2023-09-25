// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

// Package kbncontent implements routines for analyzing Kibana content.
//
// It provides information about Kibana assets and a single source of truth for what things are legacy and should no longer be used.
package kbncontent

import (
	"encoding/json"
	"errors"
	"strings"

	// TODO - consider fully replacing jsonpath with objx
	"github.com/PaesslerAG/jsonpath"
	"github.com/stretchr/objx"
)

// Describes a visualization.
// In this case, "visualization" means anything which can be embedded in a dashboard.
type VisualizationDescriptor struct {
	Doc    map[string]interface{}
	SoType string
	Link   string

	// root-level visualization type
	// currently empty for Lens
	Type string

	// TSVB visualizations are always type "metrics"
	// this property gives the TSVB sub type (gauge, markdown, etc)
	TSVBType string

	// meant to be a visualization-editor-agnostic name for what
	// kind of visualization this actually is (pie, bar, etc)
	// Note: does not yet support Lens
	SemanticType string

	// name of the visualization editor
	Editor string

	Title    string
	IsLegacy bool
}

func isLegacy(soType, visType string) bool {
	return soType == "visualization" && visType != "markdown" && visType != "input_control_vis" && visType != "vega"
}

func getVisEditor(soType, visType string) string {
	if soType == "lens" {
		return "Lens"
	}

	if soType == "map" {
		return "Maps"
	}

	if soType == "search" {
		return "Discover"
	}

	if soType == "visualization" {
		if visType == "metrics" {
			return "TSVB"
		}

		if visType == "vega" {
			return "Vega"
		}

		if visType == "timelion" {
			return "Timelion"
		}

		return "Aggs-based"
	}

	return "Unknown"
}

func getVisType(doc interface{}, soType string) (string, error) {
	if soType != "visualization" {
		return "", nil
	}

	if attrType, err := jsonpath.Get("$.attributes.type", doc); err == nil {
		return attrType.(string), nil
	}

	if visStateType, err := jsonpath.Get("$.attributes.visState.type", doc); err == nil {
		return visStateType.(string), nil
	}

	// by-value dashboard case
	if embeddableType, err := jsonpath.Get("$.embeddableConfig.savedVis.type", doc); err == nil {
		return embeddableType.(string), nil
	}

	return "", nil
}

func isTSVB(visType string) bool {
	return visType == "metrics"
}

func getTSVBType(doc interface{}, visType string) (string, error) {
	if !isTSVB(visType) {
		return "", nil
	}

	// saved object case
	if result, err := jsonpath.Get("$.attributes.visState.params.type", doc); err == nil {
		return result.(string), nil
	}

	// by-value dashboard panel case
	if result, err := jsonpath.Get("$.embeddableConfig.savedVis.params.type", doc); err == nil {
		return result.(string), nil
	}

	return "", nil
}

func getVisTitle(doc interface{}, soType string) (string, error) {
	if soType != "visualization" {
		return "", nil
	}

	if title, err := jsonpath.Get("$.attributes.title", doc); err == nil {
		return title.(string), nil
	}

	if title, err := jsonpath.Get("$.title", doc); err == nil {
		return title.(string), nil
	}

	// by-value dashboard case
	if title, err := jsonpath.Get("$.embeddableConfig.savedVis.title", doc); err == nil {
		return title.(string), nil
	}

	return "", nil
}

// Attaches domain knowledge as well as information from within the document
func attachMetaInfo(desc *VisualizationDescriptor) {
	if result, err := getVisType(desc.Doc, desc.SoType); err == nil {
		desc.Type = result
	}

	if result, err := getVisTitle(desc.Doc, desc.SoType); err == nil {
		desc.Title = result
	}

	if result, err := getTSVBType(desc.Doc, desc.Type); err == nil {
		desc.TSVBType = result
	}

	if isTSVB(desc.Type) {
		desc.SemanticType = desc.TSVBType
	} else {
		desc.SemanticType = desc.Type
	}

	desc.Editor = getVisEditor(desc.SoType, desc.Type)

	desc.IsLegacy = isLegacy(desc.SoType, desc.Type)
}

func deserializeSubPaths(doc objx.Map) (map[string]interface{}, error) {
	uiState := doc.Get("attributes.uiStateJSON")
	if uiState.IsStr() {
		if parsed, err := objx.FromJSON(uiState.Str()); err == nil {
			doc.Set("attributes.uiStateJSON", parsed)
		}
	}

	visState := doc.Get("attributes.visState")
	if visState.IsStr() {
		if parsed, err := objx.FromJSON(visState.Str()); err == nil {
			doc.Set("attributes.visState", parsed)
		}
	}

	searchSource := doc.Get("attributes.kibanaSavedObjectMeta.searchSourceJSON")
	if searchSource.IsStr() {
		if parsed, err := objx.FromJSON(searchSource.Str()); err == nil {
			doc.Set("attributes.kibanaSavedObjectMeta.searchSourceJSON", parsed)
		}
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

	return doc, nil
}

// Report information about a visualization saved object (unmarshalled JSON)
// Supports maps, saved searches, Lens, Vega, and legacy visualizations
func DescribeVisualizationSavedObject(doc map[string]interface{}) (VisualizationDescriptor, error) {
	doc = objx.New(doc)
	deserializeSubPaths(doc)

	soType := doc["type"].(string)

	desc := VisualizationDescriptor{
		Doc:    doc,
		SoType: soType,
		Link:   "by_reference",
	}

	attachMetaInfo(&desc)

	return desc, nil
}

// Given a dashboard state (unmarshalled JSON), report information about the by-value panels
func DescribeByValueDashboardPanels(panelsJSON interface{}) (visDescriptions []VisualizationDescriptor, err error) {
	var panels []interface{}
	switch panelsJSON.(type) {
	case string:
		json.Unmarshal([]byte(panelsJSON.(string)), &panels)
	case []interface{}:
		panels = panelsJSON.([]interface{})
	}
	for _, panel := range panels {
		panelTypeJSON, err := jsonpath.Get("$.type", panel)

		if err != nil {
			// no type, so by-reference
			continue
		}

		panelType := panelTypeJSON.(string)

		// TODO - I ported these checks from JS, but I do not understand why they are necessary
		// also, note that this logic needs to change to support saved searches whenever they become by-value
		filterOut := true
		if _, err := jsonpath.Get("$.embeddableConfig.savedVis", panel); err == nil && panelType == "visualization" {
			filterOut = false
		}

		if _, err := jsonpath.Get("$.embeddableConfig.attributes", panel); err == nil && panelType == "lens" || panelType == "map" {
			filterOut = false
		}

		if !filterOut {
			desc := VisualizationDescriptor{
				Doc:    panel.(map[string]interface{}),
				SoType: panelType,
				Link:   "by_value",
			}
			attachMetaInfo(&desc)
			visDescriptions = append(visDescriptions, desc)
		}
	}
	return visDescriptions, nil
}

func GetDashboardTitle(dashboard interface{}) (string, error) {
	m := objx.Map(dashboard.(map[string]interface{}))
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
		r, ok := v.(map[string]interface{})
		if !ok {
			return nil, errors.New("conversion error to reference element")
		}
		ref := Reference{
			ID:   r["id"].(string),
			Type: r["type"].(string),
			Name: r["name"].(string),
		}

		refs = append(refs, ref)
	}
	return refs, nil
}

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
