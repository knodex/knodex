// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"net/http"

	"github.com/knodex/knodex/server/internal/api/response"
	kroadapter "github.com/knodex/knodex/server/internal/kro/graph"
	"github.com/knodex/knodex/server/internal/kro/parser"
	"github.com/knodex/knodex/server/internal/kro/watcher"
	"github.com/knodex/knodex/server/internal/models"
)

// ResourceHandler handles RGD resource graph HTTP requests
type ResourceHandler struct {
	rgdWatcher     *watcher.RGDWatcher
	resourceParser *parser.ResourceParser
}

// NewResourceHandler creates a new resource handler
func NewResourceHandler(rgdWatcher *watcher.RGDWatcher) *ResourceHandler {
	return &ResourceHandler{
		rgdWatcher:     rgdWatcher,
		resourceParser: parser.NewResourceParser(),
	}
}

// ResourceGraphResponse represents the internal resource graph of an RGD
type ResourceGraphResponse struct {
	RGDName          string                 `json:"rgdName"`
	RGDNamespace     string                 `json:"rgdNamespace"`
	Resources        []ResourceNodeResponse `json:"resources"`
	Edges            []ResourceEdgeResponse `json:"edges"`
	TopologicalOrder []string               `json:"topologicalOrder,omitempty"`
	ParseErrors      []parser.ParseError    `json:"parseErrors,omitempty"`
}

// ResourceNodeResponse represents a resource node in the resource graph
type ResourceNodeResponse struct {
	ID            string               `json:"id"`
	APIVersion    string               `json:"apiVersion"`
	Kind          string               `json:"kind"`
	IsTemplate    bool                 `json:"isTemplate"`
	IsConditional bool                 `json:"isConditional"`
	ConditionExpr string               `json:"conditionExpr,omitempty"`
	DependsOn     []string             `json:"dependsOn"`
	ExternalRef   *ExternalRefResponse `json:"externalRef,omitempty"`
	IsCollection  bool                 `json:"isCollection"`
	ForEach       []parser.Iterator    `json:"forEach,omitempty"`
	ReadyWhen     []string             `json:"readyWhen,omitempty"`
}

// RuntimeGraphNode embeds definition node fields and adds live collection runtime state.
type RuntimeGraphNode struct {
	ResourceNodeResponse
	CollectionStatus *models.CollectionStatus `json:"collectionStatus"`
}

// RuntimeGraphResponse is the response for the instance runtime graph endpoint.
type RuntimeGraphResponse struct {
	RGDName      string                 `json:"rgdName"`
	RGDNamespace string                 `json:"rgdNamespace"`
	Resources    []RuntimeGraphNode     `json:"resources"`
	Edges        []ResourceEdgeResponse `json:"edges"`
	ParseErrors  []parser.ParseError    `json:"parseErrors,omitempty"`
}

// ExternalRefResponse represents external reference information
type ExternalRefResponse struct {
	APIVersion     string `json:"apiVersion"`
	Kind           string `json:"kind"`
	NameExpr       string `json:"nameExpr"`
	NamespaceExpr  string `json:"namespaceExpr,omitempty"`
	UsesSchemaSpec bool   `json:"usesSchemaSpec"`
	SchemaField    string `json:"schemaField,omitempty"`
}

// ResourceEdgeResponse represents an edge in the resource graph
type ResourceEdgeResponse struct {
	From string `json:"from"`
	To   string `json:"to"`
	Type string `json:"type"`
}

// buildDefinitionNodeResponse converts a parser.ResourceDefinition to a ResourceNodeResponse
// including all collection metadata fields (IsCollection, ForEach, ReadyWhen).
// Used by GetDefinitionGraph and GetInstanceGraph.
func buildDefinitionNodeResponse(res parser.ResourceDefinition) ResourceNodeResponse {
	node := ResourceNodeResponse{
		ID:            res.ID,
		APIVersion:    res.APIVersion,
		Kind:          res.Kind,
		IsTemplate:    res.IsTemplate,
		IsConditional: res.IncludeWhen != nil,
		DependsOn:     res.DependsOn,
		IsCollection:  res.IsCollection,
		ForEach:       res.ForEach,
		ReadyWhen:     res.ReadyWhen,
	}
	if res.IncludeWhen != nil {
		node.ConditionExpr = res.IncludeWhen.Expression
	}
	if res.ExternalRef != nil {
		node.ExternalRef = &ExternalRefResponse{
			APIVersion:     res.ExternalRef.APIVersion,
			Kind:           res.ExternalRef.Kind,
			NameExpr:       res.ExternalRef.NameExpr,
			NamespaceExpr:  res.ExternalRef.NamespaceExpr,
			UsesSchemaSpec: res.ExternalRef.UsesSchemaSpec,
			SchemaField:    res.ExternalRef.SchemaField,
		}
	}
	return node
}

// buildEdgeResponses converts a slice of parser.ResourceEdge to response DTOs.
func buildEdgeResponses(edges []parser.ResourceEdge) []ResourceEdgeResponse {
	result := make([]ResourceEdgeResponse, len(edges))
	for i, edge := range edges {
		result[i] = ResourceEdgeResponse{
			From: edge.From,
			To:   edge.To,
			Type: edge.Type,
		}
	}
	return result
}

