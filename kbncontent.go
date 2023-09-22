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
	VisType  string
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

	if embeddableType, err := jsonpath.Get("$.embeddableConfig.savedVis.type", doc); err == nil {
		return embeddableType.(string), nil
	}

	return "", nil
}

// Report information about a visualization saved object (unmarshalled JSON)
// Supports maps, saved searches, Lens, Vega, and legacy visualizations
func DescribeVisualizationSavedObject(doc map[string]interface{}) (VisDesc, error) {
	soType := doc["type"].(string)

	var visType string

	if result, err := getVisType(doc, soType); err == nil {
		visType = result
	}

	return VisDesc{
		Doc: doc,
		SoType: soType,
		Link: "by_reference",
		VisType: visType,
		IsLegacy: isLegacy(soType, visType),
	}, nil
}

// Given a dashboard state (unmarshalled JSON), report information about the by-value panels
func DescribeByValueDashboardPanels(panelsJSON interface{}) (panelInfos []VisDesc, err error) {
	var panels []interface{}
	switch panelsJSON.(type) {
	case string:
		json.Unmarshal([]byte(panelsJSON.(string)), &panels)
	case []interface{}:
		panels = panelsJSON.([]interface{})
	}
	for _, panel := range panels {
		panelMap := panel.(map[string]interface{})

		switch panelType := panelMap["type"].(type) {
		default:
			// No op. There is no panel type, so this is by-reference.

		case string:
			switch panelType {
			case "visualization":
				embeddableConfig := panelMap["embeddableConfig"].(map[string]interface{})
				if _, ok := embeddableConfig["savedVis"]; ok {
					var visType string

					if result, err := getVisType(panelMap, panelType); err == nil {
						visType = result
					}

					panelInfos = append(panelInfos, VisDesc{
						Doc:      panelMap,
						SoType:   panelType,
						Link:     "by_value",
						VisType:  visType,
						IsLegacy: isLegacy(panelType, visType),
					})
				}
			case "lens", "map":
				embeddableConfig := panelMap["embeddableConfig"].(map[string]interface{})
				if _, ok := embeddableConfig["attributes"]; ok {
					var visType string

					if result, err := getVisType(panelMap, panelType); err == nil {
						visType = result
					}

					panelInfos = append(panelInfos, VisDesc{
						Doc:      panelMap,
						SoType:   panelType,
						Link:     "by_value",
						VisType:  visType,
						IsLegacy: isLegacy(panelType, visType),
					})
				}
			}
		}
	}
	return panelInfos, nil
}
