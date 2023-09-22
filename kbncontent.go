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

// A VisInfo describes a visualization.
// In this case, "visualization" means anything which can be embedded in a dashboard.
type VisInfo struct {
	Doc      map[string]interface{}
	SoType   string
	Link     string
	VisType  string
	IsLegacy bool
}

// TODO don't export
func IsLegacy(soType, visType string) bool {
	return soType == "visualization" && visType != "markdown" && visType != "input_control_vis" && visType != "vega"
}

// TODO don't export
func GetVisType(doc interface{}, soType string) (string, error) {
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

// Given a dashboard state (unmarshalled JSON), report information about the by-value panels
func CollectByValueDashboardPanels(panelsJSON interface{}) (panelInfos []VisInfo, err error) {
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

					if result, err := GetVisType(panelMap, panelType); err == nil {
						visType = result
					}

					panelInfos = append(panelInfos, VisInfo{
						Doc:      panelMap,
						SoType:   panelType,
						Link:     "by_value",
						VisType:  visType,
						IsLegacy: IsLegacy(panelType, visType),
					})
				}
			case "lens", "map":
				embeddableConfig := panelMap["embeddableConfig"].(map[string]interface{})
				if _, ok := embeddableConfig["attributes"]; ok {
					var visType string

					if result, err := GetVisType(panelMap, panelType); err == nil {
						visType = result
					}

					panelInfos = append(panelInfos, VisInfo{
						Doc:      panelMap,
						SoType:   panelType,
						Link:     "by_value",
						VisType:  visType,
						IsLegacy: IsLegacy(panelType, visType),
					})
				}
			}
		}
	}
	return panelInfos, nil
}