// getResourceGraph returns a ResourceGraph for the given RGD, preferring the
// cached KRO graph (via adapter) and falling back to the lightweight parser.
// Also returns the topological order when available from the KRO graph.
func (h *ResourceHandler) getResourceGraph(rgd *models.CatalogRGD) (*parser.ResourceGraph, []string, error) {
	// Try cached KRO graph first
	if g := h.rgdWatcher.GetGraph(rgd.Namespace, rgd.Name); g != nil {
		adapter := kroadapter.NewUIGraphAdapter(g)
		rg := adapter.GetResourceGraph(rgd.Name, rgd.Namespace, rgd.RawSpec)
		topoOrder := adapter.GetTopologicalOrder()
		return rg, topoOrder, nil
	}

	// Fallback to lightweight parser
	rg, err := h.resourceParser.ParseRGDResources(rgd.Name, rgd.Namespace, rgd.RawSpec)
	return rg, nil, err
}

// GetResourceGraph handles GET /api/v1/rgds/{name}/resources
// @Summary Get RGD internal resource graph
// @Description Returns the internal resources (templates and externalRefs) within an RGD.
// @Tags resources
// @Accept json
// @Produce json
// @Param name path string true "RGD name"
// @Param namespace query string false "Namespace (optional)"
// @Success 200 {object} ResourceGraphResponse
// @Failure 404 {object} api.ErrorResponse
// @Failure 503 {object} api.ErrorResponse
// @Router /api/v1/rgds/{name}/resources [get]
func (h *ResourceHandler) GetResourceGraph(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	if name == "" {
		response.BadRequest(w, "name is required", map[string]string{"name": "path parameter is required"})
		return
	}

	if h.rgdWatcher == nil {
		response.ServiceUnavailable(w, "RGD watcher not available")
		return
	}

	// Get optional namespace from query param
	namespace := r.URL.Query().Get("namespace")

	var rgd *models.CatalogRGD
	var found bool

	if namespace != "" {
		rgd, found = h.rgdWatcher.GetRGD(namespace, name)
	} else {
		rgd, found = h.rgdWatcher.GetRGDByName(name)
	}

	if !found || rgd == nil {
		response.NotFound(w, "RGD", name)
		return
	}

	resourceGraph, topoOrder, err := h.getResourceGraph(rgd)
	if err != nil {
		response.InternalError(w, "failed to parse RGD resources: "+err.Error())
		return
	}

	resp := ResourceGraphResponse{
		RGDName:          resourceGraph.RGDName,
		RGDNamespace:     resourceGraph.RGDNamespace,
		Resources:        make([]ResourceNodeResponse, len(resourceGraph.Resources)),
		TopologicalOrder: topoOrder,
		ParseErrors:      resourceGraph.ParseErrors,
	}

	for i, res := range resourceGraph.Resources {
		resp.Resources[i] = buildDefinitionNodeResponse(res)
	}

	resp.Edges = buildEdgeResponses(resourceGraph.Edges)

	response.WriteJSON(w, http.StatusOK, resp)
}

// GetDefinitionGraph handles GET /api/v1/rgds/{name}/graph
// Returns the full definition graph for the RGD, including forEach collection metadata
// (IsCollection, ForEach, ReadyWhen). Both this endpoint and GetResourceGraph (/resources)
// now return the same shape; /graph is the preferred endpoint for the graph visualization UI.
func (h *ResourceHandler) GetDefinitionGraph(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	if name == "" {
		response.BadRequest(w, "name is required", map[string]string{"name": "path parameter is required"})
		return
	}

	if h.rgdWatcher == nil {
		response.ServiceUnavailable(w, "RGD watcher not available")
		return
	}

	namespace := r.URL.Query().Get("namespace")

	var rgd *models.CatalogRGD
	var found bool

	if namespace != "" {
		rgd, found = h.rgdWatcher.GetRGD(namespace, name)
	} else {
		rgd, found = h.rgdWatcher.GetRGDByName(name)
	}

	if !found || rgd == nil {
		response.NotFound(w, "RGD", name)
		return
	}

	resourceGraph, topoOrder, err := h.getResourceGraph(rgd)
	if err != nil {
		response.InternalError(w, "failed to parse RGD resources: "+err.Error())
		return
	}

	resp := ResourceGraphResponse{
		RGDName:          resourceGraph.RGDName,
		RGDNamespace:     resourceGraph.RGDNamespace,
		Resources:        make([]ResourceNodeResponse, len(resourceGraph.Resources)),
		TopologicalOrder: topoOrder,
		ParseErrors:      resourceGraph.ParseErrors,
	}

	for i, res := range resourceGraph.Resources {
		resp.Resources[i] = buildDefinitionNodeResponse(res)
	}

	resp.Edges = buildEdgeResponses(resourceGraph.Edges)

	response.WriteJSON(w, http.StatusOK, resp)
}
