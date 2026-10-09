package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"strings"

	"gopkg.in/yaml.v3"
)

// Entry retains the original integration and its source location for navigation.
type Entry struct {
	Item Item
	File string
	Node *yaml.Node
}

type SelectedPoint struct {
	File        string                 `json:"file"`
	Line        int                    `json:"line"`
	ID          string                 `json:"id"`
	Status      string                 `json:"status"`
	PointIndex  int                    `json:"point_index"`
	Remaining   int                    `json:"remaining_count"`
	Point       interface{}            `json:"point"`
	Integration map[string]interface{} `json:"integration"`
}

func selectPoint(entries []Entry, id string, index int, exclude string, random bool) (SelectedPoint, error) {
	var candidates []SelectedPoint
	foundID := false
	for _, entry := range entries {
		if id != "" && entry.Item.ID != id {
			continue
		}
		if id != "" && foundID {
			return SelectedPoint{}, fmt.Errorf("integration ID %q has multiple owners", id)
		}
		foundID = true
		var integration map[string]interface{}
		if err := entry.Node.Decode(&integration); err != nil {
			return SelectedPoint{}, err
		}
		for i, point := range entry.Item.Remaining {
			if index > 0 && i+1 != index {
				continue
			}
			encoded, err := yaml.Marshal(point)
			if err != nil {
				return SelectedPoint{}, err
			}
			if exclude != "" && strings.Contains(strings.ToLower(string(encoded)), strings.ToLower(exclude)) {
				continue
			}
			line := entry.Node.Line
			for j := 0; j+1 < len(entry.Node.Content); j += 2 {
				if entry.Node.Content[j].Value == "remaining" {
					list := entry.Node.Content[j+1]
					if list.Kind == yaml.SequenceNode && i < len(list.Content) {
						line = list.Content[i].Line
					}
				}
			}
			candidates = append(candidates, SelectedPoint{
				File: entry.File, Line: line, ID: entry.Item.ID, Status: entry.Item.Status,
				PointIndex: i + 1, Remaining: len(entry.Item.Remaining), Point: point, Integration: integration,
			})
		}
	}
	if id != "" && !foundID {
		return SelectedPoint{}, fmt.Errorf("integration ID %q not found", id)
	}
	if len(candidates) == 0 {
		return SelectedPoint{}, fmt.Errorf("no remaining points match the selection")
	}
	if random {
		return candidates[rand.IntN(len(candidates))], nil
	}
	return candidates[0], nil
}

func printPoint(out io.Writer, point SelectedPoint, jsonOutput bool) error {
	if jsonOutput {
		encoder := json.NewEncoder(out)
		encoder.SetIndent("", "  ")
		return encoder.Encode(point)
	}
	encoded, err := yaml.Marshal(point.Point)
	if err != nil {
		return err
	}
	context, err := yaml.Marshal(point.Integration)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(out, "SELECTED REMAINING POINT\nSource: %s:%d\nIntegration: %s\nStatus: %s\nPoint: %d of %d remaining\n\n%s\nIntegration context:\n%s", point.File, point.Line, point.ID, point.Status, point.PointIndex, point.Remaining, encoded, context)
	return err
}
