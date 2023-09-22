// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

// Package kbncontent implements routines for analyzing Kibana content.
//
// It provides information about Kibana assets and a single source of truth for what things are legacy and should no longer be used.
package kbncontent

import (
	"encoding/json"

	"github.com/PaesslerAG/jsonpath"
)

// Describes a visualization.
// In this case, "visualization" means anything which can be embedded in a dashboard.
type VisDesc struct {
	Doc      map[string]interface{}
	SoType   string
	Link     string
	Type     string
	Title    string
	IsLegacy bool
}

func isLegacy(soType, visType string) bool {
	return soType == "visualization" && visType != "markdown" && visType != "input_control_vis" && visType != "vega"
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

	// I think this is the dashboard case
	if embeddableType, err := jsonpath.Get("$.embeddableConfig.savedVis.type", doc); err == nil {
		return embeddableType.(string), nil
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

	// I think this is the dashboard case
	if title, err := jsonpath.Get("$.embeddableConfig.savedVis.title", doc); err == nil {
		return title.(string), nil
	}

	return "", nil
}

// Gathers information from within the document and attaches it
func attachMetaInfo(desc *VisDesc) {
	if result, err := getVisType(desc.Doc, desc.SoType); err == nil {
		desc.Type = result
	}

	if result, err := getVisTitle(desc.Doc, desc.SoType); err == nil {
		desc.Title = result
	}

	desc.IsLegacy = isLegacy(desc.SoType, desc.Type)
}

// Report information about a visualization saved object (unmarshalled JSON)
// Supports maps, saved searches, Lens, Vega, and legacy visualizations
func DescribeVisualizationSavedObject(doc map[string]interface{}) (VisDesc, error) {
	soType := doc["type"].(string)

	desc := VisDesc{
		Doc:    doc,
		SoType: soType,
		Link:   "by_reference",
	}

	attachMetaInfo(&desc)

	return desc, nil
}

// Given a dashboard state (unmarshalled JSON), report information about the by-value panels
func DescribeByValueDashboardPanels(panelsJSON interface{}) (visDescriptions []VisDesc, err error) {
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

		// TODO - I ported these checks from JS, but I am not sure if they are necessary
		// also, note that this logic needs to change to support saved searches whenever they become by-value
		filterOut := true
		if _, err := jsonpath.Get("$.embeddableConfig.savedVis", panel); err == nil && panelType == "visualization" {
			filterOut = false
		}

		if _, err := jsonpath.Get("$.embeddableConfig.attributes", panel); err == nil && panelType == "lens" || panelType == "map" {
			filterOut = false
		}

		if !filterOut {
			desc := VisDesc{
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
