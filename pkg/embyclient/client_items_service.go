package embyclient

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

type GetItemsOptionalParams struct {
	Recursive           *bool
	AnyProviderIdEquals map[string]string
	ParentID            string
	Path                string
	Filters             []string
	IncludeItemTypes    []string
	Limit               int
}

func (c *Client) GetItems(
	ctx context.Context,
	optionalParams *GetItemsOptionalParams,
) (QueryResultBaseItemDto, error) {
	fields := []string{
		"Budget",
		"Chapters",
		"DateCreated",
		"Genres",
		"HomePageUrl",
		"IndexOptions",
		"MediaStreams",
		"Overview",
		"ParentId",
		"Path",
		"People",
		"ProviderIds",
		"PrimaryImageAspectRatio",
		"Revenue",
		"SortName",
		"Studios",
		"Taglines",
	}

	queryParams := url.Values{
		"Fields": {strings.Join(fields, ",")},
	}

	if optionalParams.Filters != nil {
		queryParams.Add("Filters", strings.Join(optionalParams.Filters, ","))
	}

	if optionalParams.ParentID != "" {
		queryParams.Add("ParentId", optionalParams.ParentID)
	}

	if optionalParams.Path != "" {
		queryParams.Add("Path", optionalParams.Path)
	}

	if optionalParams.Limit > 0 {
		queryParams.Add("Limit", strconv.Itoa(optionalParams.Limit))
	}

	if optionalParams.AnyProviderIdEquals != nil {
		providerIDs := make([]string, 0, len(optionalParams.AnyProviderIdEquals))

		for providerName, id := range optionalParams.AnyProviderIdEquals {
			providerIDs = append(providerIDs, fmt.Sprintf("%s.%s", providerName, id))
		}

		queryParams.Add("AnyProviderIdEquals", strings.Join(providerIDs, ","))
	}

	if optionalParams.IncludeItemTypes != nil {
		queryParams.Add("IncludeItemTypes", strings.Join(optionalParams.IncludeItemTypes, ","))
	}

	if optionalParams.Recursive != nil {
		queryParams.Add("Recursive", strconv.FormatBool(*optionalParams.Recursive))
	}

	var response QueryResultBaseItemDto

	err := c.sendGETRequestToAPI(ctx, "/Items", queryParams, &response)

	return response, err
}

type GetUserItemsOptionalParams struct {
	Recursive           *bool
	AnyProviderIdEquals map[string]string
	IsPlayed            *bool
	ParentID            string
	Path                string
	SortOrder           string
	Filters             []string
	IncludeItemTypes    []string
	SortBy              []string
	Limit               int
}

func (c *Client) GetUserItems(
	ctx context.Context,
	userID string,
	optionalParams *GetUserItemsOptionalParams,
) (QueryResultBaseItemDto, error) {
	fields := []string{
		"ProviderIds",
		"UserData",
	}

	queryParams := url.Values{
		"Fields": {strings.Join(fields, ",")},
	}

	if optionalParams.Filters != nil {
		queryParams.Add("Filters", strings.Join(optionalParams.Filters, ","))
	}

	if optionalParams.ParentID != "" {
		queryParams.Add("ParentId", optionalParams.ParentID)
	}

	if optionalParams.Path != "" {
		queryParams.Add("Path", optionalParams.Path)
	}

	if optionalParams.Limit > 0 {
		queryParams.Add("Limit", strconv.Itoa(optionalParams.Limit))
	}

	if optionalParams.AnyProviderIdEquals != nil {
		providerIDs := make([]string, 0, len(optionalParams.AnyProviderIdEquals))

		for providerName, id := range optionalParams.AnyProviderIdEquals {
			providerIDs = append(providerIDs, fmt.Sprintf("%s.%s", providerName, id))
		}

		queryParams.Add("AnyProviderIdEquals", strings.Join(providerIDs, ","))
	}

	if optionalParams.IncludeItemTypes != nil {
		queryParams.Add("IncludeItemTypes", strings.Join(optionalParams.IncludeItemTypes, ","))
	}

	if optionalParams.SortBy != nil {
		queryParams.Add("SortBy", strings.Join(optionalParams.SortBy, ","))
	}

	if optionalParams.SortOrder != "" {
		queryParams.Add("SortOrder", optionalParams.SortOrder)
	}

	if optionalParams.IsPlayed != nil {
		queryParams.Add("IsPlayed", strconv.FormatBool(*optionalParams.IsPlayed))
	}

	if optionalParams.Recursive != nil {
		queryParams.Add("Recursive", strconv.FormatBool(*optionalParams.Recursive))
	}

	var response QueryResultBaseItemDto

	err := c.sendGETRequestToAPI(ctx, fmt.Sprintf("/Users/%s/Items", userID), queryParams, &response)

	return response, err
}
